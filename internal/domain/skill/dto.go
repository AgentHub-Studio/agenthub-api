package skill

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the payload for creating a skill.
type CreateRequest struct {
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	Description  string          `json:"description"`
	Category     string          `json:"category"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
}

// UpdateRequest is the payload for updating a skill.
type UpdateRequest struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Category     string          `json:"category"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
}

// Response is the JSON representation of a Skill.
type Response struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	InputSchema  any       `json:"inputSchema"`
	OutputSchema any       `json:"outputSchema"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ResponseFrom converts a Skill to a Response.
func ResponseFrom(s Skill) Response {
	var in, out any
	_ = json.Unmarshal(s.InputSchema, &in)
	_ = json.Unmarshal(s.OutputSchema, &out)
	return Response{
		ID:           s.ID,
		Name:         s.Name,
		Slug:         s.Slug,
		Description:  s.Description,
		Category:     s.Category,
		InputSchema:  in,
		OutputSchema: out,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}
