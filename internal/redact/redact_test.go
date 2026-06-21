package redact

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSensitiveKey(t *testing.T) {
	assert.True(t, IsSensitiveKey("apiKey"))
	assert.True(t, IsSensitiveKey("client_secret"))
	assert.True(t, IsSensitiveKey("Authorization"))
	assert.False(t, IsSensitiveKey("maxTokens"))
	assert.False(t, IsSensitiveKey("model"))
}

func TestRemoveSensitiveJSONFields(t *testing.T) {
	raw := json.RawMessage(`{"model":"gpt-4o","apiKey":"secret","nested":{"clientSecret":"nested","temperature":0.2}}`)
	cleaned := RemoveSensitiveJSONFields(raw)
	assert.JSONEq(t, `{"model":"gpt-4o","nested":{"temperature":0.2}}`, string(cleaned))
}

func TestRedactSensitiveJSONFields(t *testing.T) {
	raw := json.RawMessage(`{"apiKey":"secret","nested":{"clientSecret":"nested"}}`)
	cleaned := RedactSensitiveJSONFields(raw, "[REDACTED]")
	assert.JSONEq(t, `{"apiKey":"[REDACTED]","nested":{"clientSecret":"[REDACTED]"}}`, string(cleaned))
}
