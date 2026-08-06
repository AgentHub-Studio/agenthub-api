package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Chain holds the configured middleware stack.
type Chain struct {
	keycloakBaseURL   string
	keycloakIssuerURL string // bug 281: validar `iss` claim contra esta URL pública. Vazio = sem validação (legacy).
	corsOrigins       []string
}

// New creates a new middleware Chain.
func New(keycloakBaseURL string, corsOrigins []string) *Chain {
	return &Chain{
		keycloakBaseURL: keycloakBaseURL,
		corsOrigins:     corsOrigins,
	}
}

// WithIssuerURL configura a URL pública do Keycloak para validação do `iss` claim.
// Bug 281: sem essa validação, o regex extractRealm aceitaria tokens com
// `iss: https://evil.com/realms/<realm>` desde que a signature do `<realm>`
// fosse válida no nosso Keycloak. Setando issuer URL, validamos start_with.
func (c *Chain) WithIssuerURL(issuerURL string) *Chain {
	c.keycloakIssuerURL = issuerURL
	return c
}

// CORSHandler returns the CORS middleware for use at the root router level.
// Must be applied before any route groups so that OPTIONS preflight requests
// are handled before chi returns 405 Method Not Allowed.
func (c *Chain) CORSHandler() func(http.Handler) http.Handler {
	return CORS(c.corsOrigins)
}

// Public returns middleware for unauthenticated routes: Recovery → RequestID → Logger.
// CORS is applied at root router level via CORSHandler(), not here.
func (c *Chain) Public() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		Recovery,
		RequestID,
		Logger,
	}
}

// Protected returns middleware for authenticated routes: Public stack + NoStoreCache + Auth + Tenant.
// NoStoreCache (bug 280) é aplicado antes do Auth para que mesmo respostas 401/403
// recebam Cache-Control: no-store — caso contrário browser pode cachear o erro
// e mostrá-lo após o token ser renovado.
// Auth validates JWT signature via Keycloak JWKS; Tenant extracts tenantID from the issuer claim.
func (c *Chain) Protected() []func(http.Handler) http.Handler {
	return append(c.Public(),
		NoStoreCache,
		authMiddleware(c.keycloakBaseURL, c.keycloakIssuerURL),
		tenantMiddleware(),
	)
}

// OptionalAuth returns middleware for routes that are public by default but
// may return tenant-owned data when a valid Bearer token is present. Responses
// are never cached because the same URL can produce a different result for an
// authenticated owner than for an anonymous request.
func (c *Chain) OptionalAuth() []func(http.Handler) http.Handler {
	return append(c.Public(),
		NoStoreCache,
		optionalAuthMiddleware(c.keycloakBaseURL, c.keycloakIssuerURL),
	)
}

// optionalAuthMiddleware leaves requests without an Authorization header
// anonymous. If the caller presents credentials, it validates them exactly as
// protected routes do and stores the tenant in the request context.
func optionalAuthMiddleware(keycloakBaseURL, expectedIssuerPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		authenticated := authMiddleware(keycloakBaseURL, expectedIssuerPrefix)(tenantMiddleware()(next))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
			}
			authenticated.ServeHTTP(w, r)
		})
	}
}

// authMiddleware validates the Bearer JWT signature against the Keycloak JWKS endpoint.
// The issuer realm is extracted from the token's unverified payload to locate the correct
// JWKS URL, then the signature is verified with the matching RSA public key. Keys are
// cached per-realm for jwksCacheTTL to avoid repeated round-trips to Keycloak.
func authMiddleware(keycloakBaseURL, expectedIssuerPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenStr, ok := extractBearerToken(authHeader)
			if !ok {
				rejectUnauthorized(w, "missing or invalid authorization header")
				return
			}

			// Structural check: must have three dot-separated non-empty parts.
			parts := strings.Split(tokenStr, ".")
			if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
				rejectUnauthorized(w, "malformed jwt token")
				return
			}

			// Extract the realm from the unverified payload so we know which JWKS to fetch.
			realm, issuer, err := extractRealmAndIssuer(parts[1])
			if err != nil {
				rejectUnauthorized(w, "cannot extract realm from jwt issuer")
				return
			}

			// Bug 281: se KEYCLOAK_ISSUER_URL configurado, valida que `iss` começa
			// com a URL pública esperada. Sem isso, atacante com signature válida
			// do realm-A poderia forjar `iss: https://evil/realms/realm-A`.
			if expectedIssuerPrefix != "" {
				expected := strings.TrimRight(expectedIssuerPrefix, "/") + "/realms/" + realm
				if issuer != expected {
					rejectUnauthorized(w, "invalid token issuer")
					return
				}
			}

			// Fetch (or return cached) signing keys for this realm.
			keys, err := getRealmKeys(keycloakBaseURL, realm)
			if err != nil {
				// Bug 264: erro de transport pode incluir URL Keycloak interna
				// (`Get "http://keycloak.agenthub.svc.cluster.local:8080/...":
				// dial tcp ...`). Loga server-side, retorna msg genérica.
				slog.Error("auth: jwks fetch failed", "realm", realm, "err", err)
				rejectUnauthorized(w, "authentication backend unavailable")
				return
			}

			// Verify the JWT signature and retain authorization claims for role middleware.
			claims := &roleClaims{}
			_, err = jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				kid, _ := token.Header["kid"].(string)
				key, ok := keys[kid]
				if !ok {
					// kid not in cache — attempt a single refresh in case Keycloak rotated keys.
					freshKeys, fetchErr := fetchRealmJWKS(keycloakBaseURL, realm)
					if fetchErr != nil {
						return nil, fmt.Errorf("unknown kid %q and refresh failed: %w", kid, fetchErr)
					}
					globalJWKSCache.set(realm, freshKeys)
					key, ok = freshKeys[kid]
					if !ok {
						return nil, fmt.Errorf("unknown kid %q even after key refresh", kid)
					}
				}
				return key, nil
			})
			if err != nil {
				rejectUnauthorized(w, "invalid or expired token")
				return
			}

			next.ServeHTTP(w, r.WithContext(contextWithClaims(r.Context(), claims)))
		})
	}
}

// rejectUnauthorized writes a JSON 401 response.
func rejectUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// extractBearerToken parses an Authorization header value matching the
// "Bearer <token>" pattern. Bug 273: scheme name é case-insensitive segundo
// RFC 7235 §2.1, então "bearer", "BEARER", "Bearer" são todos válidos.
// Retorna (token, true) se o header começa com "bearer " (case-insensitive)
// e tem token non-empty; caso contrário ("", false).
func extractBearerToken(authHeader string) (string, bool) {
	const prefix = "bearer "
	if len(authHeader) <= len(prefix) {
		return "", false
	}
	if !strings.EqualFold(authHeader[:len(prefix)], prefix) {
		return "", false
	}
	token := authHeader[len(prefix):]
	if token == "" {
		return "", false
	}
	return token, true
}

// extractRealm decodes the base64url JWT payload and returns the Keycloak realm name
// extracted from the "iss" claim (e.g. ".../realms/my-tenant" → "my-tenant").
func extractRealm(payloadB64 string) (string, error) {
	realm, _, err := extractRealmAndIssuer(payloadB64)
	return realm, err
}

// extractRealmAndIssuer decodes the JWT payload and returns both the realm name
// (extracted via regex) and the full `iss` claim. Used by authMiddleware to
// validate the issuer against KEYCLOAK_ISSUER_URL (bug 281).
func extractRealmAndIssuer(payloadB64 string) (string, string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", "", fmt.Errorf("cannot decode jwt payload: %w", err)
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Issuer == "" {
		return "", "", fmt.Errorf("missing iss claim in jwt")
	}
	m := realmRegex.FindStringSubmatch(claims.Issuer)
	if len(m) < 2 {
		return "", "", fmt.Errorf("iss claim %q does not contain /realms/<name>", claims.Issuer)
	}
	return m[1], claims.Issuer, nil
}

var realmRegex = regexp.MustCompile(`/realms/([^/]+)`)

// tenantMiddleware extracts tenantID from the JWT issuer claim and stores it in context.
// Runs after authMiddleware, which has already verified the JWT signature via Keycloak JWKS.
// Extracts the realm name from the Keycloak issuer URL
// (e.g. "https://keycloak.example.com/realms/my-tenant" → "my-tenant").
func tenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenStr, _ := extractBearerToken(authHeader)
			parts := strings.Split(tokenStr, ".")
			if len(parts) != 3 {
				rejectUnauthorized(w, "malformed jwt token")
				return
			}

			// By the time we reach this middleware, authMiddleware has already validated
			// the JWT signature. We re-extract the realm here using the same helper.
			realm, err := extractRealm(parts[1])
			if err != nil {
				rejectUnauthorized(w, err.Error())
				return
			}

			ctx := tenant.NewContextWithToken(r.Context(), realm, tokenStr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
