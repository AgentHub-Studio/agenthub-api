package llmpreset

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LLMPresetResponse is the JSON response envelope for an LLM preset.
type LLMPresetResponse struct {
	ID          uuid.UUID       `json:"id"`
	TenantID    string          `json:"tenantId"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	MaxTokens   int             `json:"maxTokens"`
	Temperature float64         `json:"temperature"`
	ConfigJSON  json.RawMessage `json:"configJson,omitempty"`
	IsDefault   bool            `json:"isDefault"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// ResponseFrom converts an LLMPreset entity to LLMPresetResponse.
func ResponseFrom(p LLMPreset) LLMPresetResponse {
	return LLMPresetResponse(p)
}

// CreateLLMPresetRequest is the JSON body for preset creation.
type CreateLLMPresetRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	MaxTokens   int             `json:"maxTokens"`
	Temperature float64         `json:"temperature"`
	ConfigJSON  json.RawMessage `json:"configJson"`
	IsDefault   bool            `json:"isDefault"`
}

// UpdateLLMPresetRequest is the JSON body for partial preset updates.
type UpdateLLMPresetRequest struct {
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Provider    *string          `json:"provider,omitempty"`
	Model       *string          `json:"model,omitempty"`
	MaxTokens   *int             `json:"maxTokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	ConfigJSON  json.RawMessage  `json:"configJson,omitempty"`
}
