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

func TestResponseFrom_RedactsSensitiveURLQueryValues(t *testing.T) {
	const apiKey = "query-api-key-secret"
	const accessToken = "query-access-token-secret"
	response := tool.ResponseFrom(tool.Tool{Config: json.RawMessage(`{
		"url":"https://api.example.com/v1?api_key=` + apiKey + `&safe=visible",
		"urlTemplate":"https://api.example.com/search?access_token=` + accessToken + `&page=1",
		"baseUrl":"https://backend.example.com?authorization=` + accessToken + `&region=br"
	}`)})

	config, ok := response.Config.(map[string]any)
	require.True(t, ok)
	for _, value := range []string{
		config["url"].(string),
		config["urlTemplate"].(string),
		config["baseUrl"].(string),
	} {
		assert.NotContains(t, value, apiKey)
		assert.NotContains(t, value, accessToken)
	}
	assert.Contains(t, config["url"], "safe=visible")
	assert.Contains(t, config["urlTemplate"], "page=1")
	assert.Contains(t, config["baseUrl"], "region=br")
}

func TestResponseFrom_RedactsURLUserInfo(t *testing.T) {
	const username = "url-user-secret"
	const password = "url-password-secret"
	response := tool.ResponseFrom(tool.Tool{Config: json.RawMessage(`{
		"url":"https://` + username + `:` + password + `@api.example.com/v1?safe=visible"
	}`)})

	config, ok := response.Config.(map[string]any)
	require.True(t, ok)
	publicURL := config["url"].(string)
	assert.NotContains(t, publicURL, username)
	assert.NotContains(t, publicURL, password)
	assert.Contains(t, publicURL, "api.example.com")
	assert.Contains(t, publicURL, "safe=visible")
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

func TestToolResponse_DoesNotContainNestedCredentialKeys(t *testing.T) {
	t_ := tool.Tool{
		Type: tool.ToolTypeHTTP,
		Config: []byte(`{
			"url":"https://api.example.com",
			"auth":{
				"apiKey":"nested-api-key",
				"password":"nested-password",
				"safe":"kept"
			},
			"steps":[
				{"name":"safe-step","secret":"nested-secret"}
			]
		}`),
	}

	resp := tool.ResponseFrom(t_)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	body := string(data)

	assert.NotContains(t, body, "apiKey")
	assert.NotContains(t, body, "password")
	assert.NotContains(t, body, "secret")
	assert.NotContains(t, body, "nested-api-key")
	assert.NotContains(t, body, "nested-password")
	assert.NotContains(t, body, "nested-secret")
	assert.Contains(t, body, "kept")
	assert.Contains(t, body, "safe-step")
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
