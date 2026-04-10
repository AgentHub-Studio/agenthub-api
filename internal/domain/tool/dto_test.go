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

// TestToolResponse_DoesNotContainAuthToken matches the test name expected by TASK-05 spec.
func TestToolResponse_DoesNotContainAuthToken(t *testing.T) {
	t_ := tool.Tool{
		Config: []byte(`{"url":"https://api.example.com","auth_token":"Bearer secret-token"}`),
	}
	resp := tool.ResponseFrom(t_)
	data, _ := json.Marshal(resp)
	assert.NotContains(t, string(data), "secret-token")
	assert.NotContains(t, string(data), "auth_token")
}

// TestToolResponse_NonSensitiveConfigPreserved verifies non-sensitive fields survive sanitization.
func TestToolResponse_NonSensitiveConfigPreserved(t *testing.T) {
	t_ := tool.Tool{
		Config: []byte(`{"url":"https://api.example.com","method":"POST","auth_token":"x"}`),
	}
	resp := tool.ResponseFrom(t_)
	data, _ := json.Marshal(resp.Config)
	var cfg map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &cfg))
	assert.Equal(t, "https://api.example.com", cfg["url"])
	assert.NotContains(t, cfg, "auth_token")
}

// --- TR-01-TASK-10: InputSchema (P-C175-1/P-C175-2) ---

// TestResponseFrom_ContainsInputSchema verifies that InputSchema is included in the response.
func TestResponseFrom_ContainsInputSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`)
	t_ := tool.Tool{InputSchema: schema}
	resp := tool.ResponseFrom(t_)
	assert.Equal(t, string(schema), string(resp.InputSchema))
}

// TestResponseFrom_NilInputSchema_OmittedFromJSON verifies that a nil InputSchema
// is omitted from the JSON response (omitempty).
func TestResponseFrom_NilInputSchema_OmittedFromJSON(t *testing.T) {
	t_ := tool.Tool{}
	resp := tool.ResponseFrom(t_)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "inputSchema")
}

// TestCreateRequest_InputSchema_PassedThrough verifies the DTO carries InputSchema.
func TestCreateRequest_InputSchema_PassedThrough(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer"}},"required":["limit"]}`)
	raw := []byte(`{"name":"search","type":"HTTP","config":{},"inputSchema":` + string(schema) + `}`)
	var req tool.CreateRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	assert.Equal(t, string(schema), string(req.InputSchema))
}
