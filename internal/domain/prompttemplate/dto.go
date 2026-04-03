package prompttemplate

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Response is the JSON response envelope for a PromptTemplate.
type Response struct {
	ID            uuid.UUID       `json:"id"`
	AgentID       *uuid.UUID      `json:"agentId,omitempty"`
	Name          string          `json:"name"`
	Slug          string          `json:"slug"`
	Description   string          `json:"description"`
	Content       string          `json:"content"`
	Category      string          `json:"category"`
	ModelOverride *string         `json:"modelOverride,omitempty"`
	AllowedTools  json.RawMessage `json:"allowedTools,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

// ResponseFrom converts a PromptTemplate entity to Response.
func ResponseFrom(t PromptTemplate) Response {
	return Response{
		ID:            t.ID,
		AgentID:       t.AgentID,
		Name:          t.Name,
		Slug:          t.Slug,
		Description:   t.Description,
		Content:       t.Content,
		Category:      string(t.Category),
		ModelOverride: t.ModelOverride,
		AllowedTools:  t.AllowedTools,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

// CreateRequest is the JSON body for creating a prompt template.
type CreateRequest struct {
	Name          string          `json:"name"`
	Slug          string          `json:"slug"`
	Description   string          `json:"description"`
	Content       string          `json:"content"`
	Category      string          `json:"category,omitempty"`
	ModelOverride *string         `json:"modelOverride,omitempty"`
	AllowedTools  json.RawMessage `json:"allowedTools,omitempty"`
}

// UpdateRequest is the JSON body for updating a prompt template.
type UpdateRequest struct {
	Name          *string         `json:"name,omitempty"`
	Slug          *string         `json:"slug,omitempty"`
	Description   *string         `json:"description,omitempty"`
	Content       *string         `json:"content,omitempty"`
	Category      *string         `json:"category,omitempty"`
	ModelOverride *string         `json:"modelOverride,omitempty"`
	AllowedTools  json.RawMessage `json:"allowedTools,omitempty"`
}
