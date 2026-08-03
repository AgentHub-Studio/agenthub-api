package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- HookCommandType constants ---

func TestHookCommandType_Constants(t *testing.T) {
	assert.Equal(t, agentic.HookCommandType("command"), agentic.HookCommandBash)
	assert.Equal(t, agentic.HookCommandType("prompt"), agentic.HookCommandPrompt)
	assert.Equal(t, agentic.HookCommandType("agent"), agentic.HookCommandAgent)
	assert.Equal(t, agentic.HookCommandType("http"), agentic.HookCommandHTTP)
}

// --- DefaultTimeout ---

func TestDefaultTimeout_ByType(t *testing.T) {
	assert.Equal(t, 30, agentic.DefaultTimeout(agentic.HookCommandBash))
	assert.Equal(t, 30, agentic.DefaultTimeout(agentic.HookCommandPrompt))
	assert.Equal(t, 60, agentic.DefaultTimeout(agentic.HookCommandAgent))
	assert.Equal(t, 10, agentic.DefaultTimeout(agentic.HookCommandHTTP))
	assert.Equal(t, 30, agentic.DefaultTimeout("unknown"))
}

// --- EffectiveTimeout ---

func TestHookCommand_EffectiveTimeout_Default(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandAgent}
	assert.Equal(t, 60*time.Second, cmd.EffectiveTimeout())
}

func TestHookCommand_EffectiveTimeout_Override(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandBash, Timeout: 5}
	assert.Equal(t, 5*time.Second, cmd.EffectiveTimeout())
}

// --- Validate ---

func TestHookCommand_Validate_BashValid(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandBash, Command: "echo hello"}
	assert.NoError(t, cmd.Validate())
}

func TestHookCommand_Validate_BashMissingCommand(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandBash}
	assert.ErrorContains(t, cmd.Validate(), "non-empty 'command'")
}

func TestHookCommand_Validate_BashBadShell(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandBash, Command: "ls", Shell: "zsh"}
	assert.ErrorContains(t, cmd.Validate(), "bash")
}

func TestHookCommand_Validate_BashValidShells(t *testing.T) {
	for _, shell := range []string{"bash", "powershell"} {
		cmd := agentic.HookCommand{Type: agentic.HookCommandBash, Command: "ls", Shell: shell}
		assert.NoError(t, cmd.Validate(), "shell=%s", shell)
	}
}

func TestHookCommand_Validate_PromptValid(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandPrompt, Prompt: "Check this"}
	assert.NoError(t, cmd.Validate())
}

func TestHookCommand_Validate_PromptMissing(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandPrompt}
	assert.ErrorContains(t, cmd.Validate(), "non-empty 'prompt'")
}

func TestHookCommand_Validate_AgentValid(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandAgent, Prompt: "Verify output"}
	assert.NoError(t, cmd.Validate())
}

func TestHookCommand_Validate_AgentMissing(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandAgent}
	assert.ErrorContains(t, cmd.Validate(), "non-empty 'prompt'")
}

func TestHookCommand_Validate_HTTPValid(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandHTTP, URL: "https://example.com/hook"}
	assert.NoError(t, cmd.Validate())
}

func TestHookCommand_Validate_HTTPMissing(t *testing.T) {
	cmd := agentic.HookCommand{Type: agentic.HookCommandHTTP}
	assert.ErrorContains(t, cmd.Validate(), "non-empty 'url'")
}

func TestHookCommand_Validate_UnknownType(t *testing.T) {
	cmd := agentic.HookCommand{Type: "magic"}
	assert.ErrorContains(t, cmd.Validate(), "unknown hook command type")
}

// --- HookMatcher ---

func TestHookMatcher_Fields(t *testing.T) {
	m := agentic.HookMatcher{
		Matcher: "Write",
		Hooks: []agentic.HookCommand{
			{Type: agentic.HookCommandBash, Command: "echo pre-write"},
		},
	}
	assert.Equal(t, "Write", m.Matcher)
	assert.Len(t, m.Hooks, 1)
}

func TestHookMatcher_EmptyMatcher(t *testing.T) {
	m := agentic.HookMatcher{
		Hooks: []agentic.HookCommand{
			{Type: agentic.HookCommandPrompt, Prompt: "check"},
		},
	}
	assert.Empty(t, m.Matcher)
	assert.Len(t, m.Hooks, 1)
}

// --- ExtendedHookEvent ---

func TestExtendedHookEvent_AllDefined(t *testing.T) {
	assert.GreaterOrEqual(t, len(agentic.AllExtendedHookEvents), 20)
}

func TestIsValidHookEvent_Valid(t *testing.T) {
	assert.True(t, agentic.IsValidHookEvent("PreToolUse"))
	assert.True(t, agentic.IsValidHookEvent("SessionStart"))
	assert.True(t, agentic.IsValidHookEvent("Stop"))
	assert.True(t, agentic.IsValidHookEvent("TaskCreated"))
}

func TestIsValidHookEvent_Invalid(t *testing.T) {
	assert.False(t, agentic.IsValidHookEvent("nonexistent"))
	assert.False(t, agentic.IsValidHookEvent(""))
	assert.False(t, agentic.IsValidHookEvent("pretooluse")) // case-sensitive
}

// --- HooksSettings ---

func TestHooksSettings_MatchersFor_Nil(t *testing.T) {
	var hs agentic.HooksSettings
	assert.Nil(t, hs.MatchersFor(agentic.HookPreToolUseExt))
}

func TestHooksSettings_MatchersFor_Existing(t *testing.T) {
	hs := agentic.HooksSettings{
		agentic.HookPreToolUseExt: {
			{Matcher: "Write", Hooks: []agentic.HookCommand{{Type: agentic.HookCommandBash, Command: "check"}}},
		},
	}
	matchers := hs.MatchersFor(agentic.HookPreToolUseExt)
	assert.Len(t, matchers, 1)
	assert.Equal(t, "Write", matchers[0].Matcher)
}

func TestHooksSettings_MatchersFor_Missing(t *testing.T) {
	hs := agentic.HooksSettings{}
	assert.Nil(t, hs.MatchersFor(agentic.HookSessionStartExt))
}

// --- HookExecutionEvent ---

func TestHookExecutionEvent_Types(t *testing.T) {
	assert.Equal(t, agentic.HookExecutionEventType("started"), agentic.HookExecStarted)
	assert.Equal(t, agentic.HookExecutionEventType("progress"), agentic.HookExecProgress)
	assert.Equal(t, agentic.HookExecutionEventType("response"), agentic.HookExecResponse)
}

func TestHookExecutionEvent_Fields(t *testing.T) {
	exitCode := 0
	evt := agentic.HookExecutionEvent{
		Type:      agentic.HookExecResponse,
		HookID:    "h1",
		HookName:  "pre-check",
		HookEvent: "PreToolUse",
		Output:    "ok",
		ExitCode:  &exitCode,
		Outcome:   "success",
	}
	assert.Equal(t, "h1", evt.HookID)
	assert.Equal(t, "success", evt.Outcome)
	assert.NotNil(t, evt.ExitCode)
}

// --- HookJSONOutput ---

func TestHookJSONOutput_ShouldContinue_Default(t *testing.T) {
	out := agentic.HookJSONOutput{}
	assert.True(t, out.ShouldContinue())
}

func TestHookJSONOutput_ShouldContinue_True(t *testing.T) {
	b := true
	out := agentic.HookJSONOutput{Continue: &b}
	assert.True(t, out.ShouldContinue())
}

func TestHookJSONOutput_ShouldContinue_False(t *testing.T) {
	b := false
	out := agentic.HookJSONOutput{Continue: &b}
	assert.False(t, out.ShouldContinue())
}

func TestHookJSONOutput_IsAsync(t *testing.T) {
	assert.False(t, (&agentic.HookJSONOutput{}).IsAsync())
	assert.True(t, (&agentic.HookJSONOutput{Async: true}).IsAsync())
}

// --- InterpolateHeaders ---

func TestInterpolateHeaders_Empty(t *testing.T) {
	result := agentic.InterpolateHeaders(nil, nil)
	assert.Nil(t, result)
}

func TestInterpolateHeaders_NoVars(t *testing.T) {
	headers := map[string]string{"Accept": "application/json"}
	result := agentic.InterpolateHeaders(headers, nil)
	assert.Equal(t, "application/json", result["Accept"])
}

func TestInterpolateHeaders_WithAllowedVar(t *testing.T) {
	t.Setenv("TEST_TOKEN_HOOKTYPE", "secret123")

	headers := map[string]string{"Authorization": "Bearer $TEST_TOKEN_HOOKTYPE"}
	result := agentic.InterpolateHeaders(headers, []string{"TEST_TOKEN_HOOKTYPE"})
	assert.Equal(t, "Bearer secret123", result["Authorization"])
}

func TestInterpolateHeaders_DisallowedVar(t *testing.T) {
	t.Setenv("SECRET_KEY", "should_not_appear")

	headers := map[string]string{"X-Key": "$SECRET_KEY"}
	result := agentic.InterpolateHeaders(headers, []string{"OTHER_VAR"})
	assert.Equal(t, "$SECRET_KEY", result["X-Key"], "disallowed var should not be interpolated")
}

func TestInterpolateHeaders_MultipleVars(t *testing.T) {
	t.Setenv("HOOK_HOST", "example.com")
	t.Setenv("HOOK_PORT", "8080")

	headers := map[string]string{"X-Target": "$HOOK_HOST:$HOOK_PORT"}
	result := agentic.InterpolateHeaders(headers, []string{"HOOK_HOST", "HOOK_PORT"})
	assert.Equal(t, "example.com:8080", result["X-Target"])
}

func TestInterpolateHeaders_MixedAllowed(t *testing.T) {
	t.Setenv("ALLOWED_VAR", "yes")

	headers := map[string]string{"X-Mix": "$ALLOWED_VAR and $DENIED_VAR"}
	result := agentic.InterpolateHeaders(headers, []string{"ALLOWED_VAR"})
	assert.Equal(t, "yes and $DENIED_VAR", result["X-Mix"])
}

// --- HookCommand optional fields ---

func TestHookCommand_OptionalFields(t *testing.T) {
	cmd := agentic.HookCommand{
		Type:          agentic.HookCommandBash,
		Command:       "echo test",
		If:            "tool_name == 'Write'",
		StatusMessage: "Checking...",
		Once:          true,
		Async:         true,
		AsyncRewake:   true,
	}
	require.NoError(t, cmd.Validate())
	assert.Equal(t, "tool_name == 'Write'", cmd.If)
	assert.True(t, cmd.Once)
	assert.True(t, cmd.Async)
	assert.True(t, cmd.AsyncRewake)
}

func TestHookCommand_PromptWithModel(t *testing.T) {
	cmd := agentic.HookCommand{
		Type:   agentic.HookCommandPrompt,
		Prompt: "Verify $ARGUMENTS",
		Model:  "claude-sonnet-4-6",
	}
	require.NoError(t, cmd.Validate())
	assert.Equal(t, "claude-sonnet-4-6", cmd.Model)
}

func TestHookCommand_HTTPWithHeaders(t *testing.T) {
	cmd := agentic.HookCommand{
		Type:           agentic.HookCommandHTTP,
		URL:            "https://example.com/hook",
		Headers:        map[string]string{"X-Custom": "$API_KEY"},
		AllowedEnvVars: []string{"API_KEY"},
	}
	require.NoError(t, cmd.Validate())
	assert.Len(t, cmd.AllowedEnvVars, 1)
}
