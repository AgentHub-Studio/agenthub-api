package agentic

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// HookCommand is the discriminated union of hook action types.
// Each variant is identified by the Type field.
//
// Inspired by Claude Code's HookCommandSchema in schemas/hooks.ts.
type HookCommand struct {
	// Type discriminates the hook variant: "command", "prompt", "agent", "http".
	Type HookCommandType `json:"type"`

	// --- Common fields (all types) ---

	// If is a permission-rule-syntax condition that must match for this hook to fire.
	If string `json:"if,omitempty"`
	// Timeout in seconds. Defaults vary by type.
	Timeout int `json:"timeout,omitempty"`
	// StatusMessage is a custom spinner message shown while the hook executes.
	StatusMessage string `json:"statusMessage,omitempty"`
	// Once means the hook is removed after a single successful execution.
	Once bool `json:"once,omitempty"`

	// --- BashCommandHook fields (type: "command") ---

	// Command is the shell command to execute.
	Command string `json:"command,omitempty"`
	// Shell selects the interpreter: "bash" (default) or "powershell".
	Shell string `json:"shell,omitempty"`
	// Async runs the command in the background without blocking the caller.
	Async bool `json:"async,omitempty"`
	// AsyncRewake runs in background and rewakes the agent on exit code 2.
	AsyncRewake bool `json:"asyncRewake,omitempty"`

	// --- PromptHook / AgentHook fields (type: "prompt" or "agent") ---

	// Prompt is the LLM prompt text. Supports $ARGUMENTS placeholder.
	Prompt string `json:"prompt,omitempty"`
	// Model specifies which LLM model to use (e.g. "claude-sonnet-4-6").
	Model string `json:"model,omitempty"`

	// --- HttpHook fields (type: "http") ---

	// URL is the endpoint to POST the hook input JSON to.
	URL string `json:"url,omitempty"`
	// Headers are custom HTTP headers. Values support $VAR_NAME env interpolation.
	Headers map[string]string `json:"headers,omitempty"`
	// AllowedEnvVars is an explicit list of env vars permitted for header interpolation.
	AllowedEnvVars []string `json:"allowedEnvVars,omitempty"`
}

// HookCommandType identifies the kind of hook action.
type HookCommandType string

const (
	HookCommandBash   HookCommandType = "command"
	HookCommandPrompt HookCommandType = "prompt"
	HookCommandAgent  HookCommandType = "agent"
	HookCommandHTTP   HookCommandType = "http"
)

// DefaultTimeout returns the default timeout (seconds) for a hook command type.
func DefaultTimeout(typ HookCommandType) int {
	switch typ {
	case HookCommandAgent:
		return 60
	case HookCommandBash:
		return 30
	case HookCommandPrompt:
		return 30
	case HookCommandHTTP:
		return 10
	default:
		return 30
	}
}

// EffectiveTimeout returns the timeout to use, falling back to the type default.
func (hc *HookCommand) EffectiveTimeout() time.Duration {
	t := hc.Timeout
	if t <= 0 {
		t = DefaultTimeout(hc.Type)
	}
	return time.Duration(t) * time.Second
}

// Validate checks that the HookCommand has valid fields for its type.
func (hc *HookCommand) Validate() error {
	switch hc.Type {
	case HookCommandBash:
		if hc.Command == "" {
			return fmt.Errorf("hook type 'command' requires a non-empty 'command' field")
		}
		if hc.Shell != "" && hc.Shell != "bash" && hc.Shell != "powershell" {
			return fmt.Errorf("hook shell must be 'bash' or 'powershell', got %q", hc.Shell)
		}
	case HookCommandPrompt:
		if hc.Prompt == "" {
			return fmt.Errorf("hook type 'prompt' requires a non-empty 'prompt' field")
		}
	case HookCommandAgent:
		if hc.Prompt == "" {
			return fmt.Errorf("hook type 'agent' requires a non-empty 'prompt' field")
		}
	case HookCommandHTTP:
		if hc.URL == "" {
			return fmt.Errorf("hook type 'http' requires a non-empty 'url' field")
		}
	default:
		return fmt.Errorf("unknown hook command type: %q", hc.Type)
	}
	return nil
}

// --- HookMatcher ---

// HookMatcher pairs an optional string pattern with one or more hook commands.
// When the matcher is empty, the hooks fire unconditionally for the event.
//
// Inspired by Claude Code's HookMatcherSchema.
type HookMatcher struct {
	// Matcher is an optional pattern (e.g. tool name "Write", glob "Read*").
	Matcher string `json:"matcher,omitempty"`
	// Hooks are the commands to execute when the matcher matches.
	Hooks []HookCommand `json:"hooks"`
}

// --- HooksSettings ---

// ExtendedHookEvent extends the base HookEvent with additional events
// from Claude Code's comprehensive hook event system.
type ExtendedHookEvent string

const (
	HookPreToolUseExt          ExtendedHookEvent = "PreToolUse"
	HookPostToolUseExt         ExtendedHookEvent = "PostToolUse"
	HookPostToolUseFailureExt  ExtendedHookEvent = "PostToolUseFailure"
	HookNotificationExt        ExtendedHookEvent = "Notification"
	HookUserPromptSubmitExt    ExtendedHookEvent = "UserPromptSubmit"
	HookSessionStartExt        ExtendedHookEvent = "SessionStart"
	HookSessionEndExt          ExtendedHookEvent = "SessionEnd"
	HookStopExt                ExtendedHookEvent = "Stop"
	HookStopFailureExt         ExtendedHookEvent = "StopFailure"
	HookSubagentStartExt       ExtendedHookEvent = "SubagentStart"
	HookSubagentStopExt        ExtendedHookEvent = "SubagentStop"
	HookPreCompactExt          ExtendedHookEvent = "PreCompact"
	HookPostCompactExt         ExtendedHookEvent = "PostCompact"
	HookPermissionRequestExt   ExtendedHookEvent = "PermissionRequest"
	HookPermissionDeniedExt    ExtendedHookEvent = "PermissionDenied"
	HookSetupExt               ExtendedHookEvent = "Setup"
	HookTaskCreatedExt         ExtendedHookEvent = "TaskCreated"
	HookTaskCompletedExt       ExtendedHookEvent = "TaskCompleted"
	HookElicitationExt         ExtendedHookEvent = "Elicitation"
	HookElicitationResultExt   ExtendedHookEvent = "ElicitationResult"
	HookConfigChangeExt        ExtendedHookEvent = "ConfigChange"
	HookInstructionsLoadedExt  ExtendedHookEvent = "InstructionsLoaded"
	HookFileChangedExt         ExtendedHookEvent = "FileChanged"
)

// AllExtendedHookEvents lists every supported hook event name.
var AllExtendedHookEvents = []ExtendedHookEvent{
	HookPreToolUseExt, HookPostToolUseExt, HookPostToolUseFailureExt,
	HookNotificationExt, HookUserPromptSubmitExt,
	HookSessionStartExt, HookSessionEndExt,
	HookStopExt, HookStopFailureExt,
	HookSubagentStartExt, HookSubagentStopExt,
	HookPreCompactExt, HookPostCompactExt,
	HookPermissionRequestExt, HookPermissionDeniedExt,
	HookSetupExt, HookTaskCreatedExt, HookTaskCompletedExt,
	HookElicitationExt, HookElicitationResultExt,
	HookConfigChangeExt, HookInstructionsLoadedExt, HookFileChangedExt,
}

// IsValidHookEvent checks if a string is a known hook event.
func IsValidHookEvent(event string) bool {
	for _, e := range AllExtendedHookEvents {
		if string(e) == event {
			return true
		}
	}
	return false
}

// HooksSettings maps hook events to their matchers.
// This is the top-level configuration shape for an agent's hook settings.
//
// Inspired by Claude Code's HooksSettings type.
type HooksSettings map[ExtendedHookEvent][]HookMatcher

// MatchersFor returns all matchers registered for the given event.
func (hs HooksSettings) MatchersFor(event ExtendedHookEvent) []HookMatcher {
	if hs == nil {
		return nil
	}
	return hs[event]
}

// --- Hook Execution Event System ---

// HookExecutionEventType identifies a hook execution lifecycle event.
type HookExecutionEventType string

const (
	HookExecStarted  HookExecutionEventType = "started"
	HookExecProgress HookExecutionEventType = "progress"
	HookExecResponse HookExecutionEventType = "response"
)

// HookExecutionEvent tracks the lifecycle of a single hook execution.
//
// Inspired by Claude Code's hookEvents.ts.
type HookExecutionEvent struct {
	Type      HookExecutionEventType `json:"type"`
	HookID    string                 `json:"hookId"`
	HookName  string                 `json:"hookName"`
	HookEvent string                 `json:"hookEvent"`
	// Output is the combined stdout+stderr (for progress/response).
	Output string `json:"output,omitempty"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	// ExitCode is only set for response events.
	ExitCode *int `json:"exitCode,omitempty"`
	// Outcome is only set for response events.
	Outcome string `json:"outcome,omitempty"` // "success", "error", "cancelled"
}

// --- Hook JSON Output ---

// HookJSONOutput is the structured output a hook can return to control agent behavior.
//
// Inspired by Claude Code's HookJSONOutput type.
type HookJSONOutput struct {
	// Async makes the hook run in the background (non-blocking).
	Async bool `json:"async,omitempty"`
	// AsyncTimeout is the max seconds to wait for an async hook (if Async is true).
	AsyncTimeout int `json:"asyncTimeout,omitempty"`

	// Continue indicates whether the agent should proceed. False blocks the action.
	Continue *bool `json:"continue,omitempty"`
	// SuppressOutput prevents the hook's output from being shown to the user.
	SuppressOutput bool `json:"suppressOutput,omitempty"`
	// StopReason causes the agent to stop with this reason.
	StopReason string `json:"stopReason,omitempty"`
	// Decision is used by permission hooks: "approve" or "block".
	Decision string `json:"decision,omitempty"`
	// Reason explains the decision.
	Reason string `json:"reason,omitempty"`
	// SystemMessage is injected into the conversation as a system message.
	SystemMessage string `json:"systemMessage,omitempty"`
}

// IsAsync returns true if this is an asynchronous hook output.
func (o *HookJSONOutput) IsAsync() bool {
	return o.Async
}

// ShouldContinue returns whether the agent should continue after this hook.
// Defaults to true if Continue is nil.
func (o *HookJSONOutput) ShouldContinue() bool {
	if o.Continue == nil {
		return true
	}
	return *o.Continue
}

// --- Env var interpolation for HTTP hook headers ---

// InterpolateHeaders replaces $VAR_NAME references in header values with
// environment variable values, restricted to the allowedEnvVars list.
func InterpolateHeaders(headers map[string]string, allowedEnvVars []string) map[string]string {
	if len(headers) == 0 {
		return headers
	}

	allowed := make(map[string]bool, len(allowedEnvVars))
	for _, v := range allowedEnvVars {
		allowed[v] = true
	}

	result := make(map[string]string, len(headers))
	for k, v := range headers {
		result[k] = interpolateValue(v, allowed)
	}
	return result
}

// interpolateValue replaces $VAR_NAME tokens in a string with env values.
func interpolateValue(value string, allowed map[string]bool) string {
	var b strings.Builder
	i := 0
	for i < len(value) {
		if value[i] == '$' && i+1 < len(value) {
			// Extract var name (alphanumeric + underscore).
			j := i + 1
			for j < len(value) && isVarChar(value[j]) {
				j++
			}
			varName := value[i+1 : j]
			if varName != "" && allowed[varName] {
				b.WriteString(os.Getenv(varName))
			} else {
				b.WriteString(value[i:j])
			}
			i = j
		} else {
			b.WriteByte(value[i])
			i++
		}
	}
	return b.String()
}

func isVarChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}
