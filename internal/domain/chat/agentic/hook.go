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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
type PromptHookConfig struct {
	Template string `json:"template"` // Go text/template with {{.ToolName}}, {{.Input}}, {{.Output}}
}

// HookPayload is the data sent to hook executors.
type HookPayload struct {
	Event     HookEvent       `json:"event"`
	AgentID   string          `json:"agentId"`
	SessionID string          `json:"sessionId"`
	ToolName  string          `json:"toolName,omitempty"`
	ToolInput json.RawMessage `json:"toolInput,omitempty"`
	ToolOutput json.RawMessage `json:"toolOutput,omitempty"`
	ToolError  *string         `json:"toolError,omitempty"`
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
	query := `SELECT id, agent_id, event, matcher, hook_type, config, enabled, priority,
		       timeout_seconds, is_async, run_once, status_message,
		       created_at, updated_at
		FROM agent_hook
		WHERE agent_id = $1 AND event = $2 AND enabled = TRUE
		ORDER BY priority ASC, created_at ASC`

	rows, err := r.pool.Query(ctx, query, agentID, string(event))
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
	_, err := r.pool.Exec(ctx, `UPDATE agent_hook SET enabled = FALSE, updated_at = NOW() WHERE id = $1`, hookID)
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

	var results []HookResult
	for _, hook := range hooks {
		if !matchesToolName(hook.Matcher, payload.ToolName) {
			continue
		}

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

	// Simple variable substitution (no full template engine to avoid injection).
	result := cfg.Template
	return HookResult{Inject: result}
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
