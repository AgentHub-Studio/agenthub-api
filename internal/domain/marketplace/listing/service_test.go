package listing_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/listing"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockListingRepo struct {
	data map[uuid.UUID]listing.Listing
}

func newMockRepo() *mockListingRepo {
	return &mockListingRepo{data: make(map[uuid.UUID]listing.Listing)}
}

func (m *mockListingRepo) FindAll(_ context.Context, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	out := make([]listing.Listing, 0, len(m.data))
	for _, l := range m.data {
		out = append(out, l)
	}
	return out, int64(len(out)), nil
}

func (m *mockListingRepo) FindByType(_ context.Context, t listing.PackageType, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	var out []listing.Listing
	for _, l := range m.data {
		if l.Type == t {
			out = append(out, l)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockListingRepo) FindByCategory(_ context.Context, cat string, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	var out []listing.Listing
	for _, l := range m.data {
		if l.Category == cat {
			out = append(out, l)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockListingRepo) FindByTenant(_ context.Context, tenantID string, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	var out []listing.Listing
	for _, l := range m.data {
		if l.TenantID == tenantID {
			out = append(out, l)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockListingRepo) FindByID(_ context.Context, id uuid.UUID) (listing.Listing, error) {
	l, ok := m.data[id]
	if !ok {
		return listing.Listing{}, listing.ErrNotFound
	}
	return l, nil
}

func (m *mockListingRepo) FindBySlug(_ context.Context, slug string) (listing.Listing, error) {
	for _, l := range m.data {
		if l.Slug == slug {
			return l, nil
		}
	}
	return listing.Listing{}, listing.ErrNotFound
}

func (m *mockListingRepo) Create(_ context.Context, l listing.Listing) (listing.Listing, error) {
	m.data[l.ID] = l
	return l, nil
}

func (m *mockListingRepo) Update(_ context.Context, l listing.Listing) (listing.Listing, error) {
	if _, ok := m.data[l.ID]; !ok {
		return listing.Listing{}, listing.ErrNotFound
	}
	m.data[l.ID] = l
	return l, nil
}

func (m *mockListingRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return listing.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockListingRepo) UpdateRatingStats(_ context.Context, id uuid.UUID, avg float64, count int) error {
	l, ok := m.data[id]
	if !ok {
		return listing.ErrNotFound
	}
	l.AvgRating = avg
	l.ReviewCount = count
	m.data[id] = l
	return nil
}

func TestListingService_Create_AutoSlug(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	res, err := svc.Create(context.Background(), "tenant-1", listing.CreateListingRequest{
		Name:      "My Agent",
		PackageID: uuid.New(),
		Type:      "AGENT",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-agent", res.Slug)
	assert.Equal(t, "ACTIVE", res.Status)
}

func TestListingService_Create_CustomSlug(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	res, err := svc.Create(context.Background(), "tenant-1", listing.CreateListingRequest{
		Name:      "My Agent",
		Slug:      "custom-slug",
		PackageID: uuid.New(),
		Type:      "AGENT",
	})
	require.NoError(t, err)
	assert.Equal(t, "custom-slug", res.Slug)
}

func TestListingService_Create_MissingName(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), "tenant-1", listing.CreateListingRequest{PackageID: uuid.New()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestListingService_GetByID_NotFound(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func TestListingService_Update_Forbidden(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	res, err := svc.Create(context.Background(), "tenant-owner", listing.CreateListingRequest{
		Name: "Agent", PackageID: uuid.New(), Type: "AGENT",
	})
	require.NoError(t, err)

	name := "New Name"
	_, err = svc.Update(context.Background(), res.ID, "other-tenant", listing.UpdateListingRequest{Name: &name})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestListingService_Delete_NotFound(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New(), "tenant-1")
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func TestListingService_ListAll(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), "tenant-1", listing.CreateListingRequest{
			Name: "Agent", PackageID: uuid.New(), Type: "AGENT",
		})
		require.NoError(t, err)
	}
	page, err := svc.ListAll(context.Background(), pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
}

func TestListingService_ListByType(t *testing.T) {
	svc := listing.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), "t1", listing.CreateListingRequest{Name: "A1", PackageID: uuid.New(), Type: "AGENT"})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), "t1", listing.CreateListingRequest{Name: "S1", PackageID: uuid.New(), Type: "SKILL"})
	require.NoError(t, err)

	page, err := svc.ListByType(context.Background(), listing.PackageTypeAgent, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
}
