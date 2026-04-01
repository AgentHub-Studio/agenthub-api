package tool

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the payload for creating a tool.
type CreateRequest struct {
	Name        string          `json:"name"`
	Type        ToolType        `json:"type"`
	Config      json.RawMessage `json:"config"`
	Description string          `json:"description"`
	Labels      []string        `json:"labels"`
}

// UpdateRequest is the payload for updating a tool.
type UpdateRequest struct {
	Name        string          `json:"name"`
	Type        ToolType        `json:"type"`
	Config      json.RawMessage `json:"config"`
	Description string          `json:"description"`
	Labels      []string        `json:"labels"`
}

// BindRequest is the payload for binding a tool to a skill.
type BindRequest struct {
	ToolID   uuid.UUID `json:"toolId"`
	Priority int       `json:"priority"`
	IsActive *bool     `json:"isActive"`
}

// Response is the JSON representation of a Tool.
type Response struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        ToolType  `json:"type"`
	Config      any       `json:"config"`
	Description string    `json:"description"`
	Labels      []string  `json:"labels"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SkillToolResponse is the JSON representation of a SkillTool binding.
type SkillToolResponse struct {
	ID        uuid.UUID    `json:"id"`
	SkillID   uuid.UUID    `json:"skillId"`
	Tool      Response     `json:"tool"`
	Priority  int          `json:"priority"`
	IsActive  bool         `json:"isActive"`
	CreatedAt time.Time    `json:"createdAt"`
}

// ResponseFrom converts a Tool to a Response.
func ResponseFrom(t Tool) Response {
	var config any
	_ = json.Unmarshal(t.Config, &config)
	labels := t.Labels
	if labels == nil {
		labels = []string{}
	}
	return Response{
		ID:          t.ID,
		Name:        t.Name,
		Type:        t.Type,
		Config:      config,
		Description: t.Description,
		Labels:      labels,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
