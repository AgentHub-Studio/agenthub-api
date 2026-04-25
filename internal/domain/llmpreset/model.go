// Package llmpreset implements the LLM configuration preset domain.
package llmpreset

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LLMPreset is the domain entity for a per-tenant LLM configuration preset
// stored in the public schema with an explicit tenant_id for isolation.
type LLMPreset struct {
	ID            uuid.UUID
	TenantID      string // Keycloak realm slug, e.g. "my-company"
	Name          string
	Description   string
	Provider      string // OPENAI, ANTHROPIC, OLLAMA, OPENROUTER
	Model         string
	MaxTokens     int
	ContextWindow int
	Temperature   float64
	ConfigJSON    json.RawMessage // free-form JSONB: top_p, frequency_penalty, etc.
	IsDefault     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
