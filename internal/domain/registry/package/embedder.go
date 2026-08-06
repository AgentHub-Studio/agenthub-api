package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

// QueryEmbedding is a vector and the model identity required to keep cosine
// comparisons within one embedding space.
type QueryEmbedding struct {
	Vector []float32
	Model  string
}

// QueryEmbedder produces a semantic query vector for registry search.
type QueryEmbedder interface {
	Embed(ctx context.Context, text string) (QueryEmbedding, error)
}

// HTTPQueryEmbedder calls the configured AgentHub embedding service.
type HTTPQueryEmbedder struct {
	endpoint string
	client   *http.Client
}

var errEmbeddingRedirectNotAllowed = errors.New("registry: embedding redirect is not allowed")

// NewHTTPQueryEmbedder builds an embedder for an embedding service base URL.
func NewHTTPQueryEmbedder(baseURL string) *HTTPQueryEmbedder {
	return &HTTPQueryEmbedder{
		endpoint: strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/embed",
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (e *HTTPQueryEmbedder) Embed(ctx context.Context, text string) (QueryEmbedding, error) {
	endpoint, err := validateEmbeddingEndpoint(e.endpoint)
	if err != nil {
		return QueryEmbedding{}, err
	}
	body, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})
	if err != nil {
		return QueryEmbedding{}, fmt.Errorf("registry: marshal embedding request: %w", err)
	}
	// #nosec G704 -- validateEmbeddingEndpoint applies the shared SSRF URL policy before request construction.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return QueryEmbedding{}, fmt.Errorf("registry: create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// #nosec G704 -- req URL and every redirect are validated by the shared SSRF policy.
	resp, err := e.validatedHTTPClient().Do(req)
	if err != nil {
		return QueryEmbedding{}, fmt.Errorf("registry: request embedding: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return QueryEmbedding{}, fmt.Errorf("registry: embedding service HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		Embedding []float32 `json:"embedding"`
		Model     string    `json:"model"`
		Dimension int       `json:"dimension"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return QueryEmbedding{}, fmt.Errorf("registry: decode embedding response: %w", err)
	}
	if len(response.Embedding) == 0 || response.Model == "" || response.Dimension != len(response.Embedding) {
		return QueryEmbedding{}, fmt.Errorf("registry: invalid embedding response")
	}
	return QueryEmbedding{Vector: response.Embedding, Model: response.Model}, nil
}

func (e *HTTPQueryEmbedder) validatedHTTPClient() *http.Client {
	client := e.client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	protected := *client
	previousCheckRedirect := client.CheckRedirect
	protected.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if _, err := validateEmbeddingEndpoint(req.URL.String()); err != nil {
			return errEmbeddingRedirectNotAllowed
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		return nil
	}
	return &protected
}

func validateEmbeddingEndpoint(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" || strings.ContainsAny(raw, "\x00\r\n\t") {
		return nil, fmt.Errorf("registry: invalid embedding endpoint")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("registry: invalid embedding endpoint")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("registry: invalid embedding endpoint scheme")
	}
	if err := ssrf.ValidateURL(parsed.String()); err != nil {
		return nil, fmt.Errorf("registry: unsafe embedding endpoint: %w", err)
	}
	return parsed, nil
}
