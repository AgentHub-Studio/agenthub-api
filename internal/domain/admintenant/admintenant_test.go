package admintenant

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type recordingTenantAPI struct {
	createCalls int
}

func (r *recordingTenantAPI) Create(context.Context, tenant.CreateTenantRequest) (tenant.TenantResponse, error) {
	r.createCalls++
	return tenant.TenantResponse{}, nil
}

func (r *recordingTenantAPI) List(context.Context, pagination.PageRequest) (pagination.Page[tenant.TenantResponse], error) {
	return pagination.Page[tenant.TenantResponse]{}, nil
}

type recordingTenantRepo struct {
	updateCalls int
}

func (r *recordingTenantRepo) UpdateName(context.Context, string, string) error {
	r.updateCalls++
	return nil
}

func (*recordingTenantRepo) Delete(context.Context, string) error { return nil }

type recordingAdminUserAPI struct {
	createCalls int
}

func (r *recordingAdminUserAPI) CreateUser(context.Context, string, user.CreateUserRequest) (user.User, error) {
	r.createCalls++
	return user.User{}, nil
}

type noopRealmDeleter struct{}

func (noopRealmDeleter) DeleteRealm(context.Context, string) error { return nil }

func setupAdminTenantRouter() (*chi.Mux, *recordingTenantAPI, *recordingTenantRepo, *recordingAdminUserAPI) {
	tenants := &recordingTenantAPI{}
	repo := &recordingTenantRepo{}
	users := &recordingAdminUserAPI{}
	handler := NewHandler(NewService(tenants, repo, users, noopRealmDeleter{}))
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	return router, tenants, repo, users
}

func TestWriteAdminTenantErr_UpstreamLeakSanitized(t *testing.T) {
	cases := []struct {
		name string
		err  string
	}{
		{"keycloak svc cluster URL", `Post "http://keycloak.agenthub.svc.cluster.local:8080/admin/realms/x/users": context deadline exceeded`},
		{"context deadline only", "context deadline exceeded"},
		{"keycloak word match", "keycloak responded with 503"},
		{"dial tcp", "dial tcp 10.0.0.1:8080: connect: connection refused"},
		{"Get http URL", `Get "http://upstream.internal/x": net/http: timeout`},
		{"Delete realm URL", `Delete "http://keycloak.internal/admin/realms/x": EOF`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeAdminTenantErr(rr, "test.op", errors.New(tc.err))
			if rr.Code != 502 {
				t.Fatalf("expected 502, got %d (body=%s)", rr.Code, rr.Body.String())
			}
			body := rr.Body.String()
			if strings.Contains(body, "svc.cluster.local") || strings.Contains(body, "10.0.0.1") || strings.Contains(body, "context deadline") {
				t.Fatalf("body leaks upstream detail: %s", body)
			}
			if !strings.Contains(body, "tenant provisioning service unavailable") {
				t.Fatalf("body missing sanitized message: %s", body)
			}
		})
	}
}

func TestWriteAdminTenantErr_ValidationStaysAs400(t *testing.T) {
	cases := []string{
		"tenantId is required",
		"adminPassword must be at least 8 characters",
		"cannot delete the core tenant",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeAdminTenantErr(rr, "test.op", errors.New(msg))
			if rr.Code != 400 {
				t.Fatalf("expected 400 for %q, got %d", msg, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), msg) {
				t.Fatalf("expected body to contain %q, got %s", msg, rr.Body.String())
			}
		})
	}
}

func TestAdminTenantHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		router, tenants, _, users := setupAdminTenantRouter()
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/admin/tenants/",
			strings.NewReader(`{"tenantId":"test","tenantName":"Test","adminUsername":"admin","adminPassword":"password-123"}{"tenantName":"Ignored"}`),
		)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
		if tenants.createCalls != 0 || users.createCalls != 0 {
			t.Fatalf("unexpected create calls: tenants=%d users=%d", tenants.createCalls, users.createCalls)
		}
	})

	t.Run("update", func(t *testing.T) {
		router, _, repo, _ := setupAdminTenantRouter()
		req := httptest.NewRequest(
			http.MethodPut,
			"/api/admin/tenants/test",
			strings.NewReader(`{"tenantName":"Changed"}{"tenantName":"Ignored"}`),
		)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
		if repo.updateCalls != 0 {
			t.Fatalf("unexpected update calls: %d", repo.updateCalls)
		}
	})
}
