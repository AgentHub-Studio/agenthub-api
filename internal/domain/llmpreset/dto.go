package llmpreset

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LLMPresetResponse is the JSON response envelope for an LLM preset.
type LLMPresetResponse struct {
	ID            uuid.UUID       `json:"id"`
	TenantID      string          `json:"tenantId"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	Provider      string          `json:"provider"`
	Model         string          `json:"model"`
	MaxTokens     int             `json:"maxTokens"`
	ContextWindow int             `json:"contextWindow"`
	Temperature   float64         `json:"temperature"`
	ConfigJSON    json.RawMessage `json:"configJson,omitempty"`
	IsDefault     bool            `json:"isDefault"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

// ResponseFrom converts an LLMPreset entity to LLMPresetResponse.
func ResponseFrom(p LLMPreset) LLMPresetResponse {
	response := LLMPresetResponse(p)
	response.ConfigJSON = redactPublicConfigJSON(p.ConfigJSON)
	return response
}

// redactPublicConfigJSON keeps model tuning values visible while omitting
// credentials from the public preset DTO. The stored JSONB remains unchanged
// for execution paths that need the original configuration.
func redactPublicConfigJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(`{}`)
	}
	redacted, err := json.Marshal(redactPublicConfigValue(value))
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return redacted
}

func redactPublicConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if isSensitiveConfigKey(key) {
				continue
			}
			result[key] = redactPublicConfigValue(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactPublicConfigValue(child)
		}
		return result
	default:
		return value
	}
}

func isSensitiveConfigKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(key))
	if normalized == "authorization" || normalized == "proxyauthorization" ||
		normalized == "xapikey" || normalized == "xapitoken" ||
		normalized == "xauthtoken" || normalized == "xaccesstoken" ||
		normalized == "xsecret" {
		return true
	}
	for _, suffix := range []string{"apikey", "secret", "password", "token", "credential"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

// CreateLLMPresetRequest is the JSON body for preset creation.
type CreateLLMPresetRequest struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Provider      string          `json:"provider"`
	Model         string          `json:"model"`
	MaxTokens     int             `json:"maxTokens"`
	ContextWindow int             `json:"contextWindow"`
	Temperature   float64         `json:"temperature"`
	ConfigJSON    json.RawMessage `json:"configJson"`
	IsDefault     bool            `json:"isDefault"`
}

// UpdateLLMPresetRequest is the JSON body for partial preset updates.
type UpdateLLMPresetRequest struct {
	Name          *string         `json:"name,omitempty"`
	Description   *string         `json:"description,omitempty"`
	Provider      *string         `json:"provider,omitempty"`
	Model         *string         `json:"model,omitempty"`
	MaxTokens     *int            `json:"maxTokens,omitempty"`
	ContextWindow *int            `json:"contextWindow,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	ConfigJSON    json.RawMessage `json:"configJson,omitempty"`
}
