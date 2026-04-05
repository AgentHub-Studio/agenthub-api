package integration

import (
	"encoding/json"
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

// HTTPCreateRequest is the simplified payload for HTTP/API integrations.
type HTTPCreateRequest struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Method          string          `json:"method"`
	URL             string          `json:"url"`
	Headers         json.RawMessage `json:"headers"`
	BodyTemplate    string          `json:"bodyTemplate"`
	CredentialID    *string         `json:"credentialId"`
	ResponseMapping json.RawMessage `json:"responseMapping"`
	InputSchema     json.RawMessage `json:"inputSchema"`
	ReadOnly        bool            `json:"readOnly"`
}

// HTTPResponse is the editable representation used by the simplified HTTP form.
type HTTPResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Method          string    `json:"method"`
	URL             string    `json:"url"`
	Headers         any       `json:"headers"`
	BodyTemplate    string    `json:"bodyTemplate"`
	CredentialID    *string   `json:"credentialId,omitempty"`
	ResponseMapping any       `json:"responseMapping"`
	InputSchema     any       `json:"inputSchema"`
	ReadOnly        bool      `json:"readOnly"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	LegacyPath      string    `json:"legacyPath"`
}
