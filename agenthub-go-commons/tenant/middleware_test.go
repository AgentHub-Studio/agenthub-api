package tenant_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/auth"
	. "github.com/AgentHub-Studio/agenthub-go-commons/tenant"
)

// tenantTestKeyPair holds an RSA key pair for tests.
type tenantTestKeyPair struct {
	privateKey *rsa.PrivateKey
	kid        string
}

func generateKey(t *testing.T) *tenantTestKeyPair {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return &tenantTestKeyPair{privateKey: pk, kid: "tenant-test-kid"}
}

// newTenantJWKSServer starts an httptest server that serves JWKS for the given key pair.
func newTenantJWKSServer(t *testing.T, kp *tenantTestKeyPair) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pub := &kp.privateKey.PublicKey
		nBytes := pub.N.Bytes()
		eBytes := new(big.Int).SetInt64(int64(pub.E)).Bytes()

		type jwk struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		}
		type jwksResp struct {
			Keys []jwk `json:"keys"`
		}

		resp := jwksResp{Keys: []jwk{{
			Kid: kp.kid,
			Kty: "RSA",
			Alg: "RS256",
			N:   base64.RawURLEncoding.EncodeToString(nBytes),
			E:   base64.RawURLEncoding.EncodeToString(eBytes),
		}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// signTenantToken creates a signed JWT with the given issuer.
func signTenantToken(t *testing.T, kp *tenantTestKeyPair, issuer string) string {
	t.Helper()
	type keycloakClaims struct {
		jwt.RegisteredClaims
		RealmAccess   map[string][]string `json:"realm_access"`
		PreferredName string              `json:"preferred_username"`
	}
	claims := keycloakClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "user-xyz",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		RealmAccess:   map[string][]string{"roles": {"user"}},
		PreferredName: "testuser",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kp.kid
	signed, err := tok.SignedString(kp.privateKey)
	require.NoError(t, err)
	return signed
}

// buildAuthStack returns (tenantID, jwksServer, signedToken) for integration-style tests.
func buildAuthStack(t *testing.T, tenantID string) (string, *httptest.Server, string) {
	t.Helper()
	kp := generateKey(t)
	srv := newTenantJWKSServer(t, kp)
	issuer := srv.URL + "/realms/" + tenantID
	token := signTenantToken(t, kp, issuer)
	return tenantID, srv, token
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestMiddleware_ValidToken_InjectsTenantID(t *testing.T) {
	tenantID, srv, tokenStr := buildAuthStack(t, "acme-corp")
	defer srv.Close()

	cfg := auth.Config{KeycloakBaseURL: srv.URL}

	var capturedTenantID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	chain := auth.Middleware(cfg)(Middleware()(inner))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenantID, capturedTenantID)
}

func TestMiddleware_MissingClaims_Unauthorized(t *testing.T) {
	// Call tenant.Middleware WITHOUT auth.Middleware — claims will be nil.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := Middleware()(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing JWT claims")
}

func TestMiddleware_MultipleTenantsIsolated(t *testing.T) {
	kp := generateKey(t)
	srv := newTenantJWKSServer(t, kp)
	defer srv.Close()

	cfg := auth.Config{KeycloakBaseURL: srv.URL}

	for _, tenantID := range []string{"tenant-a", "tenant-b", "my-company"} {
		tenantID := tenantID
		t.Run(tenantID, func(t *testing.T) {
			issuer := srv.URL + "/realms/" + tenantID
			tokenStr := signTenantToken(t, kp, issuer)

			var got string
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = FromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			chain := auth.Middleware(cfg)(Middleware()(inner))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			rec := httptest.NewRecorder()
			chain.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tenantID, got)
		})
	}
}

func TestFromContext_EmptyWhenNotSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	result := FromContext(req.Context())
	assert.Equal(t, "", result)
}

func TestNewContext_StoreAndRetrieve(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewContext(req.Context(), "acme-corp")
	assert.Equal(t, "acme-corp", FromContext(ctx))
}

func TestNewContext_OverwritesPreviousValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewContext(req.Context(), "first-tenant")
	ctx = NewContext(ctx, "second-tenant")
	assert.Equal(t, "second-tenant", FromContext(ctx))
}
