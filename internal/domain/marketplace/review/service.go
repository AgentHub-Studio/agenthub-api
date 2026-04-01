package review

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/listing"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ReviewRepository is the persistence interface for reviews.
type ReviewRepository interface {
	FindByListing(ctx context.Context, listingID uuid.UUID, req pagination.PageRequest) ([]Review, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (Review, error)
	Create(ctx context.Context, r Review) (Review, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetRatingStats(ctx context.Context, listingID uuid.UUID) (RatingStats, error)
}

// ListingRepository is the subset of listing persistence needed by review service.
type ListingRepository interface {
	FindAll(ctx context.Context, req pagination.PageRequest) ([]listing.Listing, int64, error)
	FindByType(ctx context.Context, t listing.PackageType, req pagination.PageRequest) ([]listing.Listing, int64, error)
	FindByCategory(ctx context.Context, cat string, req pagination.PageRequest) ([]listing.Listing, int64, error)
	FindBySlug(ctx context.Context, slug string) (listing.Listing, error)
	FindByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) ([]listing.Listing, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (listing.Listing, error)
	Create(ctx context.Context, l listing.Listing) (listing.Listing, error)
	Update(ctx context.Context, l listing.Listing) (listing.Listing, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	UpdateRatingStats(ctx context.Context, id uuid.UUID, avg float64, count int) error
}

// Service implements business logic for marketplace reviews.
type Service struct {
	reviews  ReviewRepository
	listings ListingRepository
}

// NewService creates a new Service.
func NewService(reviews ReviewRepository, listings ListingRepository) *Service {
	return &Service{reviews: reviews, listings: listings}
}

// Create adds a new review for a listing.
func (s *Service) Create(ctx context.Context, listingID uuid.UUID, tenantID string, req CreateRequest) (ReviewResponse, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return ReviewResponse{}, fmt.Errorf("review: rating must be between 1 and 5")
	}
	l, err := s.listings.FindByID(ctx, listingID)
	if err != nil {
		return ReviewResponse{}, err
	}
	_ = l
	rev := Review{
		ID:        uuid.New(),
		ListingID: listingID,
		TenantID:  tenantID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}
	created, err := s.reviews.Create(ctx, rev)
	if err != nil {
		return ReviewResponse{}, err
	}
	// update aggregated rating
	stats, err := s.reviews.GetRatingStats(ctx, listingID)
	if err == nil {
		_ = s.listings.UpdateRatingStats(ctx, listingID, stats.Avg, stats.Count)
	}
	return ResponseFrom(created), nil
}

// Delete removes a review owned by the given tenant.
func (s *Service) Delete(ctx context.Context, listingID uuid.UUID, reviewID uuid.UUID, tenantID string) error {
	rev, err := s.reviews.FindByID(ctx, reviewID)
	if err != nil {
		return err
	}
	if rev.TenantID != tenantID {
		return fmt.Errorf("review: forbidden")
	}
	if err := s.reviews.Delete(ctx, reviewID); err != nil {
		return err
	}
	// update aggregated rating
	stats, err := s.reviews.GetRatingStats(ctx, listingID)
	if err == nil {
		_ = s.listings.UpdateRatingStats(ctx, listingID, stats.Avg, stats.Count)
	}
	return nil
}

// ListByListing returns paginated reviews for a listing.
func (s *Service) ListByListing(ctx context.Context, listingID uuid.UUID, req pagination.PageRequest) (pagination.Page[ReviewResponse], error) {
	reviews, total, err := s.reviews.FindByListing(ctx, listingID, req)
	if err != nil {
		return pagination.Page[ReviewResponse]{}, fmt.Errorf("review: list by listing: %w", err)
	}
	responses := make([]ReviewResponse, len(reviews))
	for i, r := range reviews {
		responses[i] = ResponseFrom(r)
	}
	return pagination.NewPage(responses, total, req), nil
}
