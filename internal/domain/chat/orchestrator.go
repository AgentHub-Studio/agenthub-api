package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OrchestratorClient calls the agenthub-orchestrator execution API.
type OrchestratorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOrchestratorClient creates a client that calls the orchestrator at baseURL.
// Pass an empty baseURL to get a no-op client that returns empty responses.
func NewOrchestratorClient(baseURL string) *OrchestratorClient {
	return &OrchestratorClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// executionRequest is the payload sent to POST /api/executions on the orchestrator.
type executionRequest struct {
	AgentID string          `json:"agentId"`
	Input   json.RawMessage `json:"input"`
}

// executionResponse is the response from the orchestrator's execution endpoint.
type executionResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output string `json:"output"` // populated when execution completes synchronously
}

// RunAgentExecution starts an execution on the orchestrator for the given agent
// with the provided user message as input. It returns the assistant reply text.
// If the orchestrator URL is empty or the call fails, an empty string is returned
// (graceful degradation — the user message is still stored).
func (c *OrchestratorClient) RunAgentExecution(ctx context.Context, bearerToken string, agentID string, userMessage string) (string, error) {
	if c.baseURL == "" {
		return "", nil
	}

	input, err := json.Marshal(map[string]any{"message": userMessage})
	if err != nil {
		return "", fmt.Errorf("chat orchestrator: marshal input: %w", err)
	}

	payload, err := json.Marshal(executionRequest{
		AgentID: agentID,
		Input:   json.RawMessage(input),
	})
	if err != nil {
		return "", fmt.Errorf("chat orchestrator: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/executions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("chat orchestrator: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat orchestrator: do request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat orchestrator: unexpected status %d", resp.StatusCode)
	}

	var execResp executionResponse
	if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
		return "", fmt.Errorf("chat orchestrator: decode response: %w", err)
	}

	// If the execution already completed synchronously, return immediately.
	if execResp.Status == "COMPLETED" {
		return execResp.Output, nil
	}
	if execResp.Status == "FAILED" {
		return "", fmt.Errorf("chat orchestrator: execution failed")
	}

	// Poll for completion — the orchestrator runs pipelines asynchronously.
	execID := execResp.ID
	maxAttempts := 60
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}

		getReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
			c.baseURL+"/api/executions/"+execID, nil)
		if err != nil {
			continue
		}
		getReq.Header.Set("Authorization", bearerToken)

		getResp, err := c.httpClient.Do(getReq)
		if err != nil {
			continue
		}
		var polledResp executionResponse
		json.NewDecoder(getResp.Body).Decode(&polledResp) //nolint:errcheck
		getResp.Body.Close()                              //nolint:errcheck

		if polledResp.Status == "COMPLETED" {
			return polledResp.Output, nil
		}
		if polledResp.Status == "FAILED" {
			return "", fmt.Errorf("chat orchestrator: execution failed")
		}
	}
	return "", fmt.Errorf("chat orchestrator: execution timed out after %d attempts", maxAttempts)
}
