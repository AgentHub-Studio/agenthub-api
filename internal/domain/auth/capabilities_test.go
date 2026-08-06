package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/auth"
)

func TestCapabilitiesEndpointReturnsUserRolesPermissionsAndFeatures(t *testing.T) {
	r := chi.NewRouter()
	auth.NewHandler(func(*http.Request) (auth.AuthenticatedUser, string, []string) {
		return auth.AuthenticatedUser{
			ID:       "user-1",
			Email:    "admin@example.test",
			Username: "admin",
		}, "test", []string{"admin"}
	}, auth.FeatureFlags{RBAC: true, ACL: true}).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/capabilities", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body auth.CapabilitiesResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.User.ID != "user-1" || body.User.Email != "admin@example.test" {
		t.Fatalf("unexpected user: %+v", body.User)
	}
	if body.TenantID != "test" {
		t.Fatalf("tenantId = %q, want test", body.TenantID)
	}
	if !body.Features.RBAC || !body.Features.ACL {
		t.Fatalf("features = %+v, want rbac+acl", body.Features)
	}
	if !auth.HasAny(body.Permissions, "agents:delete") {
		t.Fatalf("admin permissions should include agents wildcard: %v", body.Permissions)
	}
}

func TestPermissionMatchEndpoint(t *testing.T) {
	r := chi.NewRouter()
	auth.NewHandler(nil, auth.FeatureFlags{}).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/permissions/match", strings.NewReader(`{
		"granted": "agents:*",
		"required": "agents:read"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Matches bool `json:"matches"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Matches {
		t.Fatalf("matches = false, want true")
	}
}

func TestPermissionMatchEndpointRejectsMissingFields(t *testing.T) {
	r := chi.NewRouter()
	auth.NewHandler(nil, auth.FeatureFlags{}).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/permissions/match", strings.NewReader(`{"granted":"agents:*"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPermissionMatchEndpointRejectsTrailingJSON(t *testing.T) {
	r := chi.NewRouter()
	auth.NewHandler(nil, auth.FeatureFlags{}).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/permissions/match", strings.NewReader(`{"granted":"agents:read","required":"agents:read"} {"granted":"*"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
