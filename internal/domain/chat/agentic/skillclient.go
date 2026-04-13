package agentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// SkillRuntimeClient calls the skill-runtime service to execute tools.
type SkillRuntimeClient struct {
	baseURL      string
	serviceToken string // P-C282-1: service account token for runner→skill-runtime auth
	client       *http.Client
}

// NewSkillRuntimeClient creates a client pointing at the given skill-runtime base URL.
// If baseURL is empty it defaults to http://agenthub-skill-runtime:8083.
func NewSkillRuntimeClient(baseURL string) *SkillRuntimeClient {
	if baseURL == "" {
		baseURL = "http://agenthub-skill-runtime:8083"
	}
	return &SkillRuntimeClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second, // outer safety net; per-call uses ctx deadline
		},
	}
}

// WithServiceToken configures a service account token that the client uses for
// all requests to the skill-runtime. When set, the user's context token is NOT
// forwarded, preventing privilege escalation.
// P-C282-1: runner must authenticate to skill-runtime with its own service identity.
func (c *SkillRuntimeClient) WithServiceToken(token string) *SkillRuntimeClient {
	c.serviceToken = token
	return c
}

// ToolExecResult holds the response from a tool execution.
type ToolExecResult struct {
	Output    json.RawMessage `json:"output,omitempty"`
	Error     *string         `json:"error,omitempty"`
	LatencyMs int64           `json:"latencyMs"`
	// ToolName is set by the executor for descriptive empty-result messages.
	ToolName string `json:"toolName,omitempty"`
	// EmittedToStream is set to true by executors that already emitted
	// EventToolResult to the SSE channel. The main loop skips re-emission
	// to avoid duplicate tool_result events.
	EmittedToStream bool `json:"-"`
	// SubtaskTokens and SubtaskCostUSD carry the aggregate token/cost consumed
	// by a sub-agent run. Set by SubtaskExecutor so the parent runner can roll
	// up these values into its own totals (ACT-F3-13 / P-C336-1).
	SubtaskTokens  int     `json:"-"`
	SubtaskCostUSD float64 `json:"-"`
	// InjectText carries text that a post_tool_use hook wants injected into the
	// conversation as a separate [SYSTEM NOTE] user message — NOT appended to the
	// tool result itself (which caused LLM retry loops, see BUG-HOOK-PROMPT-INJECT).
	// The runner collects all non-empty InjectText values after a tool-execution turn
	// and prepends them as a single user message before the next LLM call.
	InjectText string `json:"-"`
}

// skillExecRequest is the body sent to the skill-runtime.
type skillExecRequest struct {
	Input   map[string]any    `json:"input"`
	Context skillExecContext   `json:"context"`
}

type skillExecContext struct {
	TenantID    string `json:"tenantId"`
	AgentID     string `json:"agentId,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	// CallerToken is the raw Bearer JWT of the authenticated user.
	// When populated, the skill-runtime uses it to execute tools configured with
	// use_caller_token=true (e.g. ah_core platform tools) in the caller's context.
	CallerToken string `json:"callerToken,omitempty"`
}

// skillExecResponse is the response from the skill-runtime.
type skillExecResponse struct {
	Output    json.RawMessage `json:"output"`
	Error     *string         `json:"error"`
	LatencyMs int64           `json:"latencyMs"`
}

// Execute calls a single skill by slug and returns the result.
func (c *SkillRuntimeClient) Execute(ctx context.Context, slug string, input json.RawMessage, tenantID, agentID, sessionID string) (*ToolExecResult, error) {
	var inputMap map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &inputMap); err != nil {
			// If input is not a map, wrap it.
			inputMap = map[string]any{"input": string(input)}
		}
	}
	if inputMap == nil {
		inputMap = map[string]any{}
	}

	// Propagate the caller's JWT so the skill-runtime can use it for tools
	// configured with use_caller_token=true (e.g. ah_core platform tools).
	// This is always forwarded regardless of whether a service token is used for
	// the Authorization header — the two tokens serve different purposes.
	callerToken := tenant.TokenFromContext(ctx)
	body := skillExecRequest{
		Input: inputMap,
		Context: skillExecContext{
			TenantID:    tenantID,
			AgentID:     agentID,
			SessionID:   sessionID,
			CallerToken: callerToken,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("skillclient: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/skills/%s/execute", c.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("skillclient: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// P-C282-1: prefer service account token over user token.
	// When a service token is configured, it is used exclusively — the user's JWT
	// is never forwarded to the skill-runtime to avoid privilege escalation.
	if c.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.serviceToken)
	} else if tok := tenant.TokenFromContext(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	start := time.Now()
	resp, err := c.client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("skillclient: execute %s: %w", slug, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("skillclient: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		// Unwrap JSON error envelope {"error":"..."} from skill-runtime to avoid
		// double-encoding the message in the tool_result SSE event.
		errMsg := string(respBody)
		var errEnvelope struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errEnvelope) == nil && errEnvelope.Error != "" {
			errMsg = errEnvelope.Error
		}
		return &ToolExecResult{
			Error:     &errMsg,
			LatencyMs: elapsed,
		}, nil
	}

	var execResp skillExecResponse
	if err := json.Unmarshal(respBody, &execResp); err != nil {
		// If we can't parse the response, return the raw body as output.
		return &ToolExecResult{
			Output:    respBody,
			LatencyMs: elapsed,
		}, nil
	}

	return &ToolExecResult{
		Output:    execResp.Output,
		Error:     execResp.Error,
		LatencyMs: max(execResp.LatencyMs, elapsed),
	}, nil
}

// ToolCall represents a pending tool invocation for parallel execution.
type ToolCall struct {
	ID        string
	Slug      string
	Input     json.RawMessage
	TenantID  string
	AgentID   string
	SessionID string
}

// ExecuteParallel runs multiple tool calls concurrently up to the concurrency limit.
func (c *SkillRuntimeClient) ExecuteParallel(ctx context.Context, calls []ToolCall, concurrency int) []ToolExecResult {
	if concurrency <= 0 {
		concurrency = 3
	}

	results := make([]ToolExecResult, len(calls))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc ToolCall) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := c.Execute(ctx, tc.Slug, tc.Input, tc.TenantID, tc.AgentID, tc.SessionID)
			if err != nil {
				errMsg := err.Error()
				results[idx] = ToolExecResult{Error: &errMsg}
				return
			}
			results[idx] = *result
		}(i, call)
	}

	wg.Wait()
	return results
}
