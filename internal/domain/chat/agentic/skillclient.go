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
	baseURL string
	client  *http.Client
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

// ToolExecResult holds the response from a tool execution.
type ToolExecResult struct {
	Output    json.RawMessage `json:"output,omitempty"`
	Error     *string         `json:"error,omitempty"`
	LatencyMs int64           `json:"latencyMs"`
	// ToolName is set by the executor for descriptive empty-result messages.
	ToolName string `json:"toolName,omitempty"`
}

// skillExecRequest is the body sent to the skill-runtime.
type skillExecRequest struct {
	Input   map[string]any    `json:"input"`
	Context skillExecContext   `json:"context"`
}

type skillExecContext struct {
	TenantID  string `json:"tenantId"`
	AgentID   string `json:"agentId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
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

	body := skillExecRequest{
		Input: inputMap,
		Context: skillExecContext{
			TenantID:  tenantID,
			AgentID:   agentID,
			SessionID: sessionID,
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
	if tok := tenant.TokenFromContext(ctx); tok != "" {
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
		errMsg := string(respBody)
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
