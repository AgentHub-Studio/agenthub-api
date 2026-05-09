package tool

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the payload for creating a tool.
// SkillID is optional — when provided the tool is automatically bound to that
// skill after creation (active, priority 0), saving callers a second API call.
type CreateRequest struct {
	Name        string          `json:"name"`
	// Slug is optional. When omitted, it is auto-derived from Name (see ToSlug).
	// Must be unique per tenant; conflicts return 422 DuplicateName.
	Slug        string          `json:"slug,omitempty"`
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
	Slug        *string         `json:"slug,omitempty"`
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
	Slug        string          `json:"slug"`
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
	"clientSecret", "client_secret", "bearerToken", "bearer_token",
}

// sensitiveHeaderKeys lists HTTP header names whose values must be masked in
// tool config responses. Bug 156: tool config.headers exposed Authorization,
// X-API-Key, etc — credentials any read-access user could harvest.
var sensitiveHeaderKeys = map[string]bool{
	"authorization":   true,
	"proxy-authorization": true,
	"x-api-key":       true,
	"x-api-token":     true,
	"x-auth-token":    true,
	"x-access-token":  true,
	"x-secret":        true,
	"cookie":          true,
	"set-cookie":      true,
}

// SanitizeToolConfig removes credential keys from a raw tool config JSON blob
// and masks sensitive HTTP header values inside config.headers.
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
	// Bug 156: mask sensitive headers (Authorization, X-API-Key, etc).
	if hdrs, ok := m["headers"].(map[string]interface{}); ok {
		for k, v := range hdrs {
			if sensitiveHeaderKeys[strings.ToLower(k)] {
				if s, ok := v.(string); ok && s != "" {
					hdrs[k] = "***"
				}
			}
		}
		m["headers"] = hdrs
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
		Slug:        t.Slug,
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
