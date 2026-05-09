// Package listing defines the marketplace listing domain.
package listing

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a listing is not found.
var ErrNotFound = errors.New("listing not found")

// ErrDuplicateSlug é retornado quando já existe um listing com o
// mesmo packageId+slug. Mapeado para 409 no handler.
var ErrDuplicateSlug = errors.New("listing: a listing with this slug already exists for this package")

// ErrValidation é retornado quando o request falha validação semantic
// (ex: name vazio em Update). Mapeado para 422 no handler.
var ErrValidation = errors.New("listing: validation failed")

// ErrForbidden é retornado quando o tenant não pode operar sobre o
// listing (cross-tenant). Mapeado para 403 no handler.
var ErrForbidden = errors.New("listing: forbidden")

// PackageType represents the category of a marketplace listing.
type PackageType string

const (
	PackageTypeAgent         PackageType = "AGENT"
	PackageTypeSkill         PackageType = "SKILL"
	PackageTypeTool          PackageType = "TOOL"
	PackageTypeKnowledgeBase PackageType = "KNOWLEDGE_BASE"
)

// ListingStatus represents the lifecycle state of a listing.
type ListingStatus string

const (
	StatusActive   ListingStatus = "ACTIVE"
	StatusInactive ListingStatus = "INACTIVE"
	StatusRemoved  ListingStatus = "REMOVED"
)

// Listing is the domain entity for a marketplace entry.
type Listing struct {
	ID          uuid.UUID
	TenantID    string
	PackageID   uuid.UUID
	Name        string
	Slug        string
	Description string
	Type        PackageType
	Category    string
	Status      ListingStatus
	AvgRating   float64
	ReviewCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListingResponse is the DTO returned to clients.
type ListingResponse struct {
	ID          uuid.UUID `json:"id"`
	TenantID    string    `json:"tenantId"`
	PackageID   uuid.UUID `json:"packageId"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Category    string    `json:"category"`
	Status      string    `json:"status"`
	AvgRating   float64   `json:"avgRating"`
	ReviewCount int       `json:"reviewCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ResponseFrom converts a Listing to ListingResponse.
func ResponseFrom(l Listing) ListingResponse {
	return ListingResponse{
		ID:          l.ID,
		TenantID:    l.TenantID,
		PackageID:   l.PackageID,
		Name:        l.Name,
		Slug:        l.Slug,
		Description: l.Description,
		Type:        string(l.Type),
		Category:    l.Category,
		Status:      string(l.Status),
		AvgRating:   l.AvgRating,
		ReviewCount: l.ReviewCount,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
	}
}

// CreateListingRequest is the DTO for creating a listing.
type CreateListingRequest struct {
	PackageID   uuid.UUID `json:"packageId"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Category    string    `json:"category"`
}

// UpdateListingRequest is the DTO for updating a listing.
type UpdateListingRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
}
