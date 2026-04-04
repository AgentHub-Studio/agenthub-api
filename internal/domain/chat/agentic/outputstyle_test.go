package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- OutputStyleConfig ---

func TestOutputStyleConfig_Fields(t *testing.T) {
	style := agentic.OutputStyleConfig{
		Name:                   "Test",
		Description:            "A test style",
		Prompt:                 "Be helpful",
		Source:                 agentic.OutputStyleUser,
		KeepCodingInstructions: true,
	}
	assert.Equal(t, "Test", style.Name)
	assert.Equal(t, agentic.OutputStyleUser, style.Source)
}

// --- OutputStyleRegistry ---

func TestOutputStyleRegistry_BuiltInStyles(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	styles := reg.List()
	assert.GreaterOrEqual(t, len(styles), 3, "should have explanatory, concise, learning")
}

func TestOutputStyleRegistry_GetDefault(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	style := reg.Get("default")
	assert.Nil(t, style, "default style should be nil")
}

func TestOutputStyleRegistry_GetExplanatory(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	style := reg.Get("explanatory")
	require.NotNil(t, style)
	assert.Equal(t, "Explanatory", style.Name)
	assert.NotEmpty(t, style.Prompt)
	assert.True(t, style.KeepCodingInstructions)
}

func TestOutputStyleRegistry_GetConcise(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	style := reg.Get("concise")
	require.NotNil(t, style)
	assert.Equal(t, "Concise", style.Name)
}

func TestOutputStyleRegistry_GetLearning(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	style := reg.Get("learning")
	require.NotNil(t, style)
	assert.Equal(t, "Learning", style.Name)
}

func TestOutputStyleRegistry_CaseInsensitive(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	style := reg.Get("EXPLANATORY")
	require.NotNil(t, style)
	assert.Equal(t, "Explanatory", style.Name)
}

func TestOutputStyleRegistry_Register(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	reg.Register("custom", &agentic.OutputStyleConfig{
		Name:   "Custom",
		Prompt: "Be creative",
		Source: agentic.OutputStyleUser,
	})
	style := reg.Get("custom")
	require.NotNil(t, style)
	assert.Equal(t, "Custom", style.Name)
	assert.Equal(t, agentic.OutputStyleUser, style.Source)
}

func TestOutputStyleRegistry_RegisterOverwrite(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	reg.Register("explanatory", &agentic.OutputStyleConfig{
		Name:   "My Explanatory",
		Prompt: "Custom prompt",
		Source: agentic.OutputStyleUser,
	})
	style := reg.Get("explanatory")
	require.NotNil(t, style)
	assert.Equal(t, "My Explanatory", style.Name)
}

func TestOutputStyleRegistry_GetUnknown(t *testing.T) {
	reg := agentic.NewOutputStyleRegistry()
	style := reg.Get("nonexistent")
	assert.Nil(t, style)
}

// --- ApplyOutputStyle ---

func TestApplyOutputStyle_NilStyle(t *testing.T) {
	result := agentic.ApplyOutputStyle("base prompt", nil)
	assert.Equal(t, "base prompt", result)
}

func TestApplyOutputStyle_EmptyPrompt(t *testing.T) {
	style := &agentic.OutputStyleConfig{Name: "Empty", Prompt: ""}
	result := agentic.ApplyOutputStyle("base prompt", style)
	assert.Equal(t, "base prompt", result)
}

func TestApplyOutputStyle_KeepCodingInstructions(t *testing.T) {
	style := &agentic.OutputStyleConfig{
		Name:                   "Test",
		Prompt:                 "Be concise",
		KeepCodingInstructions: true,
	}
	result := agentic.ApplyOutputStyle("You are a helpful assistant.", style)
	assert.Contains(t, result, "You are a helpful assistant.")
	assert.Contains(t, result, "Be concise")
	assert.Contains(t, result, "Output Style: Test")
}

func TestApplyOutputStyle_ReplaceCodingInstructions(t *testing.T) {
	style := &agentic.OutputStyleConfig{
		Name:                   "Replacement",
		Prompt:                 "Only respond in haiku",
		KeepCodingInstructions: false,
	}
	result := agentic.ApplyOutputStyle("You are a helpful assistant.", style)
	assert.Equal(t, "Only respond in haiku", result)
	assert.NotContains(t, result, "helpful assistant")
}

// --- RecoverConversationMessages ---

func TestRecoverConversation_EmptyInput(t *testing.T) {
	result, err := agentic.RecoverConversationMessages(nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestRecoverConversation_ValidMessages(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi there"},
	}
	raw, _ := json.Marshal(msgs)
	result, err := agentic.RecoverConversationMessages(raw)
	require.NoError(t, err)

	var recovered []map[string]interface{}
	err = json.Unmarshal(result, &recovered)
	require.NoError(t, err)
	assert.Len(t, recovered, 2)
}

func TestRecoverConversation_FiltersWhitespaceOnly(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "   "},
		{"role": "user", "content": "world"},
	}
	raw, _ := json.Marshal(msgs)
	result, err := agentic.RecoverConversationMessages(raw)
	require.NoError(t, err)

	var recovered []map[string]interface{}
	err = json.Unmarshal(result, &recovered)
	require.NoError(t, err)
	assert.Len(t, recovered, 2)
}

func TestRecoverConversation_FiltersThinkingOnly(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "question"},
		{"role": "assistant", "content": "", "metadata": map[string]interface{}{"thinkingContent": "reasoning..."}},
		{"role": "assistant", "content": "real answer"},
	}
	raw, _ := json.Marshal(msgs)
	result, err := agentic.RecoverConversationMessages(raw)
	require.NoError(t, err)

	var recovered []map[string]interface{}
	err = json.Unmarshal(result, &recovered)
	require.NoError(t, err)
	assert.Len(t, recovered, 2, "thinking-only message should be filtered")
}

func TestRecoverConversation_KeepsToolResults(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "run"},
		{"role": "tool", "content": "", "toolCallId": "tc_1"},
	}
	raw, _ := json.Marshal(msgs)
	result, err := agentic.RecoverConversationMessages(raw)
	require.NoError(t, err)

	var recovered []map[string]interface{}
	err = json.Unmarshal(result, &recovered)
	require.NoError(t, err)
	assert.Len(t, recovered, 2, "tool results should be kept even with empty content")
}

func TestRecoverConversation_KeepsToolCallsWithEmptyContent(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "go"},
		{"role": "assistant", "content": "", "toolCalls": []interface{}{map[string]interface{}{"id": "tc_1"}}},
	}
	raw, _ := json.Marshal(msgs)
	result, err := agentic.RecoverConversationMessages(raw)
	require.NoError(t, err)

	var recovered []map[string]interface{}
	err = json.Unmarshal(result, &recovered)
	require.NoError(t, err)
	assert.Len(t, recovered, 2, "messages with tool calls should be kept")
}

func TestRecoverConversation_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	result, err := agentic.RecoverConversationMessages(raw)
	assert.Error(t, err)
	assert.Equal(t, raw, result, "should return original on error")
}

func TestRecoverConversation_AllFiltered(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "assistant", "content": "  "},
		{"role": "assistant", "content": ""},
	}
	raw, _ := json.Marshal(msgs)
	result, err := agentic.RecoverConversationMessages(raw)
	require.NoError(t, err)

	var recovered []interface{}
	err = json.Unmarshal(result, &recovered)
	require.NoError(t, err)
	assert.Len(t, recovered, 0)
}

// --- Source constants ---

func TestOutputStyleSource_Constants(t *testing.T) {
	assert.Equal(t, agentic.OutputStyleSource("built-in"), agentic.OutputStyleBuiltIn)
	assert.Equal(t, agentic.OutputStyleSource("user"), agentic.OutputStyleUser)
	assert.Equal(t, agentic.OutputStyleSource("plugin"), agentic.OutputStylePlugin)
	assert.Equal(t, agentic.OutputStyleSource("agent"), agentic.OutputStyleAgent)
}
