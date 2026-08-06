package tenant_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"
)

type mockTenantRepo struct {
	data map[string]tenant.Tenant
}

func newMockRepo() *mockTenantRepo {
	return &mockTenantRepo{data: make(map[string]tenant.Tenant)}
}

func (m *mockTenantRepo) Create(_ context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	if _, exists := m.data[t.ID]; exists {
		return tenant.Tenant{}, tenant.ErrAlreadyExists
	}
	m.data[t.ID] = t
	return t, nil
}

func (m *mockTenantRepo) FindByID(_ context.Context, id string) (tenant.Tenant, error) {
	t, ok := m.data[id]
	if !ok {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	return t, nil
}

func (m *mockTenantRepo) FindAll(_ context.Context, _ pagination.PageRequest) ([]tenant.Tenant, int64, error) {
	out := make([]tenant.Tenant, 0, len(m.data))
	for _, t := range m.data {
		out = append(out, t)
	}
	return out, int64(len(out)), nil
}

func (m *mockTenantRepo) Exists(_ context.Context, id string) (bool, error) {
	_, ok := m.data[id]
	return ok, nil
}

func (m *mockTenantRepo) UpdateStatus(_ context.Context, id string, status tenant.Status) error {
	t, ok := m.data[id]
	if !ok {
		return tenant.ErrNotFound
	}
	t.Status = status
	m.data[id] = t
	return nil
}

func (m *mockTenantRepo) UpdateName(_ context.Context, id string, name string) error {
	t, ok := m.data[id]
	if !ok {
		return tenant.ErrNotFound
	}
	t.Name = name
	m.data[id] = t
	return nil
}

func (m *mockTenantRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.data[id]; !ok {
		return tenant.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

// noopProvisioner is a successful no-op.
type noopProvisioner struct{}

func (n *noopProvisioner) ProvisionRealm(_ context.Context, _, _ string) (workloadidentity.Credential, error) {
	return workloadidentity.Credential{ClientID: "agenthub-api", ClientSecret: "secret"}, nil
}

// failProvisioner always returns an error.
type failProvisioner struct{}

func (f *failProvisioner) ProvisionRealm(_ context.Context, _, _ string) (workloadidentity.Credential, error) {
	return workloadidentity.Credential{}, errors.New("keycloak unavailable")
}

func TestTenantService_Create_Success(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), &noopProvisioner{}, nil)
	res, err := svc.Create(context.Background(), tenant.CreateTenantRequest{
		ID:   "my-company",
		Name: "My Company",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-company", res.ID)
	assert.Equal(t, "ACTIVE", res.Status)
}

func TestTenantService_Create_InvalidSlug(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), nil, nil)
	_, err := svc.Create(context.Background(), tenant.CreateTenantRequest{
		ID:   "INVALID_SLUG!",
		Name: "Bad",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kebab-case")
}

func TestTenantService_Create_MissingID(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), nil, nil)
	_, err := svc.Create(context.Background(), tenant.CreateTenantRequest{Name: "Test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}

func TestTenantService_Create_MissingName(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), nil, nil)
	_, err := svc.Create(context.Background(), tenant.CreateTenantRequest{ID: "my-co"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestTenantService_Create_KeycloakFails_SetsProvisioningFailed(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), &failProvisioner{}, nil)
	res, err := svc.Create(context.Background(), tenant.CreateTenantRequest{
		ID:   "my-company",
		Name: "My Company",
	})
	// Even when Keycloak fails, Create returns success (soft failure).
	require.NoError(t, err)
	assert.Equal(t, "PROVISIONING_FAILED", res.Status)
}

func TestTenantService_GetByID_NotFound(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), nil, nil)
	_, err := svc.GetByID(context.Background(), "does-not-exist")
	require.ErrorIs(t, err, tenant.ErrNotFound)
}

func TestTenantService_Exists(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), &noopProvisioner{}, nil)
	_, err := svc.Create(context.Background(), tenant.CreateTenantRequest{ID: "acme", Name: "Acme"})
	require.NoError(t, err)

	exists, err := svc.Exists(context.Background(), "acme")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = svc.Exists(context.Background(), "ghost")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTenantService_List(t *testing.T) {
	svc := tenant.NewService(newMockRepo(), &noopProvisioner{}, nil)
	for _, id := range []string{"aa-bb", "cc-dd", "ee-ff"} {
		_, err := svc.Create(context.Background(), tenant.CreateTenantRequest{ID: id, Name: id})
		require.NoError(t, err)
	}
	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
}
