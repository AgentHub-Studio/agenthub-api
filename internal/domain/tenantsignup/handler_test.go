package tenantsignup_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenantsignup"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
)

type alreadyExistsTenantCreator struct{}

func (alreadyExistsTenantCreator) Create(context.Context, tenant.CreateTenantRequest) (tenant.TenantResponse, error) {
	return tenant.TenantResponse{}, tenant.ErrAlreadyExists
}

type noopUserCreator struct{}

func (noopUserCreator) CreateUser(context.Context, string, user.CreateUserRequest) (user.User, error) {
	return user.User{}, nil
}

type recordingTenantCreator struct {
	calls int
}

func (c *recordingTenantCreator) Create(context.Context, tenant.CreateTenantRequest) (tenant.TenantResponse, error) {
	c.calls++
	return tenant.TenantResponse{}, nil
}

type recordingUserCreator struct {
	calls int
}

func (c *recordingUserCreator) CreateUser(context.Context, string, user.CreateUserRequest) (user.User, error) {
	c.calls++
	return user.User{}, nil
}

func TestHandlerSignupMapsExistingTenantToValidationError(t *testing.T) {
	svc := tenantsignup.NewService(alreadyExistsTenantCreator{}, noopUserCreator{}, "", "")
	handler := tenantsignup.NewHandler(svc)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/tenants/signup", bytes.NewBufferString(`{
		"tenantId":"test",
		"tenantName":"Test",
		"adminEmail":"admin@example.com",
		"adminFirstName":"Admin"
	}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Contains(t, rec.Body.String(), "tenant already exists")
}

func TestHandlerSignupRejectsTrailingJSONWithoutCreatingTenant(t *testing.T) {
	tenants := &recordingTenantCreator{}
	users := &recordingUserCreator{}
	svc := tenantsignup.NewService(tenants, users, "", "")
	handler := tenantsignup.NewHandler(svc)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/tenants/signup", bytes.NewBufferString(`{
		"tenantId":"test",
		"tenantName":"Test",
		"adminEmail":"admin@example.com",
		"adminFirstName":"Admin"
	}{"tenantName":"Ignored"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Zero(t, tenants.calls)
	require.Zero(t, users.calls)
}
