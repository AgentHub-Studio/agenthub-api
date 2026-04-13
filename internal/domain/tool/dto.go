package tool

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the payload for creating a tool.
// SkillID is optional — when provided the tool is automatically bound to that
// skill after creation (active, priority 0), saving callers a second API call.
type CreateRequest struct {
	Name        string          `json:"name"`
	Type        ToolType        `json:"type"`
	Config      json.RawMessage `json:"config"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"` // P-C175-1: explicit schema
	Description string          `json:"description"`
	Labels      []string        `json:"labels"`
	ReadOnly    bool            `json:"readOnly"`
	SkillID     *uuid.UUID      `json:"skillId,omitempty"` // optional: auto-bind to skill after creation
}

// UpdateRequest is the payload for updating a tool.
// All fields use pointer types so that PATCH callers can omit fields they
// do not want to change — a nil pointer means "keep existing value".
// P-C196-1: partial updates must not overwrite fields that were not sent.
type UpdateRequest struct {
	Name        *string         `json:"name,omitempty"`
	Type        *ToolType       `json:"type,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"` // P-C175-1: explicit schema
	Description *string         `json:"description,omitempty"`
	Labels      []string        `json:"labels,omitempty"`
	ReadOnly    *bool           `json:"readOnly,omitempty"`
}

// BindRequest is the payload for binding a tool to a skill.
type BindRequest struct {
	ToolID   uuid.UUID `json:"toolId"`
	Priority int       `json:"priority"`
	IsActive *bool     `json:"isActive"`
}

// Response is the JSON representation of a Tool.
type Response struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Type        ToolType        `json:"type"`
	Config      any             `json:"config"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"` // P-C175-1: exposed when set
	Description string          `json:"description"`
	Labels      []string        `json:"labels"`
	ReadOnly    bool            `json:"readOnly"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
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

// sensitiveToolConfigKeys lists config keys that must not be returned in API responses
// or tool results. P-C239-1: auth_token must not appear in any tool response body.
var sensitiveToolConfigKeys = []string{
	"auth_token", "authToken", "password", "secret", "apiKey", "api_key",
}

// SanitizeToolConfig removes credential keys from a raw tool config JSON blob.
// Returns the sanitized JSON; on parse error returns the original input unchanged.
// Safe to call on nil or empty input.
func SanitizeToolConfig(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw // not a flat object — return as-is rather than corrupt it
	}
	for _, k := range sensitiveToolConfigKeys {
		delete(m, k)
	}
	sanitized, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return sanitized
}

// ResponseFrom converts a Tool to a Response.
func ResponseFrom(t Tool) Response {
	var config any
	_ = json.Unmarshal(SanitizeToolConfig(t.Config), &config)
	labels := t.Labels
	if labels == nil {
		labels = []string{}
	}
	return Response{
		ID:          t.ID,
		Name:        t.Name,
		Type:        t.Type,
		Config:      config,
		InputSchema: t.InputSchema,
		Description: t.Description,
		Labels:      labels,
		ReadOnly:    t.ReadOnly,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
