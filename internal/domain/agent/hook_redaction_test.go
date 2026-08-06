package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicHookResponseFromRedactsSensitiveConfig(t *testing.T) {
	hook := agentHook{
		ID: uuid.New(),
		Config: json.RawMessage(`{
			"url":"https://hook-user:hook-url-password@hooks.example.test/run?api_key=hook-url-api-key&region=br",
			"headers":{"Authorization":"Bearer hook-header-secret","X-Trace":"safe"},
			"nested":{"api_key":"hook-api-key-secret","safe":"kept"},
			"items":[{"refresh_token":"hook-refresh-secret"}]
		}`),
	}

	public := publicHookResponseFrom(hook)
	body, err := json.Marshal(public)
	require.NoError(t, err)
	for _, secret := range []string{"hook-url-password", "hook-url-api-key", "hook-header-secret", "hook-api-key-secret", "hook-refresh-secret", "Authorization", "refresh_token"} {
		assert.NotContains(t, string(body), secret)
	}
	var publicConfig struct {
		URL string `json:"url"`
	}
	require.NoError(t, json.Unmarshal(public.Config, &publicConfig))
	parsedURL, err := url.Parse(publicConfig.URL)
	require.NoError(t, err)
	password, hasPassword := parsedURL.User.Password()
	assert.True(t, hasPassword)
	assert.Equal(t, "***", password)
	assert.Equal(t, "***", parsedURL.Query().Get("api_key"))
	assert.Equal(t, "br", parsedURL.Query().Get("region"))
	var publicValue map[string]any
	require.NoError(t, json.Unmarshal(public.Config, &publicValue))
	delete(publicValue, "url")
	withoutURL, err := json.Marshal(publicValue)
	require.NoError(t, err)
	assert.JSONEq(t, `{"headers":{"X-Trace":"safe"},"nested":{"safe":"kept"},"items":[{}]}`, string(withoutURL))
	assert.Contains(t, string(hook.Config), "hook-header-secret")
	assert.Contains(t, string(hook.Config), "hook-api-key-secret")
	assert.Contains(t, string(hook.Config), "hook-url-password")
}

func FuzzPublicHookResponseFromRedactsSensitiveConfig(f *testing.F) {
	f.Add("header", "api-key", "refresh")

	f.Fuzz(func(t *testing.T, header, apiKey, refresh string) {
		markers := []string{hookSecretMarker(header), hookSecretMarker(apiKey), hookSecretMarker(refresh)}
		config, err := json.Marshal(map[string]any{
			"url":     "https://hook-user:" + markers[0] + "@hooks.example.test/run?api_key=" + markers[1] + "&region=br",
			"headers": map[string]string{"Authorization": "Bearer " + markers[0]},
			"nested":  map[string]string{"api_key": markers[1]},
			"items":   []map[string]string{{"refresh_token": markers[2]}},
			"safe":    "preserved",
		})
		require.NoError(t, err)

		original := agentHook{Config: config}
		public := publicHookResponseFrom(original)
		body, err := json.Marshal(public)
		require.NoError(t, err)
		for _, marker := range markers {
			assert.NotContains(t, string(body), marker)
		}
		assert.Contains(t, string(public.Config), "preserved")
		assert.Contains(t, string(public.Config), "region=br")
		assert.Equal(t, string(config), string(original.Config))
	})
}

func hookSecretMarker(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "hook-secret-" + hex.EncodeToString(sum[:])
}
