package tenantsignup_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenantsignup"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
)

type fakeTenantCreator struct {
	called  []tenant.CreateTenantRequest
	errWith string
	err     error
}

func (f *fakeTenantCreator) Create(_ context.Context, req tenant.CreateTenantRequest) (tenant.TenantResponse, error) {
	f.called = append(f.called, req)
	if f.err != nil {
		return tenant.TenantResponse{}, f.err
	}
	if f.errWith != "" {
		return tenant.TenantResponse{}, errors.New(f.errWith)
	}
	return tenant.TenantResponse{ID: req.ID, Name: req.Name}, nil
}

type fakeUserCreator struct {
	called  []user.CreateUserRequest
	errWith string
}

func (f *fakeUserCreator) CreateUser(_ context.Context, _ string, req user.CreateUserRequest) (user.User, error) {
	f.called = append(f.called, req)
	if f.errWith != "" {
		return user.User{}, errors.New(f.errWith)
	}
	return user.User{ID: uuid.New().String(), Username: req.Username, Email: req.Email}, nil
}

func newSvc(tc *fakeTenantCreator, uc *fakeUserCreator) *tenantsignup.Service {
	return tenantsignup.NewService(tc, uc, "http://keycloak.local", "https://app.local")
}

func TestSignup_HappyPath(t *testing.T) {
	tc, uc := &fakeTenantCreator{}, &fakeUserCreator{}
	svc := newSvc(tc, uc)

	got, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "my-company",
		TenantName:     "My Company",
		AdminEmail:     "alice@my-company.com",
		AdminFirstName: "Alice",
	})

	require.NoError(t, err)
	assert.Equal(t, "my-company", got.TenantID)
	assert.Equal(t, "alice", got.Username)
	assert.NotEmpty(t, got.TempPassword)
	assert.Len(t, got.TempPassword, 20)
	assert.Contains(t, got.LoginURL, "http://keycloak.local/realms/my-company")
	assert.Contains(t, got.LoginURL, "redirect_uri=https://app.local")

	require.Len(t, tc.called, 1)
	assert.Equal(t, "my-company", tc.called[0].ID)
	require.Len(t, uc.called, 1)
	assert.Equal(t, "alice", uc.called[0].Username)
	assert.Equal(t, []string{"admin"}, uc.called[0].Roles)
}

func TestSignup_RejectsInvalidSlug(t *testing.T) {
	svc := newSvc(&fakeTenantCreator{}, &fakeUserCreator{})

	for _, bad := range []string{"", "A", "ab", "My Company", "-leading", "trailing-", "under_score", "slash/in/name"} {
		_, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
			TenantID:       bad,
			TenantName:     "x",
			AdminEmail:     "a@b.com",
			AdminFirstName: "A",
		})
		assert.ErrorIs(t, err, tenantsignup.ErrWeakTenantID, "expected rejection for %q", bad)
	}
}

func TestSignup_RejectsInvalidEmail(t *testing.T) {
	svc := newSvc(&fakeTenantCreator{}, &fakeUserCreator{})

	_, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "okay",
		TenantName:     "OK",
		AdminEmail:     "not-an-email",
		AdminFirstName: "Alice",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adminEmail")
}

func TestSignup_RequiresFirstName(t *testing.T) {
	svc := newSvc(&fakeTenantCreator{}, &fakeUserCreator{})

	_, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "okay",
		TenantName:     "OK",
		AdminEmail:     "a@b.com",
		AdminFirstName: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adminFirstName")
}

func TestSignup_TenantCreateError_PropagatesAndSkipsUserCreate(t *testing.T) {
	tc := &fakeTenantCreator{errWith: "realm already in use"}
	uc := &fakeUserCreator{}
	svc := newSvc(tc, uc)

	_, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "taken",
		TenantName:     "Taken",
		AdminEmail:     "a@b.com",
		AdminFirstName: "A",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "realm already in use")
	assert.Empty(t, uc.called, "should not attempt user creation when tenant creation fails")
}

func TestSignup_TenantValidationErrorMapsToClientValidation(t *testing.T) {
	tc := &fakeTenantCreator{err: fmt.Errorf("%w: tenant id %q is reserved", tenant.ErrValidation, "core")}
	uc := &fakeUserCreator{}
	svc := newSvc(tc, uc)

	_, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "core",
		TenantName:     "Core Tenant",
		AdminEmail:     "admin@example.invalid",
		AdminFirstName: "Admin",
	})

	require.ErrorIs(t, err, tenantsignup.ErrValidation)
	assert.Contains(t, err.Error(), "reserved")
	assert.NotContains(t, err.Error(), "validation failed: validation failed")
	assert.Empty(t, uc.called, "should not attempt user creation when tenant validation fails")
}

func TestSignup_UserCreateError_DoesNotRollbackTenant(t *testing.T) {
	// P-tenantsignup: leaving a half-provisioned tenant when user creation fails
	// is acceptable — admin can retry user creation via /api/tenants/{id}/users.
	tc := &fakeTenantCreator{}
	uc := &fakeUserCreator{errWith: "email already in use"}
	svc := newSvc(tc, uc)

	_, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "okay",
		TenantName:     "OK",
		AdminEmail:     "a@b.com",
		AdminFirstName: "A",
	})

	require.Error(t, err)
	assert.Len(t, tc.called, 1, "tenant creation should have happened")
	assert.Len(t, uc.called, 1, "user creation should have been attempted")
}

func TestSignup_UsernameDerivesFromLocalPartOfEmail(t *testing.T) {
	tc, uc := &fakeTenantCreator{}, &fakeUserCreator{}
	svc := newSvc(tc, uc)

	got, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "acme",
		TenantName:     "Acme",
		AdminEmail:     "bob.smith@acme.com",
		AdminFirstName: "Bob",
	})

	require.NoError(t, err)
	assert.Equal(t, "bob.smith", got.Username)
}

func TestSignup_LoginURL_EmptyWhenKeycloakBaseMissing(t *testing.T) {
	svc := tenantsignup.NewService(&fakeTenantCreator{}, &fakeUserCreator{}, "", "")

	got, err := svc.Signup(context.Background(), tenantsignup.SignupRequest{
		TenantID:       "okay",
		TenantName:     "OK",
		AdminEmail:     "a@b.com",
		AdminFirstName: "A",
	})

	require.NoError(t, err)
	assert.Empty(t, got.LoginURL)
}
