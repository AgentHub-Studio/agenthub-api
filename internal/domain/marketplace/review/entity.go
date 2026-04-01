// Package review provides marketplace review domain logic.
package review

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a review is not found.
var ErrNotFound = errors.New("review not found")

// ErrDuplicate is returned when a tenant already reviewed a listing.
var ErrDuplicate = errors.New("review already exists")

// Review is the domain entity for a marketplace rating & review.
type Review struct {
	ID        uuid.UUID
	ListingID uuid.UUID
	TenantID  string
	Rating    int
	Comment   string
	CreatedAt time.Time
}

// ReviewResponse is the DTO returned to clients.
type ReviewResponse struct {
	ID        uuid.UUID `json:"id"`
	ListingID uuid.UUID `json:"listingId"`
	TenantID  string    `json:"tenantId"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}

// ResponseFrom converts a Review to ReviewResponse.
func ResponseFrom(r Review) ReviewResponse {
	return ReviewResponse(r)
}

// CreateRequest is the DTO for creating a review.
type CreateRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// RatingStats holds aggregated rating data for a listing.
type RatingStats struct {
	Avg   float64
	Count int
}
