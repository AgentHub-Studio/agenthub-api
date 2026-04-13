package agentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// HookEvent identifies when a hook fires.
type HookEvent string

const (
	HookPreToolUse      HookEvent = "pre_tool_use"
	HookPostToolUse     HookEvent = "post_tool_use"
	HookPostToolFailure HookEvent = "post_tool_failure"
	HookSessionStart    HookEvent = "session_start"
	HookSessionEnd      HookEvent = "session_end"
	HookNotification    HookEvent = "notification"
	HookTurnEnd         HookEvent = "turn_end"
	HookRunEnd          HookEvent = "run_end"
)

// HookType identifies the kind of action a hook performs.
type HookType string

const (
	HookTypeHTTP   HookType = "http"
	HookTypePrompt HookType = "prompt"
)

// AgentHook is the domain entity for a hook attached to an agent.
type AgentHook struct {
	ID             uuid.UUID       `json:"id"`
	AgentID        uuid.UUID       `json:"agentId"`
	Event          HookEvent       `json:"event"`
	Matcher        string          `json:"matcher,omitempty"`
	HookType       HookType        `json:"hookType"`
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	Priority       int             `json:"priority"`
	// TimeoutSeconds overrides the global hook timeout for this specific hook.
	// Inspired by Claude Code's per-hook timeout field.
	TimeoutSeconds *int            `json:"timeoutSeconds,omitempty"`
	// IsAsync runs the hook in background without blocking the agentic loop.
	// Inspired by Claude Code's async hook flag.
	IsAsync        bool            `json:"isAsync"`
	// RunOnce causes the hook to fire once then auto-disable.
	// Inspired by Claude Code's once flag for one-shot hooks.
	RunOnce        bool            `json:"runOnce"`
	// StatusMessage is a custom spinner message shown while the hook runs.
	StatusMessage  string          `json:"statusMessage,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// HTTPHookConfig is the config shape for hook_type = "http".
type HTTPHookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout int               `json:"timeoutMs,omitempty"` // milliseconds
}

// PromptHookConfig is the config shape for hook_type = "prompt".
// Both "template" and "inject" are accepted; "template" takes priority when both are set.
// "inject" is provided as an intuitive alias for static text that requires no substitution.
type PromptHookConfig struct {
	Template string `json:"template"` // Go text/template with {{.ToolName}}, {{.Input}}, {{.Output}}
	Inject   string `json:"inject"`   // alias for static inject text (no templating)
}

// HookPayload is the data sent to hook executors and used as the template
// data object when rendering prompt hooks. All fields are optional — only those
// relevant to the specific event are populated.
type HookPayload struct {
	Event      HookEvent       `json:"event"`
	AgentID    string          `json:"agentId"`
	SessionID  string          `json:"sessionId"`
	ToolName   string          `json:"toolName,omitempty"`
	ToolInput  json.RawMessage `json:"toolInput,omitempty"`
	ToolOutput json.RawMessage `json:"toolOutput,omitempty"`
	ToolError  *string         `json:"toolError,omitempty"`
	// Turn-end / run-end fields (populated for turn_end and run_end events).
	// These allow prompt hook templates to use {{.TurnIndex}}, {{.TotalTurns}}, etc.
	TurnIndex        int            `json:"turnIndex,omitempty"`
	AssistantContent string         `json:"assistantContent,omitempty"`
	ToolCalls        []ToolCallInfo `json:"toolCalls,omitempty"`
	TotalTurns       int            `json:"totalTurns,omitempty"`
	TotalTokens      int            `json:"totalTokens,omitempty"`
	TotalCostUSD     float64        `json:"totalCostUSD,omitempty"`
}

// HookResult holds the result of a hook execution.
type HookResult struct {
	Inject string  // text to inject into context (prompt hooks)
	Error  *string // non-nil if hook failed (non-fatal)
}

// --- Hook Repository ---

// HookRepository loads and manages hooks for an agent.
type HookRepository interface {
	FindByAgentAndEvent(ctx context.Context, agentID uuid.UUID, event HookEvent) ([]AgentHook, error)
	DisableHook(ctx context.Context, hookID uuid.UUID) error
}

type pgHookRepository struct {
	pool *pgxpool.Pool
}

// NewHookRepository creates a PostgreSQL-backed hook repository.
func NewHookRepository(pool *pgxpool.Pool) HookRepository {
	return &pgHookRepository{pool: pool}
}

func (r *pgHookRepository) FindByAgentAndEvent(ctx context.Context, agentID uuid.UUID, event HookEvent) ([]AgentHook, error) {
	// BUG-HOOK2 fix: use tenant-aware connection so search_path is set correctly.
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, fmt.Errorf("hook repo: acquire: %w", err)
	}
	defer release()

	query := `SELECT id, agent_id, event, matcher, hook_type, config, enabled, priority,
		       timeout_seconds, is_async, run_once, status_message,
		       created_at, updated_at
		FROM agent_hook
		WHERE agent_id = $1 AND event = $2 AND enabled = TRUE
		ORDER BY priority ASC, created_at ASC`

	rows, err := conn.Query(ctx, query, agentID, string(event))
	if err != nil {
		return nil, fmt.Errorf("hook repo: query: %w", err)
	}
	defer rows.Close()

	var hooks []AgentHook
	for rows.Next() {
		h, err := scanHook(rows)
		if err != nil {
			return nil, fmt.Errorf("hook repo: scan: %w", err)
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

func (r *pgHookRepository) DisableHook(ctx context.Context, hookID uuid.UUID) error {
	// BUG-HOOK2 fix: use tenant-aware connection for UPDATE as well.
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return fmt.Errorf("hook repo: acquire for disable: %w", err)
	}
	defer release()
	_, err = conn.Exec(ctx, `UPDATE agent_hook SET enabled = FALSE, updated_at = NOW() WHERE id = $1`, hookID)
	if err != nil {
		return fmt.Errorf("hook repo: disable: %w", err)
	}
	return nil
}

func scanHook(row pgx.Row) (AgentHook, error) {
	var h AgentHook
	var event, hookType string
	var statusMsg *string
	err := row.Scan(
		&h.ID, &h.AgentID, &event, &h.Matcher, &hookType, &h.Config,
		&h.Enabled, &h.Priority,
		&h.TimeoutSeconds, &h.IsAsync, &h.RunOnce, &statusMsg,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return AgentHook{}, err
	}
	h.Event = HookEvent(event)
	h.HookType = HookType(hookType)
	if statusMsg != nil {
		h.StatusMessage = *statusMsg
	}
	return h, nil
}

// --- Hook Executor ---

// HookExecutor runs hooks for an agent at a given event.
type HookExecutor struct {
	repo   HookRepository
	client *http.Client
}

// NewHookExecutor creates a HookExecutor.
func NewHookExecutor(repo HookRepository) *HookExecutor {
	return &HookExecutor{
		repo: repo,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Execute runs all matching hooks for the given event and returns combined results.
// Errors in individual hooks are logged but do not stop execution.
// Async hooks (IsAsync=true) run in background goroutines and do not produce results.
// RunOnce hooks are auto-disabled after the first execution.
// Per-hook TimeoutSeconds overrides the global hook timeout.
func (e *HookExecutor) Execute(ctx context.Context, payload HookPayload) []HookResult {
	agentID, err := uuid.Parse(payload.AgentID)
	if err != nil {
		return nil
	}

	hooks, err := e.repo.FindByAgentAndEvent(ctx, agentID, payload.Event)
	if err != nil {
		slog.Warn("hook executor: failed to load hooks", "error", err)
		return nil
	}

	if len(hooks) > 0 {
		slog.Debug("hook executor: loaded hooks", "event", payload.Event, "count", len(hooks), "toolName", payload.ToolName)
	}

	var results []HookResult
	for _, hook := range hooks {
		if !matchesToolName(hook.Matcher, payload.ToolName) {
			continue
		}
		slog.Info("hook executor: firing hook", "hookID", hook.ID, "event", hook.Event, "hookType", hook.HookType, "toolName", payload.ToolName)

		// Apply per-hook timeout if set.
		hookCtx := ctx
		var cancel context.CancelFunc
		if hook.TimeoutSeconds != nil && *hook.TimeoutSeconds > 0 {
			hookCtx, cancel = context.WithTimeout(ctx, time.Duration(*hook.TimeoutSeconds)*time.Second)
		}

		// Async hooks run in background without blocking the agentic loop.
		if hook.IsAsync {
			go func(h AgentHook, hctx context.Context, cfn context.CancelFunc) {
				if cfn != nil {
					defer cfn()
				}
				result := e.executeHook(hctx, h, payload)
				if result.Error != nil {
					slog.Warn("async hook failed", "hookID", h.ID, "error", *result.Error)
				}
				if h.RunOnce {
					e.disableHook(ctx, h.ID)
				}
			}(hook, hookCtx, cancel)
			continue
		}

		result := e.executeHook(hookCtx, hook, payload)
		if cancel != nil {
			cancel()
		}
		// BUG-HOOK-ERROR-SILENT: log synchronous hook errors so operators can
		// diagnose misconfigured or unreachable hooks without having to trace
		// SSE events. Async hooks already log errors in their goroutine.
		if result.Error != nil {
			slog.Warn("hook executor: hook failed", "hookID", hook.ID, "event", hook.Event,
				"hookType", hook.HookType, "toolName", payload.ToolName, "error", *result.Error)
		}
		results = append(results, result)

		// RunOnce hooks are disabled after the first synchronous execution.
		if hook.RunOnce {
			e.disableHook(ctx, hook.ID)
		}
	}
	return results
}

// disableHook marks a hook as disabled (used for RunOnce hooks).
func (e *HookExecutor) disableHook(ctx context.Context, hookID uuid.UUID) {
	if err := e.repo.DisableHook(ctx, hookID); err != nil {
		slog.Warn("hook executor: failed to disable run_once hook", "hookID", hookID, "error", err)
	}
}

func (e *HookExecutor) executeHook(ctx context.Context, hook AgentHook, payload HookPayload) HookResult {
	switch hook.HookType {
	case HookTypeHTTP:
		return e.executeHTTPHook(ctx, hook, payload)
	case HookTypePrompt:
		return e.executePromptHook(hook, payload)
	default:
		errMsg := fmt.Sprintf("unknown hook type: %s", hook.HookType)
		return HookResult{Error: &errMsg}
	}
}

func (e *HookExecutor) executeHTTPHook(ctx context.Context, hook AgentHook, payload HookPayload) HookResult {
	var cfg HTTPHookConfig
	if err := json.Unmarshal(hook.Config, &cfg); err != nil {
		errMsg := fmt.Sprintf("invalid http hook config: %v", err)
		return HookResult{Error: &errMsg}
	}

	method := cfg.Method
	if method == "" {
		method = http.MethodPost
	}

	body, _ := json.Marshal(payload)

	timeout := 10 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Millisecond
	}
	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(hookCtx, method, cfg.URL, bytes.NewReader(body))
	if err != nil {
		errMsg := fmt.Sprintf("hook request error: %v", err)
		return HookResult{Error: &errMsg}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("hook http error: %v", err)
		return HookResult{Error: &errMsg}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10_000))

	if resp.StatusCode >= 400 {
		errMsg := fmt.Sprintf("hook returned %d: %s", resp.StatusCode, string(respBody))
		return HookResult{Error: &errMsg}
	}

	// If hook returns text, use it as inject content.
	return HookResult{Inject: string(respBody)}
}

func (e *HookExecutor) executePromptHook(hook AgentHook, payload HookPayload) HookResult {
	var cfg PromptHookConfig
	if err := json.Unmarshal(hook.Config, &cfg); err != nil {
		errMsg := fmt.Sprintf("invalid prompt hook config: %v", err)
		return HookResult{Error: &errMsg}
	}

	// Prefer template; fall back to inject alias for static text.
	raw := cfg.Template
	if raw == "" {
		raw = cfg.Inject
	}

	// BUG-HOOK-TEMPLATE: execute Go text/template substitutions so that
	// {{.ToolName}}, {{.ToolInput}}, {{.ToolOutput}} etc. are expanded.
	// If the template has no actions (no {{...}}), this is a no-op and the
	// raw string is returned unchanged — fully backward-compatible.
	if strings.Contains(raw, "{{") {
		tmpl, err := template.New("hook").Parse(raw)
		if err != nil {
			// Malformed template — fall back to raw string rather than erroring.
			slog.Warn("prompt hook: malformed template, using raw text", "hookID", hook.ID, "error", err)
			return HookResult{Inject: raw}
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, payload); err != nil {
			slog.Warn("prompt hook: template execute failed, using raw text", "hookID", hook.ID, "error", err)
			return HookResult{Inject: raw}
		}
		return HookResult{Inject: buf.String()}
	}

	return HookResult{Inject: raw}
}

// --- Turn-End / Run-End Payloads ---

// TurnEndPayload is the data sent to turn-end hooks after all tool executions complete.
type TurnEndPayload struct {
	Event            HookEvent       `json:"event"`
	AgentID          string          `json:"agentId"`
	SessionID        string          `json:"sessionId"`
	TurnIndex        int             `json:"turnIndex"`
	AssistantContent string          `json:"assistantContent,omitempty"`
	ToolCalls        []ToolCallInfo  `json:"toolCalls,omitempty"`
	TokenUsage       json.RawMessage `json:"tokenUsage,omitempty"`
}

// ToolCallInfo is a simplified view of a tool call for hook payloads.
type ToolCallInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RunEndPayload is the data sent to run-end hooks after the run completes.
type RunEndPayload struct {
	Event       HookEvent `json:"event"`
	AgentID     string    `json:"agentId"`
	SessionID   string    `json:"sessionId"`
	TotalTurns  int       `json:"totalTurns"`
	TotalTokens int       `json:"totalTokens"`
	TotalCost   float64   `json:"totalCostUsd,omitempty"`
}

// --- Turn-End Handler ---

// TurnEndHandler is executed at the end of each agentic turn.
// Handlers are non-fatal: errors are logged but do not stop the run.
type TurnEndHandler interface {
	HandleTurnEnd(ctx context.Context, payload TurnEndPayload) error
}

// RunEndHandler is executed at the end of an agentic run.
type RunEndHandler interface {
	HandleRunEnd(ctx context.Context, payload RunEndPayload) error
}

// ExecuteTurnEnd runs all turn-end hooks (persisted + in-memory handlers).
// BUG-HOOK-TURNEND-INJECT: returns inject texts from persisted hooks so the
// runner can emit [SYSTEM NOTE from hook] user messages before the next turn.
// Previously the inject field was discarded; only Error was checked.
func (e *HookExecutor) ExecuteTurnEnd(ctx context.Context, payload TurnEndPayload, handlers []TurnEndHandler) []string {
	var injects []string

	// 1. Execute persisted hooks (HTTP/prompt).
	agentID, err := uuid.Parse(payload.AgentID)
	if err == nil && e.repo != nil {
		hooks, err := e.repo.FindByAgentAndEvent(ctx, agentID, HookTurnEnd)
		if err != nil {
			slog.Warn("hook executor: failed to load turn-end hooks", "error", err)
		} else {
			for _, hook := range hooks {
				// Turn-end hooks don't use tool matcher — execute all.
				// Populate turn-end fields so prompt templates can use
				// {{.TurnIndex}}, {{.AssistantContent}}, {{.ToolCalls}}, etc.
				hookPayload := HookPayload{
					Event:            HookTurnEnd,
					AgentID:          payload.AgentID,
					SessionID:        payload.SessionID,
					TurnIndex:        payload.TurnIndex,
					AssistantContent: payload.AssistantContent,
					ToolCalls:        payload.ToolCalls,
				}
				result := e.executeHook(ctx, hook, hookPayload)
				if result.Error != nil {
					slog.Warn("turn-end hook failed", "hookId", hook.ID, "error", *result.Error)
				}
				if result.Inject != "" {
					injects = append(injects, result.Inject)
				}
			}
		}
	}

	// 2. Execute in-memory handlers (e.g., memory store).
	for _, h := range handlers {
		if err := h.HandleTurnEnd(ctx, payload); err != nil {
			slog.Warn("turn-end handler failed", "error", err)
		}
	}

	return injects
}

// ExecuteRunEnd runs all run-end hooks (persisted + in-memory handlers).
// Returns inject texts from prompt hooks so the runner can persist them as audit
// notes in the session history (there is no next LLM turn to inject into, but
// the notes remain visible in the conversation log for audit purposes).
func (e *HookExecutor) ExecuteRunEnd(ctx context.Context, payload RunEndPayload, handlers []RunEndHandler) []string {
	var injects []string
	agentID, err := uuid.Parse(payload.AgentID)
	if err == nil && e.repo != nil {
		hooks, err := e.repo.FindByAgentAndEvent(ctx, agentID, HookRunEnd)
		if err != nil {
			slog.Warn("hook executor: failed to load run-end hooks", "error", err)
		} else {
			for _, hook := range hooks {
				// Populate run-end fields so prompt templates can use
				// {{.TotalTurns}}, {{.TotalTokens}}, {{.TotalCostUSD}}, etc.
				hookPayload := HookPayload{
					Event:        HookRunEnd,
					AgentID:      payload.AgentID,
					SessionID:    payload.SessionID,
					TotalTurns:   payload.TotalTurns,
					TotalTokens:  payload.TotalTokens,
					TotalCostUSD: payload.TotalCost,
				}
				result := e.executeHook(ctx, hook, hookPayload)
				if result.Error != nil {
					slog.Warn("run-end hook failed", "hookId", hook.ID, "error", *result.Error)
				}
				if result.Inject != "" {
					injects = append(injects, result.Inject)
				}
			}
		}
	}

	for _, h := range handlers {
		if err := h.HandleRunEnd(ctx, payload); err != nil {
			slog.Warn("run-end handler failed", "error", err)
		}
	}
	return injects
}

// --- Memory Turn-End Handler ---

// MemoryTurnEndHandler wraps MemoryBridge.MaybeStore as a TurnEndHandler.
type MemoryTurnEndHandler struct {
	memory *MemoryBridge
}

// NewMemoryTurnEndHandler creates a handler that evaluates and stores memories at turn end.
func NewMemoryTurnEndHandler(memory *MemoryBridge) *MemoryTurnEndHandler {
	return &MemoryTurnEndHandler{memory: memory}
}

// HandleTurnEnd evaluates the turn's content for memorable information.
func (h *MemoryTurnEndHandler) HandleTurnEnd(ctx context.Context, payload TurnEndPayload) error {
	if h.memory == nil {
		return nil
	}

	agentID, err := uuid.Parse(payload.AgentID)
	if err != nil {
		return fmt.Errorf("memory turn-end: invalid agent ID: %w", err)
	}

	var turnMsgs []TurnMessage
	if payload.AssistantContent != "" {
		turnMsgs = append(turnMsgs, TurnMessage{Role: "assistant", Content: payload.AssistantContent})
	}
	for _, tc := range payload.ToolCalls {
		turnMsgs = append(turnMsgs, TurnMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("[tool_call: %s]", tc.Name),
		})
	}

	_, err = h.memory.MaybeStore(ctx, agentID, payload.TurnIndex, turnMsgs)
	return err
}

// matchesToolName checks if a tool name matches a glob pattern.
// Empty matcher matches everything.
func matchesToolName(matcher, toolName string) bool {
	if matcher == "" {
		return true
	}
	if toolName == "" {
		return false
	}
	matched, err := filepath.Match(matcher, toolName)
	if err != nil {
		return false
	}
	return matched
}
