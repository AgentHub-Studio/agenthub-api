package installation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/installation"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockInstallRepo is an in-memory Repository.
type mockInstallRepo struct {
	data map[uuid.UUID]installation.Installation
}

func newMockInstallRepo() *mockInstallRepo {
	return &mockInstallRepo{data: make(map[uuid.UUID]installation.Installation)}
}

func (m *mockInstallRepo) FindByTenant(_ context.Context, tenantID string, req pagination.PageRequest) ([]installation.Installation, int64, error) {
	var out []installation.Installation
	for _, i := range m.data {
		if i.TenantID == tenantID {
			out = append(out, i)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockInstallRepo) FindByID(_ context.Context, id uuid.UUID) (installation.Installation, error) {
	i, ok := m.data[id]
	if !ok {
		return installation.Installation{}, installation.ErrNotFound
	}
	return i, nil
}

func (m *mockInstallRepo) Create(_ context.Context, i installation.Installation) (installation.Installation, error) {
	i.InstalledAt = time.Now()
	m.data[i.ID] = i
	return i, nil
}

func (m *mockInstallRepo) Uninstall(_ context.Context, id uuid.UUID) error {
	i, ok := m.data[id]
	if !ok {
		return installation.ErrNotFound
	}
	i.Status = installation.InstallStatusUninstalled
	m.data[id] = i
	return nil
}

// Tests

func TestInstallService_Install_Success(t *testing.T) {
	svc := installation.NewService(newMockInstallRepo())
	pkgID := uuid.New()
	resp, err := svc.Install(context.Background(), "tenant-a", installation.InstallRequest{
		PackageID:      pkgID,
		PackageVersion: "1.0.0",
	})
	require.NoError(t, err)
	assert.Equal(t, pkgID, resp.PackageID)
	assert.Equal(t, installation.InstallStatusInstalled, resp.Status)
	assert.Equal(t, "tenant-a", resp.TenantID)
}

func TestInstallService_Install_MissingPackageID(t *testing.T) {
	svc := installation.NewService(newMockInstallRepo())
	_, err := svc.Install(context.Background(), "tenant-a", installation.InstallRequest{
		PackageVersion: "1.0.0",
	})
	require.Error(t, err)
}

func TestInstallService_Install_MissingVersion(t *testing.T) {
	svc := installation.NewService(newMockInstallRepo())
	_, err := svc.Install(context.Background(), "tenant-a", installation.InstallRequest{
		PackageID: uuid.New(),
	})
	require.Error(t, err)
}

func TestInstallService_Uninstall_Success(t *testing.T) {
	svc := installation.NewService(newMockInstallRepo())
	resp, err := svc.Install(context.Background(), "tenant-a", installation.InstallRequest{
		PackageID:      uuid.New(),
		PackageVersion: "1.0.0",
	})
	require.NoError(t, err)
	err = svc.Uninstall(context.Background(), resp.ID, "tenant-a")
	require.NoError(t, err)
}

func TestInstallService_Uninstall_NotFound(t *testing.T) {
	svc := installation.NewService(newMockInstallRepo())
	err := svc.Uninstall(context.Background(), uuid.New(), "tenant-a")
	require.ErrorIs(t, err, installation.ErrNotFound)
}

func TestInstallService_Uninstall_Forbidden(t *testing.T) {
	svc := installation.NewService(newMockInstallRepo())
	resp, err := svc.Install(context.Background(), "owner-tenant", installation.InstallRequest{
		PackageID:      uuid.New(),
		PackageVersion: "1.0.0",
	})
	require.NoError(t, err)
	err = svc.Uninstall(context.Background(), resp.ID, "other-tenant")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestInstallService_ListByTenant(t *testing.T) {
	svc := installation.NewService(newMockInstallRepo())
	for i := 0; i < 2; i++ {
		_, err := svc.Install(context.Background(), "tenant-a", installation.InstallRequest{
			PackageID:      uuid.New(),
			PackageVersion: "1.0.0",
		})
		require.NoError(t, err)
	}
	// install for a different tenant
	_, err := svc.Install(context.Background(), "tenant-b", installation.InstallRequest{
		PackageID:      uuid.New(),
		PackageVersion: "1.0.0",
	})
	require.NoError(t, err)

	page, err := svc.ListByTenant(context.Background(), "tenant-a", pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.TotalElements)
}
