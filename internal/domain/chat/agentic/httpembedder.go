package agentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPEmbedder implements Embedder using the AgentHub embedding service REST API.
// It calls POST {baseURL}/embed with {"text": "..."} and parses {"embedding": [...]}.
type HTTPEmbedder struct {
	baseURL    string
	httpClient *http.Client
}

// defaultEmbedTimeout caps how long a single /embed call is allowed to take.
// The e5-large model on CPU takes ~20–28s for a single sentence in practice;
// the prior 30s budget caused false timeouts whenever the embedding pod had
// the slightest load. 60s gives headroom without making the caller wait
// forever if the service is genuinely stuck.
const defaultEmbedTimeout = 60 * time.Second

// NewHTTPEmbedder creates an HTTPEmbedder that sends requests to baseURL/embed.
func NewHTTPEmbedder(baseURL string) *HTTPEmbedder {
	return NewHTTPEmbedderWithTimeout(baseURL, defaultEmbedTimeout)
}

// NewHTTPEmbedderWithTimeout is like NewHTTPEmbedder but allows overriding
// the per-request timeout, primarily for tests that need a tight deadline.
func NewHTTPEmbedderWithTimeout(baseURL string, timeout time.Duration) *HTTPEmbedder {
	return &HTTPEmbedder{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type httpEmbedRequest struct {
	Text string `json:"text"`
}

type httpEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed calls the embedding service and returns the dense vector for text.
func (e *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(httpEmbedRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("httpembedder: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("httpembedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpembedder: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("httpembedder: embedding service returned HTTP %d", resp.StatusCode)
	}

	var result httpEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("httpembedder: decode response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("httpembedder: service returned empty vector")
	}
	return result.Embedding, nil
}
