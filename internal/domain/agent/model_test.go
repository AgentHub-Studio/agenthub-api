package agent_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
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

func TestResponseFrom_RedactsSensitiveConfig(t *testing.T) {
	const apiKeySecret = "agent-config-api-key-sentinel"
	const authorizationSecret = "Bearer agent-config-authorization-sentinel"
	config := json.RawMessage(`{
		"maxIterations": 12,
		"apiKey": "agent-config-api-key-sentinel",
		"headers": {"Authorization": "Bearer agent-config-authorization-sentinel"},
		"nested": {"refresh_token": "agent-config-refresh-token-sentinel", "safe": "preserve-this"}
	}`)
	ag := agent.Agent{Config: config}

	resp := agent.ResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	body := string(data)

	assert.NotContains(t, body, apiKeySecret)
	assert.NotContains(t, body, authorizationSecret)
	assert.NotContains(t, body, "agent-config-refresh-token-sentinel")
	assert.NotContains(t, body, "apiKey")
	assert.NotContains(t, body, "Authorization")
	assert.NotContains(t, body, "refresh_token")
	assert.Contains(t, body, "preserve-this")
	assert.Contains(t, body, "maxIterations")
	assert.Equal(t, string(config), string(ag.Config), "the public DTO must not mutate persisted config")
}

func FuzzResponseFromRedactsSensitiveConfig(f *testing.F) {
	f.Add("agent-config-secret", "safe-agent-config-option")
	f.Add("credential-seed", "max-iterations-option")

	f.Fuzz(func(t *testing.T, secretSeed, safeSeed string) {
		secretSum := sha256.Sum256([]byte(secretSeed))
		safeSum := sha256.Sum256([]byte(safeSeed))
		secret := "agent-config-secret-" + hex.EncodeToString(secretSum[:])
		safeValue := "safe-config-value-" + hex.EncodeToString(safeSum[:])
		config, err := json.Marshal(map[string]any{
			"maxIterations": 12,
			"api_key":       secret,
			"headers": map[string]string{
				"Authorization": "Bearer " + secret,
			},
			"nested": map[string]any{
				"refreshToken": secret,
				"safe":         safeValue,
			},
		})
		require.NoError(t, err)
		ag := agent.Agent{Config: config}

		response := agent.ResponseFrom(ag)
		public := string(response.Config)
		if !json.Valid(response.Config) {
			t.Fatalf("redacted config is not valid JSON: %q", public)
		}
		if strings.Contains(public, secret) {
			t.Fatalf("public config leaked secret: %q", public)
		}
		for _, sensitiveKey := range []string{"api_key", "Authorization", "refreshToken"} {
			if strings.Contains(public, sensitiveKey) {
				t.Fatalf("public config retained sensitive key %q: %q", sensitiveKey, public)
			}
		}
		if !strings.Contains(public, safeValue) {
			t.Fatalf("public config removed safe value: %q", public)
		}
		if string(ag.Config) != string(config) {
			t.Fatalf("ResponseFrom mutated persisted config")
		}
	})
}

// TestAgentResponse_DoesNotContainApiKey matches the test name expected by TASK-05 spec.
func TestAgentResponse_DoesNotContainApiKey(t *testing.T) {
	ag := agent.Agent{
		ModelConfig: json.RawMessage(`{"provider":"openai","model":"gpt-4o","apiKey":"sk-secret123"}`),
	}
	resp := agent.ResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sk-secret123")
	assert.NotContains(t, string(data), "apiKey")
}

func TestAgentResponse_DoesNotContainNestedModelCredentialKeys(t *testing.T) {
	ag := agent.Agent{
		ModelConfig: json.RawMessage(`{
			"provider":"openai",
			"model":"gpt-4o",
			"credentials":{
				"apiKey":"nested-model-key",
				"api_secret":"nested-legacy-secret",
				"safe":"kept"
			},
			"fallbacks":[
				{"model":"backup","clientSecret":"nested-client-secret"}
			],
			"voice":{
				"enabled":true,
				"ttsVoice":"nova"
			}
		}`),
	}

	resp := agent.ResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	body := string(data)

	assert.NotContains(t, body, "apiKey")
	assert.NotContains(t, body, "api_secret")
	assert.NotContains(t, body, "clientSecret")
	assert.NotContains(t, body, "nested-model-key")
	assert.NotContains(t, body, "nested-legacy-secret")
	assert.NotContains(t, body, "nested-client-secret")
	assert.Contains(t, body, "kept")
	assert.Contains(t, body, "nova")
}

func TestAgentVersionResponse_DoesNotContainConfigAPIKey(t *testing.T) {
	version := agent.AgentVersion{
		ConfigJSON: json.RawMessage(`{
			"provider":"openrouter",
			"model":"openai/gpt-oss-120b",
			"apiKey":"sk-version-secret",
			"api_secret":"legacy-version-secret"
		}`),
	}

	resp := agent.VersionResponseFrom(version)
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "apiKey")
	assert.NotContains(t, string(data), "api_secret")
	assert.NotContains(t, string(data), "sk-version-secret")
	assert.NotContains(t, string(data), "legacy-version-secret")
	assert.Contains(t, string(data), "openai/gpt-oss-120b")
}

// TestAgentResponse_AlwaysContainsSystemPromptField verifies systemPrompt key is present even when nil.
// P-C164-4: frontend needs the key to detect whether field is unset vs empty.
func TestAgentResponse_AlwaysContainsSystemPromptField(t *testing.T) {
	ag := agent.Agent{SystemPrompt: nil}
	resp := agent.ResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"systemPrompt"`)
	assert.Contains(t, string(data), `"systemPrompt":null`)
}

// TestAgentResponse_WithSystemPrompt_IncludedInResponse verifies systemPrompt value is serialised.
func TestAgentResponse_WithSystemPrompt_IncludedInResponse(t *testing.T) {
	prompt := "You are a helpful assistant."
	ag := agent.Agent{SystemPrompt: &prompt}
	resp := agent.ResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(data), "You are a helpful assistant.")
}

// TestAgentManageResponse_StillOmitsSystemPrompt verifies the redacted LLM DTO never leaks systemPrompt.
// The AgentManageResponse is used inside agenthub_manage tool results — the LLM must not see systemPrompt.
func TestAgentManageResponse_StillOmitsSystemPrompt(t *testing.T) {
	prompt := "secret system instructions"
	ag := agent.Agent{SystemPrompt: &prompt}
	resp := agent.ManageResponseFrom(ag)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "systemPrompt")
	assert.NotContains(t, string(data), "secret system instructions")
}

// TestAgentResponse_ModelConfigFieldsPreserved verifies non-sensitive fields survive sanitization.
func TestAgentResponse_ModelConfigFieldsPreserved(t *testing.T) {
	ag := agent.Agent{
		ModelConfig: json.RawMessage(`{"provider":"openrouter","model":"openai/gpt-oss-20b","maxTokens":4096,"apiKey":"sk-x"}`),
	}
	resp := agent.ResponseFrom(ag)
	var mc map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.ModelConfig, &mc))
	assert.Equal(t, "openrouter", mc["provider"])
	assert.Equal(t, float64(4096), mc["maxTokens"])
	assert.NotContains(t, mc, "apiKey")
}
