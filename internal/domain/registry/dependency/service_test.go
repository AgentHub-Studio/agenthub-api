package dependency_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/dependency"
	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockDepRepo is an in-memory implementation of DependencyRepository.
type mockDepRepo struct {
	deps  map[uuid.UUID]dependency.PackageDependency
	names map[uuid.UUID]struct{ name, slug string }
}

func newMockDepRepo() *mockDepRepo {
	return &mockDepRepo{
		deps:  make(map[uuid.UUID]dependency.PackageDependency),
		names: make(map[uuid.UUID]struct{ name, slug string }),
	}
}

func (m *mockDepRepo) ListByPackage(_ context.Context, packageID uuid.UUID) ([]dependency.PackageDependency, error) {
	var out []dependency.PackageDependency
	for _, d := range m.deps {
		if d.PackageID == packageID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (m *mockDepRepo) Create(_ context.Context, d dependency.PackageDependency) (dependency.PackageDependency, error) {
	d.ID = uuid.New()
	m.deps[d.ID] = d
	return d, nil
}

func (m *mockDepRepo) Delete(_ context.Context, packageID, depID uuid.UUID) error {
	for id, d := range m.deps {
		if d.PackageID == packageID && d.ID == depID {
			delete(m.deps, id)
			return nil
		}
	}
	return dependency.ErrNotFound
}

func (m *mockDepRepo) GetPackageName(_ context.Context, packageID uuid.UUID) (string, string, error) {
	info, ok := m.names[packageID]
	if !ok {
		return "", "", dependency.ErrNotFound
	}
	return info.name, info.slug, nil
}

// mockPkgRepo is a minimal in-memory implementation of pkg.PackageRepository.
type mockPkgRepo struct {
	data map[uuid.UUID]pkg.Package
}

func newMockPkgRepo() *mockPkgRepo {
	return &mockPkgRepo{data: make(map[uuid.UUID]pkg.Package)}
}

func (m *mockPkgRepo) ListPublic(_ context.Context, req pagination.PageRequest) ([]pkg.Package, int64, error) {
	return nil, 0, nil
}
func (m *mockPkgRepo) GetByID(_ context.Context, id uuid.UUID) (pkg.Package, error) {
	p, ok := m.data[id]
	if !ok {
		return pkg.Package{}, pkg.ErrNotFound
	}
	return p, nil
}
func (m *mockPkgRepo) GetBySlug(_ context.Context, slug string) (pkg.Package, error) {
	return pkg.Package{}, pkg.ErrNotFound
}
func (m *mockPkgRepo) ListByTenant(_ context.Context, tenantID string, req pagination.PageRequest) ([]pkg.Package, int64, error) {
	return nil, 0, nil
}
func (m *mockPkgRepo) Create(_ context.Context, p pkg.Package) (pkg.Package, error) {
	p.ID = uuid.New()
	m.data[p.ID] = p
	return p, nil
}
func (m *mockPkgRepo) Update(_ context.Context, id uuid.UUID, name, description, visibility string, tags []string) (pkg.Package, error) {
	return pkg.Package{}, nil
}
func (m *mockPkgRepo) Delete(_ context.Context, id uuid.UUID) error { return nil }
func (m *mockPkgRepo) UpdateLatestVersion(_ context.Context, id uuid.UUID, ver string) error {
	return nil
}

func seedPackage(t *testing.T, pr *mockPkgRepo, tenantID string) pkg.Package {
	t.Helper()
	p := pkg.Package{
		ID:             uuid.New(),
		Name:           "Test Pkg",
		Slug:           "test-pkg-" + uuid.New().String()[:8],
		Type:           pkg.PackageTypeAgent,
		AuthorTenantID: tenantID,
	}
	pr.data[p.ID] = p
	return p
}

// Tests

func TestDepService_Add_Success(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")
	dep := seedPackage(t, pr, "other")

	svc := dependency.NewService(dr, pr)
	resp, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{
		DependencyID:      dep.ID,
		VersionConstraint: ">=1.0.0",
	}, "owner")
	require.NoError(t, err)
	assert.Equal(t, parent.ID, resp.PackageID)
	assert.Equal(t, dep.ID, resp.DependencyID)
	assert.Equal(t, ">=1.0.0", resp.VersionConstraint)
}

func TestDepService_Add_MissingVersionConstraint(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")
	dep := seedPackage(t, pr, "other")

	svc := dependency.NewService(dr, pr)
	_, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{
		DependencyID:      dep.ID,
		VersionConstraint: "",
	}, "owner")
	require.Error(t, err)
	var ve *dependency.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "versionConstraint", ve.Field)
}

func TestDepService_Add_SelfDependency(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := dependency.NewService(dr, pr)
	_, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{
		DependencyID:      parent.ID, // same package
		VersionConstraint: ">=1.0.0",
	}, "owner")
	require.Error(t, err)
	var ve *dependency.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "dependencyId", ve.Field)
}

func TestDepService_Add_Forbidden(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")
	dep := seedPackage(t, pr, "other")

	svc := dependency.NewService(dr, pr)
	_, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{
		DependencyID:      dep.ID,
		VersionConstraint: ">=1.0.0",
	}, "intruder")
	require.Error(t, err)
	var fe *dependency.ForbiddenError
	require.ErrorAs(t, err, &fe)
}

func TestDepService_Add_DependencyNotFound(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := dependency.NewService(dr, pr)
	_, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{
		DependencyID:      uuid.New(), // nonexistent
		VersionConstraint: ">=1.0.0",
	}, "owner")
	require.Error(t, err)
}

func TestDepService_List(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")
	dep1 := seedPackage(t, pr, "other")
	dep2 := seedPackage(t, pr, "other")

	svc := dependency.NewService(dr, pr)
	_, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{DependencyID: dep1.ID, VersionConstraint: ">=1.0.0"}, "owner")
	require.NoError(t, err)
	_, err = svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{DependencyID: dep2.ID, VersionConstraint: ">=2.0.0"}, "owner")
	require.NoError(t, err)

	deps, err := svc.List(context.Background(), parent.ID)
	require.NoError(t, err)
	assert.Len(t, deps, 2)
}

func TestDepService_Remove_Success(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")
	dep := seedPackage(t, pr, "other")

	svc := dependency.NewService(dr, pr)
	resp, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{DependencyID: dep.ID, VersionConstraint: ">=1.0.0"}, "owner")
	require.NoError(t, err)

	err = svc.Remove(context.Background(), parent.ID, resp.ID, "owner")
	require.NoError(t, err)
}

func TestDepService_Remove_Forbidden(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")
	dep := seedPackage(t, pr, "other")

	svc := dependency.NewService(dr, pr)
	resp, err := svc.Add(context.Background(), parent.ID, dependency.AddDependencyRequest{DependencyID: dep.ID, VersionConstraint: ">=1.0.0"}, "owner")
	require.NoError(t, err)

	err = svc.Remove(context.Background(), parent.ID, resp.ID, "intruder")
	require.Error(t, err)
	var fe *dependency.ForbiddenError
	require.ErrorAs(t, err, &fe)
}

func TestDepService_Remove_NotFound(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := dependency.NewService(dr, pr)
	err := svc.Remove(context.Background(), parent.ID, uuid.New(), "owner")
	require.ErrorIs(t, err, dependency.ErrNotFound)
}

func TestDepService_Resolve_NotFound(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()

	svc := dependency.NewService(dr, pr)
	_, err := svc.Resolve(context.Background(), uuid.New())
	require.ErrorIs(t, err, dependency.ErrNotFound)
}

func TestDepService_Resolve_Success(t *testing.T) {
	dr := newMockDepRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	// Register names in dep repo for resolution
	dr.names[parent.ID] = struct{ name, slug string }{"Parent", "parent"}

	svc := dependency.NewService(dr, pr)
	tree, err := svc.Resolve(context.Background(), parent.ID)
	require.NoError(t, err)
	assert.Equal(t, parent.ID, tree.PackageID)
	assert.Equal(t, "Parent", tree.Name)
}
