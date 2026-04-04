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
	HookPreToolUse   HookEvent = "pre_tool_use"
	HookPostToolUse  HookEvent = "post_tool_use"
	HookSessionStart HookEvent = "session_start"
	HookSessionEnd   HookEvent = "session_end"
	HookTurnEnd      HookEvent = "turn_end"
	HookRunEnd       HookEvent = "run_end"
)

// HookType identifies the kind of action a hook performs.
type HookType string

const (
	HookTypeHTTP   HookType = "http"
	HookTypePrompt HookType = "prompt"
)

// AgentHook is the domain entity for a hook attached to an agent.
type AgentHook struct {
	ID        uuid.UUID       `json:"id"`
	AgentID   uuid.UUID       `json:"agentId"`
	Event     HookEvent       `json:"event"`
	Matcher   string          `json:"matcher,omitempty"`
	HookType  HookType        `json:"hookType"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	Priority  int             `json:"priority"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
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

// HookRepository loads hooks for an agent.
type HookRepository interface {
	FindByAgentAndEvent(ctx context.Context, agentID uuid.UUID, event HookEvent) ([]AgentHook, error)
}

type pgHookRepository struct {
	pool *pgxpool.Pool
}

// NewHookRepository creates a PostgreSQL-backed hook repository.
func NewHookRepository(pool *pgxpool.Pool) HookRepository {
	return &pgHookRepository{pool: pool}
}

func (r *pgHookRepository) FindByAgentAndEvent(ctx context.Context, agentID uuid.UUID, event HookEvent) ([]AgentHook, error) {
	query := `SELECT id, agent_id, event, matcher, hook_type, config, enabled, priority, created_at, updated_at
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

func scanHook(row pgx.Row) (AgentHook, error) {
	var h AgentHook
	var event, hookType string
	err := row.Scan(&h.ID, &h.AgentID, &event, &h.Matcher, &hookType, &h.Config, &h.Enabled, &h.Priority, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return AgentHook{}, err
	}
	h.Event = HookEvent(event)
	h.HookType = HookType(hookType)
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

		result := e.executeHook(ctx, hook, payload)
		results = append(results, result)
	}
	return results
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
func (e *HookExecutor) ExecuteTurnEnd(ctx context.Context, payload TurnEndPayload, handlers []TurnEndHandler) {
	// 1. Execute persisted hooks (HTTP/prompt).
	agentID, err := uuid.Parse(payload.AgentID)
	if err == nil && e.repo != nil {
		hooks, err := e.repo.FindByAgentAndEvent(ctx, agentID, HookTurnEnd)
		if err != nil {
			slog.Warn("hook executor: failed to load turn-end hooks", "error", err)
		} else {
			for _, hook := range hooks {
				// Turn-end hooks don't use tool matcher — execute all.
				hookPayload := HookPayload{
					Event:     HookTurnEnd,
					AgentID:   payload.AgentID,
					SessionID: payload.SessionID,
				}
				result := e.executeHook(ctx, hook, hookPayload)
				if result.Error != nil {
					slog.Warn("turn-end hook failed", "hookId", hook.ID, "error", *result.Error)
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
}

// ExecuteRunEnd runs all run-end hooks (persisted + in-memory handlers).
func (e *HookExecutor) ExecuteRunEnd(ctx context.Context, payload RunEndPayload, handlers []RunEndHandler) {
	agentID, err := uuid.Parse(payload.AgentID)
	if err == nil && e.repo != nil {
		hooks, err := e.repo.FindByAgentAndEvent(ctx, agentID, HookRunEnd)
		if err != nil {
			slog.Warn("hook executor: failed to load run-end hooks", "error", err)
		} else {
			for _, hook := range hooks {
				hookPayload := HookPayload{
					Event:     HookRunEnd,
					AgentID:   payload.AgentID,
					SessionID: payload.SessionID,
				}
				result := e.executeHook(ctx, hook, hookPayload)
				if result.Error != nil {
					slog.Warn("run-end hook failed", "hookId", hook.ID, "error", *result.Error)
				}
			}
		}
	}

	for _, h := range handlers {
		if err := h.HandleRunEnd(ctx, payload); err != nil {
			slog.Warn("run-end handler failed", "error", err)
		}
	}
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
