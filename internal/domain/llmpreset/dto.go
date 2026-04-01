package llmpreset

import (
	"time"

	"github.com/google/uuid"
)

// LLMPresetResponse is the JSON response envelope for an LLM preset.
type LLMPresetResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	BaseURL     string    `json:"baseUrl"`
	APIKeyEnv   string    `json:"apiKeyEnv"`
	MaxTokens   int       `json:"maxTokens"`
	Temperature float64   `json:"temperature"`
	IsDefault   bool      `json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ResponseFrom converts an LLMPreset entity to LLMPresetResponse.
func ResponseFrom(p LLMPreset) LLMPresetResponse {
	return LLMPresetResponse{
		ID:          p.ID,
		Name:        p.Name,
		Provider:    p.Provider,
		Model:       p.Model,
		BaseURL:     p.BaseURL,
		APIKeyEnv:   p.APIKeyEnv,
		MaxTokens:   p.MaxTokens,
		Temperature: p.Temperature,
		IsDefault:   p.IsDefault,
		CreatedAt:   p.CreatedAt,
	}
}

// CreateLLMPresetRequest is the JSON body for preset creation.
type CreateLLMPresetRequest struct {
	Name        string  `json:"name"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	BaseURL     string  `json:"baseUrl"`
	APIKeyEnv   string  `json:"apiKeyEnv"`
	MaxTokens   int     `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
	IsDefault   bool    `json:"isDefault"`
}

// UpdateLLMPresetRequest is the JSON body for partial preset updates.
type UpdateLLMPresetRequest struct {
	Name        *string  `json:"name,omitempty"`
	Provider    *string  `json:"provider,omitempty"`
	Model       *string  `json:"model,omitempty"`
	BaseURL     *string  `json:"baseUrl,omitempty"`
	APIKeyEnv   *string  `json:"apiKeyEnv,omitempty"`
	MaxTokens   *int     `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
}
