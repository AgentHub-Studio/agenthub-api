package settings

import (
	"encoding/json"
	"strings"
	"time"
)

// SettingResponse is the JSON response envelope for a setting.
type SettingResponse struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// sensitiveKeySuffixes lists key suffixes whose values must be masked in responses.
var sensitiveKeySuffixes = []string{"apikey", "secret", "password", "token", "credential"}

// isSensitiveKey reports whether the setting key holds a secret that should be masked.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, suffix := range sensitiveKeySuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// maskSecret replaces a JSON string value with a redacted representation that
// keeps the first 8 characters and hides the rest. Non-string values are returned
// as the literal "***" JSON string.
func maskSecret(raw json.RawMessage) json.RawMessage {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && len(s) > 8 {
		masked, _ := json.Marshal(s[:8] + "***")
		return masked
	}
	return json.RawMessage(`"***"`)
}

// ResponseFrom converts a Setting entity to SettingResponse, masking sensitive values.
func ResponseFrom(s Setting) SettingResponse {
	resp := SettingResponse(s)
	if isSensitiveKey(s.Key) {
		resp.Value = maskSecret(s.Value)
	}
	return resp
}

// UpdateSettingRequest is the JSON body for creating or updating a setting.
type UpdateSettingRequest struct {
	Value       json.RawMessage `json:"value"`
	Description *string         `json:"description,omitempty"`
}
