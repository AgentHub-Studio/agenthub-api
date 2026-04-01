package review_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/listing"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/review"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockReviewRepo is an in-memory review.Repository.
type mockReviewRepo struct {
	reviews map[uuid.UUID]review.Review
}

func newMockReviewRepo() *mockReviewRepo {
	return &mockReviewRepo{reviews: make(map[uuid.UUID]review.Review)}
}

func (m *mockReviewRepo) FindByListing(_ context.Context, listingID uuid.UUID, req pagination.PageRequest) ([]review.Review, int64, error) {
	var out []review.Review
	for _, r := range m.reviews {
		if r.ListingID == listingID {
			out = append(out, r)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockReviewRepo) FindByID(_ context.Context, id uuid.UUID) (review.Review, error) {
	r, ok := m.reviews[id]
	if !ok {
		return review.Review{}, review.ErrNotFound
	}
	return r, nil
}

func (m *mockReviewRepo) Create(_ context.Context, r review.Review) (review.Review, error) {
	for _, existing := range m.reviews {
		if existing.TenantID == r.TenantID && existing.ListingID == r.ListingID {
			return review.Review{}, review.ErrDuplicate
		}
	}
	r.CreatedAt = time.Now()
	m.reviews[r.ID] = r
	return r, nil
}

func (m *mockReviewRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.reviews[id]; !ok {
		return review.ErrNotFound
	}
	delete(m.reviews, id)
	return nil
}

func (m *mockReviewRepo) GetRatingStats(_ context.Context, listingID uuid.UUID) (review.RatingStats, error) {
	var total int
	var sum int
	for _, r := range m.reviews {
		if r.ListingID == listingID {
			sum += r.Rating
			total++
		}
	}
	avg := 0.0
	if total > 0 {
		avg = float64(sum) / float64(total)
	}
	return review.RatingStats{Avg: avg, Count: total}, nil
}

// mockListingRepo is a stub listing.Repository used by review service.
type mockListingRepo struct {
	listings map[uuid.UUID]listing.Listing
}

func newMockListingRepo() *mockListingRepo {
	return &mockListingRepo{listings: make(map[uuid.UUID]listing.Listing)}
}

func (m *mockListingRepo) FindAll(_ context.Context, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	return nil, 0, nil
}
func (m *mockListingRepo) FindByType(_ context.Context, t listing.PackageType, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	return nil, 0, nil
}
func (m *mockListingRepo) FindByCategory(_ context.Context, cat string, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	return nil, 0, nil
}
func (m *mockListingRepo) FindBySlug(_ context.Context, slug string) (listing.Listing, error) {
	return listing.Listing{}, listing.ErrNotFound
}
func (m *mockListingRepo) FindByTenant(_ context.Context, tenantID string, req pagination.PageRequest) ([]listing.Listing, int64, error) {
	return nil, 0, nil
}
func (m *mockListingRepo) FindByID(_ context.Context, id uuid.UUID) (listing.Listing, error) {
	l, ok := m.listings[id]
	if !ok {
		return listing.Listing{}, listing.ErrNotFound
	}
	return l, nil
}
func (m *mockListingRepo) Create(_ context.Context, l listing.Listing) (listing.Listing, error) {
	m.listings[l.ID] = l
	return l, nil
}
func (m *mockListingRepo) Update(_ context.Context, l listing.Listing) (listing.Listing, error) {
	m.listings[l.ID] = l
	return l, nil
}
func (m *mockListingRepo) SoftDelete(_ context.Context, id uuid.UUID) error { return nil }
func (m *mockListingRepo) UpdateRatingStats(_ context.Context, id uuid.UUID, avg float64, count int) error {
	if l, ok := m.listings[id]; ok {
		l.AvgRating = avg
		l.ReviewCount = count
		m.listings[id] = l
	}
	return nil
}

// Tests

func TestReviewService_Create_Success(t *testing.T) {
	rr := newMockReviewRepo()
	lr := newMockListingRepo()
	listingID := uuid.New()
	lr.listings[listingID] = listing.Listing{ID: listingID, TenantID: "tenant-a", Status: listing.StatusActive}

	svc := review.NewService(rr, lr)
	resp, err := svc.Create(context.Background(), listingID, "tenant-a", review.CreateRequest{Rating: 5, Comment: "great"})
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Rating)
	assert.Equal(t, listingID, resp.ListingID)
}

func TestReviewService_Create_InvalidRating(t *testing.T) {
	rr := newMockReviewRepo()
	lr := newMockListingRepo()
	listingID := uuid.New()
	lr.listings[listingID] = listing.Listing{ID: listingID}

	svc := review.NewService(rr, lr)
	_, err := svc.Create(context.Background(), listingID, "tenant-a", review.CreateRequest{Rating: 0})
	require.Error(t, err)
}

func TestReviewService_Create_ListingNotFound(t *testing.T) {
	svc := review.NewService(newMockReviewRepo(), newMockListingRepo())
	_, err := svc.Create(context.Background(), uuid.New(), "tenant-a", review.CreateRequest{Rating: 4})
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func TestReviewService_Create_Duplicate(t *testing.T) {
	rr := newMockReviewRepo()
	lr := newMockListingRepo()
	listingID := uuid.New()
	lr.listings[listingID] = listing.Listing{ID: listingID}

	svc := review.NewService(rr, lr)
	_, err := svc.Create(context.Background(), listingID, "tenant-a", review.CreateRequest{Rating: 3})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), listingID, "tenant-a", review.CreateRequest{Rating: 4})
	require.ErrorIs(t, err, review.ErrDuplicate)
}

func TestReviewService_Delete_Success(t *testing.T) {
	rr := newMockReviewRepo()
	lr := newMockListingRepo()
	listingID := uuid.New()
	lr.listings[listingID] = listing.Listing{ID: listingID}

	svc := review.NewService(rr, lr)
	resp, err := svc.Create(context.Background(), listingID, "tenant-a", review.CreateRequest{Rating: 4})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), listingID, resp.ID, "tenant-a")
	require.NoError(t, err)
}

func TestReviewService_Delete_Forbidden(t *testing.T) {
	rr := newMockReviewRepo()
	lr := newMockListingRepo()
	listingID := uuid.New()
	lr.listings[listingID] = listing.Listing{ID: listingID}

	svc := review.NewService(rr, lr)
	resp, err := svc.Create(context.Background(), listingID, "tenant-a", review.CreateRequest{Rating: 4})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), listingID, resp.ID, "other-tenant")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestReviewService_Delete_NotFound(t *testing.T) {
	svc := review.NewService(newMockReviewRepo(), newMockListingRepo())
	err := svc.Delete(context.Background(), uuid.New(), uuid.New(), "tenant-a")
	require.ErrorIs(t, err, review.ErrNotFound)
}

func TestReviewService_ListByListing(t *testing.T) {
	rr := newMockReviewRepo()
	lr := newMockListingRepo()
	listingID := uuid.New()
	lr.listings[listingID] = listing.Listing{ID: listingID}

	svc := review.NewService(rr, lr)
	_, err := svc.Create(context.Background(), listingID, "tenant-a", review.CreateRequest{Rating: 5})
	require.NoError(t, err)

	page, err := svc.ListByListing(context.Background(), listingID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
}
