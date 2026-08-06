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

var sensitiveNestedKeys = map[string]bool{
	"authorization":      true,
	"proxyauthorization": true,
	"xapikey":            true,
	"xapitoken":          true,
	"xauthtoken":         true,
	"xaccesstoken":       true,
	"xsecret":            true,
}

// isSensitiveKey reports whether the setting key holds a secret that should be masked.
func isSensitiveKey(key string) bool {
	lower := normalizeSettingKey(key)
	for _, suffix := range sensitiveKeySuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return sensitiveNestedKeys[lower]
}

func normalizeSettingKey(key string) string {
	replacer := strings.NewReplacer("_", "", "-", "", ".", "")
	return replacer.Replace(strings.ToLower(key))
}

func redactNestedSecrets(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	sanitized, err := json.Marshal(redactNestedSecretValue(value))
	if err != nil {
		return raw
	}
	return sanitized
}

func redactNestedSecretValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveKey(key) {
				out[key] = "***"
				continue
			}
			out[key] = redactNestedSecretValue(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactNestedSecretValue(child)
		}
		return out
	default:
		return value
	}
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
	} else {
		resp.Value = redactNestedSecrets(s.Value)
	}
	return resp
}

// UpdateSettingRequest is the JSON body for creating or updating a setting.
type UpdateSettingRequest struct {
	Value       json.RawMessage `json:"value"`
	Description *string         `json:"description,omitempty"`
}
