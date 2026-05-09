package abtest

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ABTestResponse is the API representation of an ABTest.
type ABTestResponse struct {
	ID               uuid.UUID  `json:"id"`
	AgentID          uuid.UUID  `json:"agentId"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	ControlVersionID *uuid.UUID `json:"controlVersionId,omitempty"`
	VariantVersionID uuid.UUID  `json:"variantVersionId"`
	TrafficPercent   int        `json:"trafficPercent"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"startedAt"`
	EndedAt          *time.Time `json:"endedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func responseFrom(t ABTest) ABTestResponse {
	return ABTestResponse{
		ID:               t.ID,
		AgentID:          t.AgentID,
		Name:             t.Name,
		Description:      t.Description,
		ControlVersionID: t.ControlVersionID,
		VariantVersionID: t.VariantVersionID,
		TrafficPercent:   t.TrafficPercent,
		Status:           string(t.Status),
		StartedAt:        t.StartedAt,
		EndedAt:          t.EndedAt,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}
}

// CreateABTestRequest is the body for POST /api/agents/:agentId/ab-tests.
type CreateABTestRequest struct {
	AgentID          uuid.UUID  `json:"-"` // injected from URL
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	ControlVersionID *uuid.UUID `json:"controlVersionId,omitempty"`
	VariantVersionID uuid.UUID  `json:"variantVersionId"`
	// TrafficPercent is the percentage (0–100) routed to the variant.
	TrafficPercent int `json:"trafficPercent"`
}

func (r CreateABTestRequest) validate() error {
	if r.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	// Bug 141: name varchar(255) — gate length antes do INSERT.
	if len(r.Name) > 255 {
		return fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(r.Name))
	}
	if r.VariantVersionID == uuid.Nil {
		return fmt.Errorf("%w: variantVersionId is required", ErrValidation)
	}
	if r.TrafficPercent < 0 || r.TrafficPercent > 100 {
		return fmt.Errorf("%w: trafficPercent must be 0–100", ErrValidation)
	}
	return nil
}

// UpdateABTestRequest is the body for PUT /api/agents/:agentId/ab-tests/:id.
type UpdateABTestRequest struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	TrafficPercent *int    `json:"trafficPercent,omitempty"`
	// Status may be "ACTIVE", "PAUSED", or "CONCLUDED".
	Status *string `json:"status,omitempty"`
}
