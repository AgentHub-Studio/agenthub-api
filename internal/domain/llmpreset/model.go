// Package llmpreset implements the LLM configuration preset domain.
package llmpreset

import (
	"time"

	"github.com/google/uuid"
)

// LLMPreset is the domain entity for a global LLM configuration preset.
type LLMPreset struct {
	ID          uuid.UUID
	Name        string
	Provider    string  // openai, anthropic, ollama, openrouter
	Model       string
	BaseURL     string
	APIKeyEnv   string  // name of the environment variable holding the API key
	MaxTokens   int
	Temperature float64
	IsDefault   bool
	CreatedAt   time.Time
}
