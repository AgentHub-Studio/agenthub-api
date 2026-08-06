package agenttemplate_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agenttemplate"
)

func TestResponseFrom_RedactsSensitiveDefinitionModelConfig(t *testing.T) {
	const apiKeySecret = "template-api-key-sentinel"
	const authorizationSecret = "Bearer template-authorization-sentinel"
	const refreshTokenSecret = "template-refresh-token-sentinel"

	definition, err := json.Marshal(map[string]any{
		"systemPrompt": "You are a safe assistant.",
		"modelConfig": map[string]any{
			"model":         "gpt-5",
			"apiKey":        apiKeySecret,
			"Authorization": authorizationSecret,
			"nested": map[string]any{
				"refresh_token": refreshTokenSecret,
				"safe":          "preserve-this",
			},
		},
		"skills": []string{"knowledge"},
	})
	require.NoError(t, err)
	template := agenttemplate.AgentTemplate{ID: uuid.New(), DefinitionJSON: definition}

	response := agenttemplate.ResponseFrom(template)
	public := string(response.DefinitionJSON)

	assert.NotContains(t, public, apiKeySecret)
	assert.NotContains(t, public, authorizationSecret)
	assert.NotContains(t, public, refreshTokenSecret)
	assert.NotContains(t, public, "apiKey")
	assert.NotContains(t, public, "Authorization")
	assert.NotContains(t, public, "refresh_token")
	assert.Contains(t, public, "You are a safe assistant.")
	assert.Contains(t, public, "preserve-this")
	assert.Equal(t, string(definition), string(template.DefinitionJSON), "the public DTO must not mutate persisted definition")
}

func FuzzResponseFromRedactsSensitiveDefinitionModelConfig(f *testing.F) {
	f.Add("template-secret", "safe-template-option")
	f.Add("credential-seed", "safe-model-config")

	f.Fuzz(func(t *testing.T, secretSeed, safeSeed string) {
		secretSum := sha256.Sum256([]byte(secretSeed))
		safeSum := sha256.Sum256([]byte(safeSeed))
		secret := "template-secret-" + hex.EncodeToString(secretSum[:])
		safeValue := "safe-value-" + hex.EncodeToString(safeSum[:])
		definition, err := json.Marshal(map[string]any{
			"systemPrompt": "safe prompt",
			"modelConfig": map[string]any{
				"api_key": secret,
				"headers": map[string]string{
					"Authorization": "Bearer " + secret,
				},
				"nested": map[string]any{
					"refreshToken": secret,
					"safe":         safeValue,
				},
			},
		})
		require.NoError(t, err)
		template := agenttemplate.AgentTemplate{DefinitionJSON: definition}

		response := agenttemplate.ResponseFrom(template)
		public := string(response.DefinitionJSON)
		if !json.Valid(response.DefinitionJSON) {
			t.Fatalf("redacted definition is not valid JSON: %q", public)
		}
		if strings.Contains(public, secret) {
			t.Fatalf("public definition leaked secret: %q", public)
		}
		for _, sensitiveKey := range []string{"api_key", "Authorization", "refreshToken"} {
			if strings.Contains(public, sensitiveKey) {
				t.Fatalf("public definition retained sensitive key %q: %q", sensitiveKey, public)
			}
		}
		if !strings.Contains(public, safeValue) {
			t.Fatalf("public definition removed safe value: %q", public)
		}
		if string(template.DefinitionJSON) != string(definition) {
			t.Fatalf("ResponseFrom mutated persisted definition")
		}
	})
}
