package auth

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
)

// testKeyPair holds an RSA key pair and its JWKS kid for test usage.
type testKeyPair struct {
	privateKey *rsa.PrivateKey
	kid        string
}

func generateTestKeyPair(t *testing.T) *testKeyPair {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return &testKeyPair{privateKey: pk, kid: "test-kid-1"}
}

// newJWKSServer starts a mock HTTP server that serves JWKS for the given key pair.
func newJWKSServer(t *testing.T, kp *testKeyPair) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pub := &kp.privateKey.PublicKey
		nBytes := pub.N.Bytes()
		eVal := big.NewInt(int64(pub.E))
		eBytes := eVal.Bytes()

		key := jwk{
			Kid: kp.kid,
			Kty: "RSA",
			Alg: "RS256",
			N:   base64.RawURLEncoding.EncodeToString(nBytes),
			E:   base64.RawURLEncoding.EncodeToString(eBytes),
		}
		resp := jwksResponse{Keys: []jwk{key}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// signToken creates and signs a JWT for testing.
func signToken(t *testing.T, kp *testKeyPair, issuer string, expiry time.Time) string {
	t.Helper()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		RealmAccess:   RealmAccess{Roles: []string{"user", "admin"}},
		PreferredName: "testuser",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kp.kid
	signed, err := token.SignedString(kp.privateKey)
	require.NoError(t, err)
	return signed
}

// handlerOK is a simple handler that returns 200 and the tenantID from claims.
var handlerOK = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "no claims", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(claims.PreferredName))
})

func TestMiddleware_ValidToken(t *testing.T) {
	kp := generateTestKeyPair(t)
	jwksSrv := newJWKSServer(t, kp)
	defer jwksSrv.Close()

	tenantID := "my-tenant"
	issuer := jwksSrv.URL + "/realms/" + tenantID
	tokenStr := signToken(t, kp, issuer, time.Now().Add(time.Hour))

	cfg := Config{KeycloakBaseURL: jwksSrv.URL}
	mw := Middleware(cfg)(handlerOK)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "testuser", rec.Body.String())
}

func TestMiddleware_MissingAuthorizationHeader(t *testing.T) {
	cfg := Config{KeycloakBaseURL: "http://keycloak.example.com"}
	mw := Middleware(cfg)(handlerOK)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing Authorization header")
}

func TestMiddleware_InvalidBearerFormat(t *testing.T) {
	cfg := Config{KeycloakBaseURL: "http://keycloak.example.com"}
	mw := Middleware(cfg)(handlerOK)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token sometoken")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "must start with Bearer")
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	kp := generateTestKeyPair(t)
	jwksSrv := newJWKSServer(t, kp)
	defer jwksSrv.Close()

	tenantID := "my-tenant"
	issuer := jwksSrv.URL + "/realms/" + tenantID
	// Token expired 1 hour ago
	tokenStr := signToken(t, kp, issuer, time.Now().Add(-time.Hour))

	cfg := Config{KeycloakBaseURL: jwksSrv.URL}
	mw := Middleware(cfg)(handlerOK)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestMiddleware_TokenWithUnknownKid(t *testing.T) {
	kp := generateTestKeyPair(t)
	jwksSrv := newJWKSServer(t, kp)
	defer jwksSrv.Close()

	// Use a different kid not present in JWKS
	kp2 := &testKeyPair{privateKey: kp.privateKey, kid: "unknown-kid"}
	tenantID := "my-tenant"
	issuer := jwksSrv.URL + "/realms/" + tenantID
	tokenStr := signToken(t, kp2, issuer, time.Now().Add(time.Hour))

	cfg := Config{KeycloakBaseURL: jwksSrv.URL}
	mw := Middleware(cfg)(handlerOK)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_AllowedIssuers_Accepted(t *testing.T) {
	kp := generateTestKeyPair(t)
	jwksSrv := newJWKSServer(t, kp)
	defer jwksSrv.Close()

	tenantID := "allowed-tenant"
	issuer := jwksSrv.URL + "/realms/" + tenantID
	tokenStr := signToken(t, kp, issuer, time.Now().Add(time.Hour))

	cfg := Config{
		KeycloakBaseURL: jwksSrv.URL,
		AllowedIssuers:  []string{jwksSrv.URL + "/realms/"},
	}
	mw := Middleware(cfg)(handlerOK)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_AllowedIssuers_Rejected(t *testing.T) {
	kp := generateTestKeyPair(t)
	jwksSrv := newJWKSServer(t, kp)
	defer jwksSrv.Close()

	tenantID := "evil-tenant"
	issuer := jwksSrv.URL + "/realms/" + tenantID
	tokenStr := signToken(t, kp, issuer, time.Now().Add(time.Hour))

	cfg := Config{
		KeycloakBaseURL: jwksSrv.URL,
		AllowedIssuers:  []string{"https://trusted.keycloak.com/realms/"},
	}
	mw := Middleware(cfg)(handlerOK)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "not in the allowed issuers list")
}

func TestExtractTenantID(t *testing.T) {
	tests := []struct {
		name     string
		issuer   string
		expected string
	}{
		{
			name:     "standard keycloak issuer",
			issuer:   "https://keycloak.example.com/realms/my-company",
			expected: "my-company",
		},
		{
			name:     "kebab-case tenant",
			issuer:   "https://keycloak.example.com/realms/my-long-tenant-name",
			expected: "my-long-tenant-name",
		},
		{
			name:     "no realms segment",
			issuer:   "https://keycloak.example.com",
			expected: "",
		},
		{
			name:     "empty string",
			issuer:   "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTenantID(tc.issuer)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRequireRole_AllowsMatchingRole(t *testing.T) {
	claims := &Claims{RealmAccess: RealmAccess{Roles: []string{"admin", "PROXY_SERVICE"}}}
	ctx := contextWithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claims)

	called := false
	handler := RequireRole("PROXY_SERVICE")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_ForbidsMissingRole(t *testing.T) {
	claims := &Claims{RealmAccess: RealmAccess{Roles: []string{"user"}}}
	ctx := contextWithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claims)

	handler := RequireRole("PROXY_SERVICE")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "forbidden")
}

func TestRequireRole_ForbidsNilClaims(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHasRole_RealmAccess(t *testing.T) {
	c := &Claims{RealmAccess: RealmAccess{Roles: []string{"admin", "user"}}}
	assert.True(t, c.HasRole("admin"))
	assert.False(t, c.HasRole("PROXY_SERVICE"))
}

func TestHasRole_ResourceAccess(t *testing.T) {
	c := &Claims{ResourceAccess: map[string]ClientRoles{
		"agenthub-frontend": {Roles: []string{"PROXY_SERVICE"}},
	}}
	assert.True(t, c.HasRole("PROXY_SERVICE"))
	assert.False(t, c.HasRole("admin"))
}

func TestHasRole_NilClaims(t *testing.T) {
	var c *Claims
	assert.False(t, c.HasRole("admin"))
}

func TestClaimsFromContext_NilWhenNotSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	claims := ClaimsFromContext(req.Context())
	assert.Nil(t, claims)
}

func TestClaimsFromContext_ReturnsStoredClaims(t *testing.T) {
	expected := &Claims{PreferredName: "alice"}
	ctx := contextWithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), expected)
	got := ClaimsFromContext(ctx)
	assert.Equal(t, expected, got)
}

func TestJWKSCache_TTL(t *testing.T) {
	kp := generateTestKeyPair(t)
	fetchCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		pub := &kp.privateKey.PublicKey
		nBytes := pub.N.Bytes()
		eVal := big.NewInt(int64(pub.E))
		eBytes := eVal.Bytes()
		key := jwk{
			Kid: kp.kid,
			Kty: "RSA",
			N:   base64.RawURLEncoding.EncodeToString(nBytes),
			E:   base64.RawURLEncoding.EncodeToString(eBytes),
		}
		json.NewEncoder(w).Encode(jwksResponse{Keys: []jwk{key}})
	}))
	defer srv.Close()

	// Very short TTL for testing
	cache := newJWKSCache(srv.URL, 50*time.Millisecond)

	// First call: should fetch
	_, err := cache.getKey(t.Context(), "tenant-a", kp.kid)
	require.NoError(t, err)
	assert.Equal(t, 1, fetchCount)

	// Second call within TTL: should use cache
	_, err = cache.getKey(t.Context(), "tenant-a", kp.kid)
	require.NoError(t, err)
	assert.Equal(t, 1, fetchCount)

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Third call after TTL: should refetch
	_, err = cache.getKey(t.Context(), "tenant-a", kp.kid)
	require.NoError(t, err)
	assert.Equal(t, 2, fetchCount)
}
