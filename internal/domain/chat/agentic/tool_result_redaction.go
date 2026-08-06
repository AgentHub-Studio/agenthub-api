package agentic

import (
	"encoding/json"
	"regexp"
	"strings"
)

var sensitiveToolResultHeaderPattern = regexp.MustCompile(`(?im)(^|:[\t ]+)[\t ]*(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|api-key|x-auth-token|password|api[_-]?key|secret|client[_-]?secret)[\t ]*[:=][^\r\n]*`)

func redactSensitiveToolResultOutput(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(redactSensitiveToolResultString(string(raw)))
	}

	redacted := redactSensitiveToolResultValue(value)
	out, err := json.Marshal(redacted)
	if err != nil {
		return json.RawMessage(redactSensitiveToolResultString(string(raw)))
	}
	return out
}

func redactSensitiveToolResultValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveToolResultKey(key) {
				continue
			}
			out[key] = redactSensitiveToolResultValue(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactSensitiveToolResultValue(child)
		}
		return out
	case string:
		return redactSensitiveToolResultString(v)
	default:
		return value
	}
}

func isSensitiveToolResultKey(key string) bool {
	normalised := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	switch normalised {
	case "authorization",
		"proxyauthorization",
		"cookie",
		"setcookie",
		"authtoken",
		"xauthtoken",
		"accesstoken",
		"refreshtoken",
		"apikey",
		"xapikey",
		"password",
		"secret",
		"clientsecret",
		"bearertoken":
		return true
	default:
		return false
	}
}

func redactSensitiveToolResultString(value string) string {
	return redactSensitiveDiagnosticString(value)
}

// redactSensitiveDiagnosticString removes credential-like values from text
// received from an external service before it reaches a log or user-facing
// diagnostic. Keep this in the agentic package so ingress, tool and runner
// paths share the same policy.
func redactSensitiveDiagnosticString(value string) string {
	value = RedactSecrets(value, "[REDACTED]")
	return sensitiveToolResultHeaderPattern.ReplaceAllString(value, "$1[REDACTED]")
}
