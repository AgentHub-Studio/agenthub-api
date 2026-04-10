package agent_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
)

// TestSanitizeModelConfig_RemovesApiKey verifies that apiKey is stripped.
func TestSanitizeModelConfig_RemovesApiKey(t *testing.T) {
	raw := json.RawMessage(`{"provider":"openai","model":"gpt-4o","apiKey":"sk-secret"}`)
	result := agent.SanitizeModelConfig(raw)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &m))
	assert.NotContains(t, m, "apiKey")
	assert.Equal(t, "openai", m["provider"])
	assert.Equal(t, "gpt-4o", m["model"])
}

// TestSanitizeModelConfig_RemovesAllCredentialVariants covers all four sensitive key names.
func TestSanitizeModelConfig_RemovesAllCredentialVariants(t *testing.T) {
	raw := json.RawMessage(`{
		"provider":"openai",
		"apiKey":"sk-a",
		"api_key":"sk-b",
		"apiSecret":"secret-c",
		"api_secret":"secret-d"
	}`)
	result := agent.SanitizeModelConfig(raw)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &m))
	assert.NotContains(t, m, "apiKey")
	assert.NotContains(t, m, "api_key")
	assert.NotContains(t, m, "apiSecret")
	assert.NotContains(t, m, "api_secret")
	assert.Equal(t, "openai", m["provider"])
}

// TestSanitizeModelConfig_PreservesNonSensitiveFields checks that safe fields survive.
func TestSanitizeModelConfig_PreservesNonSensitiveFields(t *testing.T) {
	raw := json.RawMessage(`{"provider":"openrouter","model":"openai/gpt-oss-20b","maxTokens":4096}`)
	result := agent.SanitizeModelConfig(raw)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &m))
	assert.Equal(t, "openrouter", m["provider"])
	assert.Equal(t, "openai/gpt-oss-20b", m["model"])
	assert.Equal(t, float64(4096), m["maxTokens"])
}

// TestSanitizeModelConfig_NilInput returns nil unchanged.
func TestSanitizeModelConfig_NilInput(t *testing.T) {
	result := agent.SanitizeModelConfig(nil)
	assert.Nil(t, result)
}

// TestSanitizeModelConfig_EmptyInput returns empty unchanged.
func TestSanitizeModelConfig_EmptyInput(t *testing.T) {
	result := agent.SanitizeModelConfig(json.RawMessage{})
	assert.Empty(t, result)
}

// TestSanitizeModelConfig_Idempotent verifies double-sanitization is safe.
func TestSanitizeModelConfig_Idempotent(t *testing.T) {
	raw := json.RawMessage(`{"provider":"anthropic","model":"claude-3","apiKey":"sk-x"}`)
	once := agent.SanitizeModelConfig(raw)
	twice := agent.SanitizeModelConfig(once)
	assert.JSONEq(t, string(once), string(twice))
}

// TestAgentManageResponse_OmitsSystemPrompt checks that system prompt is not serialised.
func TestAgentManageResponse_OmitsSystemPrompt(t *testing.T) {
	sp := "secret instructions"
	ag := agent.Agent{SystemPrompt: &sp}
	resp := agent.ManageResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "systemPrompt")
	assert.NotContains(t, string(data), "secret instructions")
}

// TestAgentManageResponse_OmitsPermissionRules checks that permissionRules is absent.
func TestAgentManageResponse_OmitsPermissionRules(t *testing.T) {
	ag := agent.Agent{PermissionRules: json.RawMessage(`{"allow":["*"]}`)}
	resp := agent.ManageResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "permissionRules")
}

// TestAgentManageResponse_ModelConfigSanitized ensures credentials are stripped in manage DTO.
func TestAgentManageResponse_ModelConfigSanitized(t *testing.T) {
	ag := agent.Agent{
		ModelConfig: json.RawMessage(`{"provider":"openai","apiKey":"sk-secret"}`),
	}
	resp := agent.ManageResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "apiKey")
	assert.NotContains(t, string(data), "sk-secret")
	assert.Contains(t, string(data), "openai")
}

// TestResponseFrom_DoesNotContainApiKey verifies the standard API DTO also sanitizes.
func TestResponseFrom_DoesNotContainApiKey(t *testing.T) {
	ag := agent.Agent{
		ModelConfig: json.RawMessage(`{"provider":"openai","model":"gpt-4o","apiKey":"sk-leak"}`),
	}
	resp := agent.ResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "apiKey")
	assert.NotContains(t, string(data), "sk-leak")
}
