package audit_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockAuditSvc satisfies the private auditService interface in audit.Handler.
type mockAuditSvc struct {
	logs map[uuid.UUID]audit.AuditLog
}

func newMockAuditSvc() *mockAuditSvc {
	return &mockAuditSvc{logs: make(map[uuid.UUID]audit.AuditLog)}
}

func (m *mockAuditSvc) ListAll(_ context.Context, _ string, _ audit.ListFilter, pr pagination.PageRequest) ([]audit.AuditLog, int, error) {
	items := make([]audit.AuditLog, 0, len(m.logs))
	for _, l := range m.logs {
		items = append(items, l)
	}
	return items, len(items), nil
}

func (m *mockAuditSvc) GetByID(_ context.Context, _ string, id uuid.UUID) (audit.AuditLog, error) {
	l, ok := m.logs[id]
	if !ok {
		return audit.AuditLog{}, audit.ErrNotFound
	}
	return l, nil
}

func (m *mockAuditSvc) Record(_ context.Context, _ string, req audit.RecordRequest) (audit.AuditLog, error) {
	id := uuid.New()
	l := audit.AuditLog{
		ID:         id,
		EntityType: req.EntityType,
		Action:     req.Action,
	}
	m.logs[id] = l
	return l, nil
}

func setupAudit(t *testing.T) (*chi.Mux, *mockAuditSvc, string) {
	t.Helper()
	realm := "audit-test-" + uuid.NewString()
	key, keycloakURL := setupAuditKeycloak(t, realm)
	token := signAuditJWT(t, key, realm, keycloakURL)

	svc := newMockAuditSvc()
	h := audit.NewHandler(svc)
	r := chi.NewRouter()
	chain := middleware.New(keycloakURL, []string{"*"})
	for _, mw := range chain.Protected() {
		r.Use(mw)
	}
	r.Mount("/api/audit", h.Routes())
	return r, svc, token
}

func auditAdminRequest(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func setupAuditKeycloak(t *testing.T, realm string) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwksDoc := map[string]any{"keys": []map[string]any{{
		"kid": "audit-test-kid",
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}}}
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/realms/%s/protocol/openid-connect/certs", realm), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksDoc)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return key, srv.URL
}

func signAuditJWT(t *testing.T, key *rsa.PrivateKey, realm, keycloakURL string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":          fmt.Sprintf("%s/realms/%s", keycloakURL, realm),
		"sub":          "audit-admin",
		"exp":          time.Now().Add(time.Hour).Unix(),
		"iat":          time.Now().Unix(),
		"realm_access": map[string]any{"roles": []string{"admin"}},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "audit-test-kid"
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestAuditHandler_List_Success(t *testing.T) {
	r, svc, token := setupAudit(t)
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{ID: id, EntityType: "agent", Action: audit.AuditActionCreate}

	req := auditAdminRequest(http.MethodGet, "/api/audit/", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[audit.AuditLogResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestAuditHandler_List_Empty(t *testing.T) {
	r, _, token := setupAudit(t)
	req := auditAdminRequest(http.MethodGet, "/api/audit/", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditHandler_GetByID_Success(t *testing.T) {
	r, svc, token := setupAudit(t)
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{ID: id, EntityType: "skill", Action: audit.AuditActionUpdate}

	req := auditAdminRequest(http.MethodGet, "/api/audit/"+id.String(), token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp audit.AuditLogResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, id, resp.ID)
}

func TestAuditHandler_GetByID_NotFound(t *testing.T) {
	r, _, token := setupAudit(t)
	req := auditAdminRequest(http.MethodGet, "/api/audit/"+uuid.New().String(), token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuditHandler_GetByID_InvalidID(t *testing.T) {
	r, _, token := setupAudit(t)
	req := auditAdminRequest(http.MethodGet, "/api/audit/not-a-uuid", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
