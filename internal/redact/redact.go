package redact

import (
	"encoding/json"
	"strings"
)

// IsSensitiveKey reports whether a JSON object key is expected to carry a secret.
func IsSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "" {
		return false
	}
	if sensitiveKeys[normalized] {
		return true
	}
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

var sensitiveKeys = map[string]bool{
	"apikey":             true,
	"apisecret":          true,
	"accesstoken":        true,
	"authtoken":          true,
	"authorization":      true,
	"bearertoken":        true,
	"clientsecret":       true,
	"cookie":             true,
	"credential":         true,
	"credentials":        true,
	"password":           true,
	"passwd":             true,
	"proxyauthorization": true,
	"refreshtoken":       true,
	"secret":             true,
	"setcookie":          true,
	"xaccesstoken":       true,
	"xapikey":            true,
	"xapitoken":          true,
	"xauthtoken":         true,
	"xsecret":            true,
}

var sensitiveKeyFragments = []string{
	"apikey",
	"apisecret",
	"authtoken",
	"authorization",
	"bearertoken",
	"clientsecret",
	"credential",
	"password",
	"refreshtoken",
	"secretkey",
}

func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		switch r {
		case '_', '-', '.', ' ':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// RemoveSensitiveJSONFields removes sensitive keys recursively from a JSON object.
func RemoveSensitiveJSONFields(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	cleaned := removeSensitive(value)
	out, err := json.Marshal(cleaned)
	if err != nil {
		return raw
	}
	return out
}

func removeSensitive(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if IsSensitiveKey(key) {
				continue
			}
			out[key] = removeSensitive(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = removeSensitive(item)
		}
		return out
	default:
		return value
	}
}

// RedactSensitiveJSONFields replaces sensitive values recursively in a JSON object.
func RedactSensitiveJSONFields(raw json.RawMessage, marker string) json.RawMessage {
	if len(raw) == 0 || marker == "" {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	cleaned := redactSensitive(value, marker)
	out, err := json.Marshal(cleaned)
	if err != nil {
		return raw
	}
	return out
}

func redactSensitive(value any, marker string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if IsSensitiveKey(key) {
				out[key] = marker
				continue
			}
			out[key] = redactSensitive(item, marker)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = redactSensitive(item, marker)
		}
		return out
	default:
		return value
	}
}
