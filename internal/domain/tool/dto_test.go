package tool_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
)

// TestSanitizeToolConfig_RemovesAuthToken verifies auth_token is stripped.
func TestSanitizeToolConfig_RemovesAuthToken(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://api.example.com","auth_token":"Bearer secret"}`)
	result := tool.SanitizeToolConfig(raw)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &m))
	assert.NotContains(t, m, "auth_token")
	assert.Equal(t, "https://api.example.com", m["url"])
}

// TestSanitizeToolConfig_RemovesAllCredentialVariants covers all sensitive key names.
func TestSanitizeToolConfig_RemovesAllCredentialVariants(t *testing.T) {
	raw := json.RawMessage(`{
		"url":"https://api.example.com",
		"auth_token":"t1",
		"authToken":"t2",
		"password":"p1",
		"secret":"s1",
		"apiKey":"ak1",
		"api_key":"ak2"
	}`)
	result := tool.SanitizeToolConfig(raw)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &m))
	for _, k := range []string{"auth_token", "authToken", "password", "secret", "apiKey", "api_key"} {
		assert.NotContains(t, m, k, "key %q should be removed", k)
	}
	assert.Equal(t, "https://api.example.com", m["url"])
}

// TestSanitizeToolConfig_PreservesNonSensitiveFields ensures safe fields survive.
func TestSanitizeToolConfig_PreservesNonSensitiveFields(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://api.example.com","method":"POST","timeout":30}`)
	result := tool.SanitizeToolConfig(raw)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &m))
	assert.Equal(t, "https://api.example.com", m["url"])
	assert.Equal(t, "POST", m["method"])
	assert.Equal(t, float64(30), m["timeout"])
}

// TestSanitizeToolConfig_NilInput returns nil unchanged.
func TestSanitizeToolConfig_NilInput(t *testing.T) {
	result := tool.SanitizeToolConfig(nil)
	assert.Nil(t, result)
}

// TestSanitizeToolConfig_Idempotent verifies double-sanitization is safe.
func TestSanitizeToolConfig_Idempotent(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://api.example.com","auth_token":"secret"}`)
	once := tool.SanitizeToolConfig(raw)
	twice := tool.SanitizeToolConfig(once)
	assert.JSONEq(t, string(once), string(twice))
}

// TestResponseFrom_DoesNotContainAuthToken ensures ResponseFrom sanitizes auth_token.
func TestResponseFrom_DoesNotContainAuthToken(t *testing.T) {
	t_ := tool.Tool{
		Type:   tool.ToolTypeHTTP,
		Config: []byte(`{"url":"https://api.example.com","auth_token":"Bearer leak"}`),
	}
	resp := tool.ResponseFrom(t_)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "auth_token")
	assert.NotContains(t, string(data), "Bearer leak")
}
