package llmpreset_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/llmpreset"
)

func TestResponseFrom_RedactsSensitiveConfigJSON(t *testing.T) {
	const apiKeySecret = "preset-api-key-sentinel"
	const authorizationSecret = "Bearer preset-authorization-sentinel"
	const refreshTokenSecret = "preset-refresh-token-sentinel"

	config, err := json.Marshal(map[string]any{
		"top_p":         0.9,
		"apiKey":        apiKeySecret,
		"Authorization": authorizationSecret,
		"nested": map[string]any{
			"refresh_token": refreshTokenSecret,
			"safe":          "preserve-this",
		},
	})
	require.NoError(t, err)
	preset := llmpreset.LLMPreset{ID: uuid.New(), ConfigJSON: config}

	response := llmpreset.ResponseFrom(preset)
	public := string(response.ConfigJSON)

	assert.NotContains(t, public, apiKeySecret)
	assert.NotContains(t, public, authorizationSecret)
	assert.NotContains(t, public, refreshTokenSecret)
	assert.NotContains(t, public, "apiKey")
	assert.NotContains(t, public, "Authorization")
	assert.NotContains(t, public, "refresh_token")
	assert.Contains(t, public, `"top_p":0.9`)
	assert.Contains(t, public, "preserve-this")
	assert.Equal(t, string(config), string(preset.ConfigJSON), "the public DTO must not mutate persisted config")
	assert.False(t, strings.Contains(public, "[REDACTED]"), "credential keys are omitted from the DTO")
}

func FuzzResponseFromRedactsSensitiveConfigJSON(f *testing.F) {
	f.Add("preset-secret", "safe-model-option")
	f.Add("credential-value", "preserve-top-p")

	f.Fuzz(func(t *testing.T, secretSeed, safeSeed string) {
		secretSum := sha256.Sum256([]byte(secretSeed))
		safeSum := sha256.Sum256([]byte(safeSeed))
		secret := "preset-secret-" + hex.EncodeToString(secretSum[:])
		safeValue := "safe-value-" + hex.EncodeToString(safeSum[:])
		config, err := json.Marshal(map[string]any{
			"api_key": secret,
			"headers": map[string]string{
				"Authorization": "Bearer " + secret,
			},
			"nested": map[string]any{
				"refreshToken": secret,
				"safe":         safeValue,
			},
			"top_p": 0.9,
		})
		require.NoError(t, err)
		preset := llmpreset.LLMPreset{ConfigJSON: config}

		response := llmpreset.ResponseFrom(preset)
		public := string(response.ConfigJSON)
		if !json.Valid(response.ConfigJSON) {
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
		if string(preset.ConfigJSON) != string(config) {
			t.Fatalf("ResponseFrom mutated persisted config")
		}
	})
}
