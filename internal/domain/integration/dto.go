package integration

import (
	"time"

	"github.com/google/uuid"
)

// Response is the public API representation of an integration catalog entry.
type Response struct {
	ID          uuid.UUID         `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Type        IntegrationType   `json:"type"`
	Description string            `json:"description"`
	Summary     string            `json:"summary"`
	Enabled     bool              `json:"enabled"`
	Advanced    bool              `json:"advanced"`
	Origin      IntegrationOrigin `json:"origin"`
	SourceKind  SourceKind        `json:"sourceKind"`
	LegacyPath  string            `json:"legacyPath"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// ResponseFrom converts the normalized domain model to the public DTO.
func ResponseFrom(item Integration) Response {
	return Response{
		ID:          item.ID,
		Name:        item.Name,
		Slug:        item.Slug,
		Type:        item.Type,
		Description: item.Description,
		Summary:     item.Summary,
		Enabled:     item.Enabled,
		Advanced:    item.Advanced,
		Origin:      item.Origin,
		SourceKind:  item.SourceKind,
		LegacyPath:  item.LegacyPath,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}
