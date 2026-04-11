// Package agenttemplate provides pre-built agent configurations that tenants can
// instantiate into full agents with a single API call. Built-in templates are
// seeded by the platform; tenants may also create their own templates.
package agenttemplate

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an agent template cannot be found.
var ErrNotFound = errors.New("agent template: not found")

// ErrSlugConflict is returned when a template slug is already in use.
var ErrSlugConflict = errors.New("agent template: slug conflict")

// AgentTemplate is the domain entity for a pre-built agent configuration.
// Stored in ah_{tenantID}.agent_template — no tenant_id column.
type AgentTemplate struct {
	ID             uuid.UUID
	Name           string
	Slug           string
	Description    string
	Category       string
	IsBuiltin      bool
	DefinitionJSON json.RawMessage // {"systemPrompt","modelConfig","skills","permissionRules"}
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Definition is the parsed content of DefinitionJSON.
type Definition struct {
	SystemPrompt    string          `json:"systemPrompt"`
	ModelConfig     json.RawMessage `json:"modelConfig,omitempty"`
	Skills          []string        `json:"skills"`
	PermissionRules json.RawMessage `json:"permissionRules,omitempty"`
}

// ParseDefinition parses the template's DefinitionJSON into a Definition struct.
// Returns an empty Definition on parse error.
func (t AgentTemplate) ParseDefinition() Definition {
	var d Definition
	if len(t.DefinitionJSON) > 0 {
		_ = json.Unmarshal(t.DefinitionJSON, &d)
	}
	return d
}
