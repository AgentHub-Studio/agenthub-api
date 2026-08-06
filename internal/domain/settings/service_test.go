package settings_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
)

type mockSettingsRepo struct {
	data map[string]settings.Setting
}

func newMockRepo() *mockSettingsRepo {
	return &mockSettingsRepo{data: make(map[string]settings.Setting)}
}

func (m *mockSettingsRepo) FindAll(_ context.Context) ([]settings.Setting, error) {
	out := make([]settings.Setting, 0, len(m.data))
	for _, s := range m.data {
		out = append(out, s)
	}
	return out, nil
}

func (m *mockSettingsRepo) FindByKey(_ context.Context, key string) (settings.Setting, error) {
	s, ok := m.data[key]
	if !ok {
		return settings.Setting{}, settings.ErrNotFound
	}
	return s, nil
}

func (m *mockSettingsRepo) Upsert(_ context.Context, s settings.Setting) (settings.Setting, error) {
	m.data[s.Key] = s
	return s, nil
}

func (m *mockSettingsRepo) Delete(_ context.Context, key string) error {
	if _, ok := m.data[key]; !ok {
		return settings.ErrNotFound
	}
	delete(m.data, key)
	return nil
}

func TestSettingsService_Upsert_Success(t *testing.T) {
	svc := settings.NewService(newMockRepo())
	val, _ := json.Marshal("dark")
	desc := "UI theme"
	s, err := svc.Upsert(context.Background(), "theme", settings.UpdateSettingRequest{
		Value:       val,
		Description: &desc,
	})
	require.NoError(t, err)
	assert.Equal(t, "theme", s.Key)
}

func TestSettingsService_Upsert_EmptyKey(t *testing.T) {
	svc := settings.NewService(newMockRepo())
	val, _ := json.Marshal("x")
	_, err := svc.Upsert(context.Background(), "", settings.UpdateSettingRequest{Value: val})
	require.Error(t, err)
}

func TestSettingsService_Get_NotFound(t *testing.T) {
	svc := settings.NewService(newMockRepo())
	_, err := svc.Get(context.Background(), "nonexistent")
	require.ErrorIs(t, err, settings.ErrNotFound)
}

func TestSettingsService_Get_DefaultsMissingSystemPrompt(t *testing.T) {
	svc := settings.NewService(newMockRepo())
	resp, err := svc.Get(context.Background(), "system.prompt")
	require.NoError(t, err)

	assert.Equal(t, "system.prompt", resp.Key)
	assert.JSONEq(t, `""`, string(resp.Value))
	assert.Contains(t, resp.Description, "Global system prompt")
}

func TestSettingsService_Delete_Success(t *testing.T) {
	svc := settings.NewService(newMockRepo())
	val, _ := json.Marshal(true)
	_, err := svc.Upsert(context.Background(), "feature-flag", settings.UpdateSettingRequest{Value: val})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), "feature-flag")
	require.NoError(t, err)
}

func TestSettingsService_List(t *testing.T) {
	svc := settings.NewService(newMockRepo())
	for _, k := range []string{"a", "b", "c"} {
		v, _ := json.Marshal(k)
		_, err := svc.Upsert(context.Background(), k, settings.UpdateSettingRequest{Value: v})
		require.NoError(t, err)
	}
	items, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

// TestResponseFrom_MasksSensitiveKeys verifies that API keys and secrets are
// masked in the SettingResponse returned from ResponseFrom.
func TestResponseFrom_MasksSensitiveKeys(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		wantMask bool
	}{
		{"openrouter.apiKey", "sk-or-v1-supersecrettoken123456", true},
		{"openai.apiKey", "sk-proj-verysecret", true},
		{"claude.apiKey", "sk-ant-api03-secret", true},
		{"smtp.password", "my-password-123", true},
		{"oauth.secret", "client-secret-xyz", true},
		{"general.language", "pt-BR", false},
		{"openrouter.temperature", "1", false},
		{"general.defaultProvider", "openrouter", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			rawVal, _ := json.Marshal(tt.value)
			s := settings.Setting{Key: tt.key, Value: rawVal}
			resp := settings.ResponseFrom(s)
			var got string
			require.NoError(t, json.Unmarshal(resp.Value, &got))
			if tt.wantMask {
				assert.Contains(t, got, "***", "expected value to be masked")
				assert.NotEqual(t, tt.value, got, "expected masked value to differ from original")
			} else {
				assert.Equal(t, tt.value, got, "non-sensitive key should not be masked")
			}
		})
	}
}

func TestResponseFrom_RedactsNestedSensitiveValueKeys(t *testing.T) {
	raw := json.RawMessage(`{
		"provider":"custom",
		"credentials":{
			"apiKey":"nested-api-key-secret",
			"clientSecret":"nested-client-secret",
			"headers":{
				"Authorization":"Bearer nested-authorization-token",
				"X-Trace":"trace-id"
			}
		},
		"fallbacks":[
			{"refreshToken":"nested-refresh-token"},
			{"safe":"kept"}
		]
	}`)

	resp := settings.ResponseFrom(settings.Setting{Key: "provider.bundle", Value: raw})
	body := string(resp.Value)

	for _, secret := range []string{
		"nested-api-key-secret",
		"nested-client-secret",
		"nested-authorization-token",
		"nested-refresh-token",
	} {
		assert.NotContains(t, body, secret)
	}
	assert.Contains(t, body, `"apiKey":"***"`)
	assert.Contains(t, body, `"clientSecret":"***"`)
	assert.Contains(t, body, `"Authorization":"***"`)
	assert.Contains(t, body, `"refreshToken":"***"`)
	assert.Contains(t, body, `"provider":"custom"`)
	assert.Contains(t, body, `"X-Trace":"trace-id"`)
	assert.Contains(t, body, `"safe":"kept"`)
	assert.True(t, json.Valid(resp.Value), "sanitized setting response must remain valid JSON")
	assert.False(t, strings.Contains(body, `nested-`), "no nested secret marker should remain")
}
