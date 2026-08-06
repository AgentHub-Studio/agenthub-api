package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	fastembed "github.com/AgentHub-Studio/agenthub-api/internal/infra/embedding/fastembed"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

const (
	ProviderFastEmbed = "fastembed"
	ProviderPython    = "python"
)

var (
	ErrEmptyText                         = errors.New("embedding: text is empty")
	ErrUnknownProvider                   = errors.New("embedding: unknown provider")
	errPythonEmbeddingRedirectNotAllowed = errors.New("embedding: python redirect target is not allowed")
)

type Config struct {
	DefaultProvider string
	PythonURL       string
	PythonTimeout   time.Duration
}

type Result struct {
	Vector         []float32 `json:"vector"`
	Provider       string    `json:"provider"`
	SourceProvider string    `json:"sourceProvider"`
	Model          string    `json:"model"`
	Dimension      int       `json:"dimension"`
}

type Service struct {
	defaultProvider string
	fastEmbedder    *fastembed.Embedder
	pythonClient    *pythonClient
}

func NewService(cfg Config) *Service {
	defaultProvider := normalizeProvider(cfg.DefaultProvider)
	if defaultProvider == "" {
		defaultProvider = ProviderPython
	}

	var py *pythonClient
	if strings.TrimSpace(cfg.PythonURL) != "" {
		timeout := cfg.PythonTimeout
		if timeout <= 0 {
			timeout = 750 * time.Millisecond
		}
		py = newPythonClient(cfg.PythonURL, timeout)
	}

	return &Service{
		defaultProvider: defaultProvider,
		fastEmbedder:    fastembed.New(),
		pythonClient:    py,
	}
}

func (s *Service) Embed(ctx context.Context, text, provider string) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, ErrEmptyText
	}

	provider = normalizeProvider(provider)
	if provider == "" {
		provider = s.defaultProvider
	}

	switch provider {
	case ProviderFastEmbed:
		return s.embedFast(text, ProviderFastEmbed, ProviderFastEmbed)
	case ProviderPython:
		if s.defaultProvider != ProviderFastEmbed && s.pythonClient != nil {
			if result, err := s.pythonClient.Embed(ctx, text); err == nil {
				return result, nil
			}
		}
		return s.embedFast(text, ProviderPython, ProviderFastEmbed)
	default:
		return Result{}, fmt.Errorf("%w: %s", ErrUnknownProvider, provider)
	}
}

func (s *Service) embedFast(text, requestedProvider, sourceProvider string) (Result, error) {
	vec, err := s.fastEmbedder.Embed(text)
	if err != nil {
		if errors.Is(err, fastembed.ErrEmptyText) {
			return Result{}, ErrEmptyText
		}
		return Result{}, err
	}
	return Result{
		Vector:         vec,
		Provider:       requestedProvider,
		SourceProvider: sourceProvider,
		Model:          fastembed.ModelName,
		Dimension:      len(vec),
	}, nil
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

type pythonClient struct {
	baseURL    string
	httpClient *http.Client
}

func newPythonClient(baseURL string, timeout time.Duration) *pythonClient {
	return &pythonClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{Timeout: timeout},
	}
}

type pythonEmbedRequest struct {
	Text string `json:"text"`
}

type pythonEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
	Dimension int       `json:"dimension"`
}

func (c *pythonClient) Embed(ctx context.Context, text string) (Result, error) {
	body, err := json.Marshal(pythonEmbedRequest{Text: text})
	if err != nil {
		return Result{}, fmt.Errorf("embedding: marshal python request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("embedding: build python request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// #nosec G704 -- the configured embedding service is infrastructure-owned and every redirect is checked against the SSRF policy.
	resp, err := c.validatedHTTPClient().Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("embedding: python request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("embedding: python service returned HTTP %d", resp.StatusCode)
	}

	var parsed pythonEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Result{}, fmt.Errorf("embedding: decode python response: %w", err)
	}
	if len(parsed.Embedding) == 0 {
		return Result{}, fmt.Errorf("embedding: python service returned empty vector")
	}
	model := parsed.Model
	if model == "" {
		model = "python-embedding"
	}
	return Result{
		Vector:         parsed.Embedding,
		Provider:       ProviderPython,
		SourceProvider: ProviderPython,
		Model:          model,
		Dimension:      len(parsed.Embedding),
	}, nil
}

func (c *pythonClient) validatedHTTPClient() *http.Client {
	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 750 * time.Millisecond}
	}

	protected := *client
	previousCheckRedirect := client.CheckRedirect
	protected.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := ssrf.ValidateURL(req.URL.String()); err != nil {
			return errPythonEmbeddingRedirectNotAllowed
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		return nil
	}
	return &protected
}
