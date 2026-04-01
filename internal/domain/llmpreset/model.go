// Package llmpreset implements the LLM configuration preset domain.
package llmpreset

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Visibility constants for LLM presets.
const (
	VisibilityPrivate      = "PRIVATE"
	VisibilityOrganization = "ORGANIZATION"
	VisibilityPublic       = "PUBLIC"
)

// LLMPreset is the domain entity for a per-tenant LLM configuration preset
// stored in the public schema with an explicit tenant_id for isolation.
type LLMPreset struct {
	ID          uuid.UUID
	TenantID    string          // Keycloak realm slug, e.g. "my-company"
	Name        string
	Description string
	Provider    string          // OPENAI, ANTHROPIC, OLLAMA, OPENROUTER
	Model       string
	BaseURL     string
	APIKeyEnv   string          // env-var name holding the API key
	MaxTokens   int
	Temperature float64
	ConfigJSON  json.RawMessage // free-form JSONB: top_p, frequency_penalty, etc.
	IsDefault   bool
	IsPublic    bool
	Visibility  string          // PRIVATE, ORGANIZATION, PUBLIC
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
