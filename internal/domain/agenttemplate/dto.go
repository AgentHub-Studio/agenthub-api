package agenttemplate

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TemplateResponse is the JSON response for an AgentTemplate.
type TemplateResponse struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Description    string          `json:"description"`
	Category       string          `json:"category"`
	IsBuiltin      bool            `json:"isBuiltin"`
	DefinitionJSON json.RawMessage `json:"definition"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// ResponseFrom converts an AgentTemplate to TemplateResponse.
func ResponseFrom(t AgentTemplate) TemplateResponse {
	def := t.DefinitionJSON
	if len(def) == 0 {
		def = json.RawMessage(`{}`)
	}
	return TemplateResponse{
		ID:             t.ID,
		Name:           t.Name,
		Slug:           t.Slug,
		Description:    t.Description,
		Category:       t.Category,
		IsBuiltin:      t.IsBuiltin,
		DefinitionJSON: def,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

// CreateTemplateRequest is the payload for creating a new tenant-owned template.
type CreateTemplateRequest struct {
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Definition  json.RawMessage `json:"definition"`
}

// InstantiateRequest is the optional payload for instantiating an agent from a template.
// All fields override the template defaults when provided.
type InstantiateRequest struct {
	// Name overrides the template name for the new agent.
	Name string `json:"name,omitempty"`
	// Description overrides the template description.
	Description string `json:"description,omitempty"`
}

// InstantiateResponse is returned after successfully instantiating an agent template.
type InstantiateResponse struct {
	AgentID uuid.UUID `json:"agentId"`
	Name    string    `json:"name"`
	Slug    string    `json:"slug"`
}
