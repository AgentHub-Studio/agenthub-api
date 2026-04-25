package middleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

func TestChain_Public_NoAuthRequired(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})

	// Public chain should not reject requests without Authorization header.
	handler := applyMiddlewares(chain.Public(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestChain_Protected_RejectsNoToken(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})

	// Protected chain must reject requests without an Authorization header.
	handler := applyMiddlewares(chain.Protected(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestChain_Protected_RejectsInvalidToken(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})

	handler := applyMiddlewares(chain.Protected(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestChain_Protected_RejectsFakeSignature(t *testing.T) {
	// A structurally-valid JWT with a fake (non-RSA) signature must be rejected.
	// This test guards against the regression where only structure was checked.
	// Use a unique realm so the global JWKS cache doesn't bleed into other tests.
	realm := "test-realm-fake-sig"
	key, keycloakURL := mustSetupFakeKeycloak(t, realm)

	// Sign a valid JWT with an UNKNOWN key (not the one served by the mock Keycloak).
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tokenStr := mustSignJWT(t, wrongKey, realm, keycloakURL, "wrong-kid")

	chain := middleware.New(keycloakURL, []string{"*"})
	handler := applyMiddlewares(chain.Protected(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	_ = key
}

func TestChain_Protected_AcceptsValidJWT(t *testing.T) {
	// Use a unique realm to prevent global JWKS cache collisions with other tests.
	realm := "test-realm-valid-jwt"
	key, keycloakURL := mustSetupFakeKeycloak(t, realm)

	tokenStr := mustSignJWT(t, key, realm, keycloakURL, "test-kid")

	chain := middleware.New(keycloakURL, []string{"*"})
	handler := applyMiddlewares(chain.Protected(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "valid JWT signed with realm key should be accepted")
}

func TestRequireRole_RejectsMissingRole(t *testing.T) {
	realm := "test-realm-missing-role"
	key, keycloakURL := mustSetupFakeKeycloak(t, realm)
	tokenStr := mustSignJWT(t, key, realm, keycloakURL, "test-kid")

	chain := middleware.New(keycloakURL, []string{"*"})
	stack := append(chain.Protected(), middleware.RequireRole("admin"))
	handler := applyMiddlewares(stack, okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireRole_AcceptsRealmRole(t *testing.T) {
	realm := "test-realm-has-role"
	key, keycloakURL := mustSetupFakeKeycloak(t, realm)
	tokenStr := mustSignJWTWithRoles(t, key, realm, keycloakURL, "test-kid", []string{"admin"}, nil)

	chain := middleware.New(keycloakURL, []string{"*"})
	stack := append(chain.Protected(), middleware.RequireRole("admin"))
	handler := applyMiddlewares(stack, okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_AcceptsResourceRole(t *testing.T) {
	realm := "test-realm-resource-role"
	key, keycloakURL := mustSetupFakeKeycloak(t, realm)
	tokenStr := mustSignJWTWithRoles(t, key, realm, keycloakURL, "test-kid", nil, map[string][]string{
		"agenthub-api": {"mcp-client-runtime"},
	})

	chain := middleware.New(keycloakURL, []string{"*"})
	stack := append(chain.Protected(), middleware.RequireRole("mcp-client-runtime"))
	handler := applyMiddlewares(stack, okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestChain_Public_HasCORSHeader(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"https://app.example.com"})

	// CORS is applied at root router level via CORSHandler(), not inside Public().
	handler := chain.CORSHandler()(applyMiddlewares(chain.Public(), okHandler()))
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// CORS header should be present.
	require.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestChain_CORSWildcardDoesNotAllowCredentialedOrigins(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})

	handler := chain.CORSHandler()(applyMiddlewares(chain.Public(), okHandler()))
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestChain_New_ReturnsNonNil(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})
	assert.NotNil(t, chain)
	assert.NotEmpty(t, chain.Public())
	assert.NotEmpty(t, chain.Protected())
}

// --- helpers ---

// applyMiddlewares wraps the given handler with each middleware in order.
func applyMiddlewares(middlewares []func(http.Handler) http.Handler, h http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// okHandler returns an HTTP handler that always responds 200 OK.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// mustSetupFakeKeycloak starts a test HTTP server that serves a JWKS document for the given
// realm. Returns the RSA key used for signing and the base URL of the fake Keycloak server.
func mustSetupFakeKeycloak(t *testing.T, realm string) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Build a minimal JWKS document.
	nBytes := key.PublicKey.N.Bytes()
	eVal := key.PublicKey.E
	eBytes := make([]byte, 4)
	eBytes[0] = byte(eVal >> 24)
	eBytes[1] = byte(eVal >> 16)
	eBytes[2] = byte(eVal >> 8)
	eBytes[3] = byte(eVal)
	// Trim leading zero bytes from E.
	i := 0
	for i < len(eBytes)-1 && eBytes[i] == 0 {
		i++
	}
	eBytes = eBytes[i:]

	jwksDoc := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kid": "test-kid",
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(nBytes),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/realms/%s/protocol/openid-connect/certs", realm), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksDoc)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return key, srv.URL
}

// mustSignJWT signs a JWT using the given RSA key and returns the token string.
func mustSignJWT(t *testing.T, key *rsa.PrivateKey, realm, keycloakBaseURL, kid string) string {
	t.Helper()

	return mustSignJWTWithRoles(t, key, realm, keycloakBaseURL, kid, nil, nil)
}

type testAccessRoles struct {
	Roles []string `json:"roles"`
}

type testRoleClaims struct {
	jwt.RegisteredClaims
	RealmAccess    testAccessRoles            `json:"realm_access,omitempty"`
	ResourceAccess map[string]testAccessRoles `json:"resource_access,omitempty"`
}

func mustSignJWTWithRoles(
	t *testing.T,
	key *rsa.PrivateKey,
	realm, keycloakBaseURL, kid string,
	realmRoles []string,
	resourceRoles map[string][]string,
) string {
	t.Helper()

	// Encode the public key as PEM for display only; not needed for signing.
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	_ = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	claims := testRoleClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    fmt.Sprintf("%s/realms/%s", keycloakBaseURL, realm),
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		RealmAccess:    testAccessRoles{Roles: realmRoles},
		ResourceAccess: make(map[string]testAccessRoles, len(resourceRoles)),
	}
	for clientID, roles := range resourceRoles {
		claims.ResourceAccess[clientID] = testAccessRoles{Roles: roles}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

// bigIntToBytes converts a *big.Int to its minimal big-endian byte representation.
func bigIntToBytes(n *big.Int) []byte {
	return n.Bytes()
}
