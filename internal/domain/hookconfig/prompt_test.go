package hookconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptConfigAliases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  string
		want    string
		wantErr error
	}{
		{name: "template only", config: `{"template":"Verify {{.ToolName}}"}`, want: "Verify {{.ToolName}}"},
		{name: "inject only", config: `{"inject":"Verify before execution"}`, want: "Verify before execution"},
		{name: "empty template falls back to inject", config: `{"template":"","inject":"Verify before execution"}`, want: "Verify before execution"},
		{name: "equivalent aliases", config: `{"template":"Verify before execution","inject":"Verify before execution"}`, want: "Verify before execution"},
		{name: "conflicting aliases", config: `{"template":"Prefer this","inject":"Use this instead"}`, wantErr: ErrConflictingPromptAliases},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(tc.config)
			err := ValidatePromptAliases(raw)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				resolved, resolveErr := ResolvePromptConfig(raw)
				assert.Empty(t, resolved)
				require.ErrorIs(t, resolveErr, tc.wantErr)
				return
			}
			require.NoError(t, err)
			resolved, resolveErr := ResolvePromptConfig(raw)
			require.NoError(t, resolveErr)
			assert.Equal(t, tc.want, resolved)
		})
	}
}

func TestValidatePromptAliasesDefersMalformedConfigToRuntime(t *testing.T) {
	raw := json.RawMessage(`{"template":123,"inject":"safe"}`)
	assert.NoError(t, ValidatePromptAliases(raw))
	_, err := ResolvePromptConfig(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid prompt hook config")
}
