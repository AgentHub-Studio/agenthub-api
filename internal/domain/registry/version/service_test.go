package version_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/version"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockVersionRepo is an in-memory implementation of VersionRepository.
type mockVersionRepo struct {
	data map[string]version.PackageVersion // key: packageID+":"+versionStr
}

func newMockVersionRepo() *mockVersionRepo {
	return &mockVersionRepo{data: make(map[string]version.PackageVersion)}
}

func key(packageID uuid.UUID, ver string) string {
	return packageID.String() + ":" + ver
}

func (m *mockVersionRepo) ListByPackage(_ context.Context, packageID uuid.UUID) ([]version.PackageVersion, error) {
	var out []version.PackageVersion
	for _, v := range m.data {
		if v.PackageID == packageID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *mockVersionRepo) GetByVersion(_ context.Context, packageID uuid.UUID, versionStr string) (version.PackageVersion, error) {
	v, ok := m.data[key(packageID, versionStr)]
	if !ok {
		return version.PackageVersion{}, version.ErrNotFound
	}
	return v, nil
}

func (m *mockVersionRepo) Create(_ context.Context, v version.PackageVersion) (version.PackageVersion, error) {
	v.ID = uuid.New()
	m.data[key(v.PackageID, v.Version)] = v
	return v, nil
}

func (m *mockVersionRepo) Delete(_ context.Context, packageID uuid.UUID, versionStr string) error {
	k := key(packageID, versionStr)
	if _, ok := m.data[k]; !ok {
		return version.ErrNotFound
	}
	delete(m.data, k)
	return nil
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
	p, ok := m.data[id]
	if !ok {
		return pkg.Package{}, pkg.ErrNotFound
	}
	p.Name = name
	p.Tags = tags
	m.data[id] = p
	return p, nil
}
func (m *mockPkgRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.data, id)
	return nil
}
func (m *mockPkgRepo) UpdateLatestVersion(_ context.Context, id uuid.UUID, ver string) error {
	if p, ok := m.data[id]; ok {
		p.LatestVersion = ver
		m.data[id] = p
	}
	return nil
}

// Helpers

func seedPackage(t *testing.T, pr *mockPkgRepo, tenantID string) pkg.Package {
	t.Helper()
	p := pkg.Package{
		ID:             uuid.New(),
		Name:           "Test Package",
		Slug:           "test-package",
		Type:           pkg.PackageTypeAgent,
		Visibility:     pkg.PackageVisibilityPublic,
		AuthorTenantID: tenantID,
	}
	pr.data[p.ID] = p
	return p
}

// Tests

func TestVersionService_Publish_Success(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	resp, err := svc.Publish(context.Background(), parent.ID, version.PublishVersionRequest{
		Version:     "1.0.0",
		Changelog:   "initial release",
		StoragePath: "packages/test/1.0.0.tgz",
		Checksum:    "abc123",
	}, "owner")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, parent.ID, resp.PackageID)
	assert.Equal(t, "owner", resp.PublishedBy)
}

func TestVersionService_Publish_ValidationMissingVersion(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	_, err := svc.Publish(context.Background(), parent.ID, version.PublishVersionRequest{}, "owner")
	require.Error(t, err)
	var ve *version.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "version", ve.Field)
}

func TestVersionService_Publish_ValidationInvalidSemver(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	_, err := svc.Publish(context.Background(), parent.ID, version.PublishVersionRequest{Version: "v1.0"}, "owner")
	require.Error(t, err)
	var ve *version.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "version", ve.Field)
}

func TestVersionService_Publish_Forbidden(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	_, err := svc.Publish(context.Background(), parent.ID, version.PublishVersionRequest{Version: "1.0.0"}, "intruder")
	require.Error(t, err)
	var fe *version.ForbiddenError
	require.ErrorAs(t, err, &fe)
}

func TestVersionService_Publish_PackageNotFound(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()

	svc := version.NewService(vr, pr)
	_, err := svc.Publish(context.Background(), uuid.New(), version.PublishVersionRequest{Version: "1.0.0"}, "owner")
	require.ErrorIs(t, err, pkg.ErrNotFound)
}

func TestVersionService_GetByVersion_NotFound(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	_, err := svc.GetByVersion(context.Background(), parent.ID, "9.9.9")
	require.ErrorIs(t, err, version.ErrNotFound)
}

func TestVersionService_ListByPackage(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		_, err := svc.Publish(context.Background(), parent.ID, version.PublishVersionRequest{Version: v}, "owner")
		require.NoError(t, err)
	}

	versions, err := svc.ListByPackage(context.Background(), parent.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 3)
}

func TestVersionService_Delete_Success(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	_, err := svc.Publish(context.Background(), parent.ID, version.PublishVersionRequest{Version: "1.0.0"}, "owner")
	require.NoError(t, err)

	err = svc.Delete(context.Background(), parent.ID, "1.0.0", "owner")
	require.NoError(t, err)
}

func TestVersionService_Delete_Forbidden(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	_, err := svc.Publish(context.Background(), parent.ID, version.PublishVersionRequest{Version: "1.0.0"}, "owner")
	require.NoError(t, err)

	err = svc.Delete(context.Background(), parent.ID, "1.0.0", "intruder")
	require.Error(t, err)
	var fe *version.ForbiddenError
	require.ErrorAs(t, err, &fe)
}

func TestVersionService_Delete_NotFound(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPkgRepo()
	parent := seedPackage(t, pr, "owner")

	svc := version.NewService(vr, pr)
	err := svc.Delete(context.Background(), parent.ID, "9.9.9", "owner")
	require.ErrorIs(t, err, version.ErrNotFound)
}
