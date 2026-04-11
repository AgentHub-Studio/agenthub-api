package pkg_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockPkgRepo is an in-memory implementation of PackageRepository.
type mockPkgRepo struct {
	data map[uuid.UUID]pkg.Package
}

func newMockPkgRepo() *mockPkgRepo {
	return &mockPkgRepo{data: make(map[uuid.UUID]pkg.Package)}
}

func (m *mockPkgRepo) ListPublic(_ context.Context, req pagination.PageRequest) ([]pkg.Package, int64, error) {
	var out []pkg.Package
	for _, p := range m.data {
		if p.Visibility == pkg.PackageVisibilityPublic {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockPkgRepo) GetByID(_ context.Context, id uuid.UUID) (pkg.Package, error) {
	p, ok := m.data[id]
	if !ok {
		return pkg.Package{}, pkg.ErrNotFound
	}
	return p, nil
}

func (m *mockPkgRepo) GetBySlug(_ context.Context, slug string) (pkg.Package, error) {
	for _, p := range m.data {
		if p.Slug == slug {
			return p, nil
		}
	}
	return pkg.Package{}, pkg.ErrNotFound
}

func (m *mockPkgRepo) ListByTenant(_ context.Context, tenantID string, req pagination.PageRequest) ([]pkg.Package, int64, error) {
	var out []pkg.Package
	for _, p := range m.data {
		if p.AuthorTenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockPkgRepo) Create(_ context.Context, p pkg.Package) (pkg.Package, error) {
	p.ID = uuid.New()
	m.data[p.ID] = p
	return p, nil
}

func (m *mockPkgRepo) Update(_ context.Context, id uuid.UUID, name, description, visibility string) (pkg.Package, error) {
	p, ok := m.data[id]
	if !ok {
		return pkg.Package{}, pkg.ErrNotFound
	}
	p.Name = name
	p.Description = description
	p.Visibility = pkg.PackageVisibility(visibility)
	m.data[id] = p
	return p, nil
}

func (m *mockPkgRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return pkg.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockPkgRepo) UpdateLatestVersion(_ context.Context, id uuid.UUID, version string) error {
	p, ok := m.data[id]
	if !ok {
		return pkg.ErrNotFound
	}
	p.LatestVersion = version
	m.data[id] = p
	return nil
}

func (m *mockPkgRepo) Search(_ context.Context, query string, pkgType *string, req pagination.PageRequest) ([]pkg.Package, int64, error) {
	var out []pkg.Package
	for _, p := range m.data {
		if p.Visibility == pkg.PackageVisibilityPublic {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

// Tests

func TestPkgService_Create_Success(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	resp, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name:       "My Agent",
		Slug:       "my-agent",
		Type:       "AGENT",
		Visibility: "PUBLIC",
	}, "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "My Agent", resp.Name)
	assert.Equal(t, "my-agent", resp.Slug)
	assert.Equal(t, "AGENT", resp.Type)
	assert.Equal(t, "PUBLIC", resp.Visibility)
	assert.Equal(t, "tenant-1", resp.AuthorTenantID)
}

func TestPkgService_Create_ValidationMissingName(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	_, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Slug: "my-agent",
		Type: "AGENT",
	}, "tenant-1")
	require.Error(t, err)
	var ve *pkg.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "name", ve.Field)
}

func TestPkgService_Create_ValidationMissingSlug(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	_, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "My Agent",
		Type: "AGENT",
	}, "tenant-1")
	require.Error(t, err)
	var ve *pkg.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "slug", ve.Field)
}

func TestPkgService_Create_ValidationInvalidSlug(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	_, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "My Agent",
		Slug: "My Agent", // invalid — has spaces and capitals
		Type: "AGENT",
	}, "tenant-1")
	require.Error(t, err)
	var ve *pkg.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "slug", ve.Field)
}

func TestPkgService_Create_ValidationInvalidType(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	_, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "My Agent",
		Slug: "my-agent",
		Type: "INVALID",
	}, "tenant-1")
	require.Error(t, err)
	var ve *pkg.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "type", ve.Field)
}

func TestPkgService_GetByID_NotFound(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, pkg.ErrNotFound)
}

func TestPkgService_GetBySlug_NotFound(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	_, err := svc.GetBySlug(context.Background(), "nonexistent")
	require.ErrorIs(t, err, pkg.ErrNotFound)
}

func TestPkgService_ListPublic(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	_, err := svc.Create(context.Background(), pkg.CreatePackageRequest{Name: "Public", Slug: "public-pkg", Type: "AGENT", Visibility: "PUBLIC"}, "t1")
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), pkg.CreatePackageRequest{Name: "Private", Slug: "private-pkg", Type: "AGENT", Visibility: "PRIVATE"}, "t1")
	require.NoError(t, err)

	page, err := svc.ListPublic(context.Background(), pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
	assert.Equal(t, "Public", page.Content[0].Name)
}

func TestPkgService_ListByTenant(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
			Name: "Pkg", Slug: uuid.New().String()[:8], Type: "SKILL",
		}, "tenant-a")
		require.NoError(t, err)
	}
	_, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "Other", Slug: "other-slug", Type: "TOOL",
	}, "tenant-b")
	require.NoError(t, err)

	page, err := svc.ListByTenant(context.Background(), "tenant-a", pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
}

func TestPkgService_Update_Success(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	resp, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "Old Name", Slug: "old-slug", Type: "AGENT",
	}, "owner-tenant")
	require.NoError(t, err)

	newName := "New Name"
	updated, err := svc.Update(context.Background(), resp.ID, pkg.UpdatePackageRequest{Name: &newName}, "owner-tenant")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
}

func TestPkgService_Update_Forbidden(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	resp, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "Pkg", Slug: "some-pkg", Type: "AGENT",
	}, "owner-tenant")
	require.NoError(t, err)

	newName := "Hacked"
	_, err = svc.Update(context.Background(), resp.ID, pkg.UpdatePackageRequest{Name: &newName}, "other-tenant")
	require.Error(t, err)
	var fe *pkg.ForbiddenError
	require.ErrorAs(t, err, &fe)
}

func TestPkgService_Delete_Success(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	resp, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "Pkg", Slug: "del-pkg", Type: "TOOL",
	}, "owner")
	require.NoError(t, err)

	err = svc.Delete(context.Background(), resp.ID, "owner")
	require.NoError(t, err)

	_, err = svc.GetByID(context.Background(), resp.ID)
	require.ErrorIs(t, err, pkg.ErrNotFound)
}

func TestPkgService_Delete_Forbidden(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	resp, err := svc.Create(context.Background(), pkg.CreatePackageRequest{
		Name: "Pkg", Slug: "del-forbidden", Type: "TOOL",
	}, "owner")
	require.NoError(t, err)

	err = svc.Delete(context.Background(), resp.ID, "intruder")
	require.Error(t, err)
	var fe *pkg.ForbiddenError
	require.ErrorAs(t, err, &fe)
}

func TestPkgService_Delete_NotFound(t *testing.T) {
	svc := pkg.NewService(newMockPkgRepo())
	err := svc.Delete(context.Background(), uuid.New(), "owner")
	require.ErrorIs(t, err, pkg.ErrNotFound)
}
