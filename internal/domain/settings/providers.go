package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

// providerHTTPClient is the shared HTTP client for provider model queries.
var providerHTTPClient = &http.Client{Timeout: 15 * time.Second}

var errProviderRedirectURLNotAllowed = errors.New("provider redirect URL is not allowed")

const (
	defaultOpenAIModelsBaseURL     = "https://api.openai.com/v1"
	defaultOpenRouterModelsBaseURL = "https://openrouter.ai/api/v1"
)

// ModelInfo represents a single model available from a provider.
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// OpenRouterModelInfo represents a model from the OpenRouter API with pricing details.
type OpenRouterModelInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Modality        string `json:"modality,omitempty"`
	ContextLength   int64  `json:"contextLength,omitempty"`
	PromptPrice     string `json:"promptPrice,omitempty"`
	CompletionPrice string `json:"completionPrice,omitempty"`
}

// ProviderInfo describes an available AI provider.
type ProviderInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// availableProviders is the static list of supported AI providers.
var availableProviders = []ProviderInfo{
	{ID: "openai", Name: "OpenAI", Description: "GPT-4, GPT-3.5 and DALL-E models"},
	{ID: "anthropic", Name: "Anthropic", Description: "Claude 3.5 and Claude 4 models"},
	{ID: "ollama", Name: "Ollama", Description: "Local open-source models via Ollama"},
	{ID: "openrouter", Name: "OpenRouter", Description: "Multi-model gateway with 100+ models"},
}

// ListProviders returns the supported AI providers.
func ListProviders() []ProviderInfo {
	return availableProviders
}

// ListOpenAIModels fetches available GPT models using the provided API key.
func ListOpenAIModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	endpoint, err := configuredProviderModelsURL("OPENAI_BASE_URL", defaultOpenAIModelsBaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: openai: invalid models base URL", ErrUpstream)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := protectedProviderHTTPClient().Do(req)
	if err != nil {
		if errors.Is(err, errProviderRedirectURLNotAllowed) {
			return nil, fmt.Errorf("%w: openai: URL is not allowed", ErrUpstream)
		}
		return nil, fmt.Errorf("%w: openai: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: openai: status %d: %s", ErrUpstream, resp.StatusCode, body)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: openai: decode: %v", ErrUpstream, err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID})
	}
	return models, nil
}

// ListAnthropicModels returns the hardcoded list of Claude models
// (Anthropic doesn't have a public /models endpoint).
func ListAnthropicModels(_ context.Context, _ string) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "claude-opus-4-6", Name: "Claude Opus 4.6"},
		{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5"},
		{ID: "claude-opus-4-5", Name: "Claude Opus 4.5"},
		{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5"},
	}, nil
}

// ListOllamaModels fetches available models from a local Ollama instance.
func ListOllamaModels(ctx context.Context, baseURL string) ([]ModelInfo, error) {
	endpoint, err := buildOllamaTagsURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("ollama: invalid base URL: %w", err)
	}

	// #nosec G704 -- endpoint is built from a validated HTTP(S) base URL with fixed escaped path segments.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	// #nosec G704 -- request URL is produced by buildOllamaTagsURL above.
	resp, err := protectedProviderHTTPClient().Do(req)
	if err != nil {
		if errors.Is(err, errProviderRedirectURLNotAllowed) {
			return nil, fmt.Errorf("%w: ollama: URL is not allowed", ErrUpstream)
		}
		return nil, fmt.Errorf("%w: ollama: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: ollama: status %d: %s", ErrUpstream, resp.StatusCode, body)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: ollama: decode: %v", ErrUpstream, err)
	}

	models := make([]ModelInfo, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, ModelInfo{ID: m.Name, Name: m.Name})
	}
	return models, nil
}

func buildOllamaTagsURL(rawBase string) (string, error) {
	if strings.TrimSpace(rawBase) == "" {
		rawBase = "http://localhost:11434"
	}
	base, err := parseProviderHTTPBaseURL(rawBase)
	if err != nil {
		return "", err
	}
	if err := appendEscapedProviderPathSegments(base, "api", "tags"); err != nil {
		return "", err
	}
	return base.String(), nil
}

func parseProviderHTTPBaseURL(rawBase string) (*url.URL, error) {
	rawBase = strings.TrimSpace(rawBase)
	if rawBase == "" {
		return nil, fmt.Errorf("empty base URL")
	}
	if containsProviderURLControlChar(rawBase) {
		return nil, fmt.Errorf("base URL contains control characters")
	}
	parsed, err := url.Parse(rawBase)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("userinfo is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("query and fragment are not allowed")
	}
	return parsed, nil
}

func appendEscapedProviderPathSegments(base *url.URL, segments ...string) error {
	escapedPath := strings.TrimRight(base.EscapedPath(), "/")
	for _, segment := range segments {
		if segment == "" || containsProviderURLControlChar(segment) {
			return fmt.Errorf("invalid path segment")
		}
		escapedPath += "/" + url.PathEscape(segment)
	}
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return err
	}
	base.Path = decodedPath
	base.RawPath = escapedPath
	return nil
}

func containsProviderURLControlChar(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0
}

// ListOpenRouterModels fetches available models from OpenRouter with pricing details.
func ListOpenRouterModels(ctx context.Context, apiKey string) ([]OpenRouterModelInfo, error) {
	endpoint, err := configuredProviderModelsURL("OPENROUTER_BASE_URL", defaultOpenRouterModelsBaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: openrouter: invalid models base URL", ErrUpstream)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := protectedProviderHTTPClient().Do(req)
	if err != nil {
		if errors.Is(err, errProviderRedirectURLNotAllowed) {
			return nil, fmt.Errorf("%w: openrouter: URL is not allowed", ErrUpstream)
		}
		return nil, fmt.Errorf("%w: openrouter: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: openrouter: status %d: %s", ErrUpstream, resp.StatusCode, body)
	}

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			ContextLen  int64  `json:"context_length"`
			Pricing     struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			Architecture struct {
				Modality string `json:"modality"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: openrouter: decode: %v", ErrUpstream, err)
	}

	models := make([]OpenRouterModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, OpenRouterModelInfo{
			ID:              m.ID,
			Name:            m.Name,
			Description:     m.Description,
			Modality:        m.Architecture.Modality,
			ContextLength:   m.ContextLen,
			PromptPrice:     m.Pricing.Prompt,
			CompletionPrice: m.Pricing.Completion,
		})
	}
	return models, nil
}

// ListOpenRouterEmbeddingModels returns OpenRouter models whose modality includes "text".
// These are suitable for embedding/text-processing use cases.
func ListOpenRouterEmbeddingModels(ctx context.Context, apiKey string) ([]OpenRouterModelInfo, error) {
	all, err := ListOpenRouterModels(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	var embedding []OpenRouterModelInfo
	for _, m := range all {
		if strings.Contains(m.Modality, "text") && strings.Contains(strings.ToLower(m.ID+m.Name), "embed") {
			embedding = append(embedding, m)
		}
	}
	if embedding == nil {
		embedding = []OpenRouterModelInfo{}
	}
	return embedding, nil
}

func configuredProviderModelsURL(envKey, fallback string) (string, error) {
	baseURL := strings.TrimSpace(os.Getenv(envKey))
	if baseURL == "" {
		baseURL = fallback
	}
	if err := ssrf.ValidateURL(baseURL); err != nil {
		return "", err
	}
	base, err := parseProviderHTTPBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	if err := appendEscapedProviderPathSegments(base, "models"); err != nil {
		return "", err
	}
	return base.String(), nil
}

// SmtpTestRequest holds the SMTP configuration for a test connection.
type SmtpTestRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	UseTLS   bool   `json:"useTls"`
}

// TestSMTPConnection attempts to open a TCP connection to the SMTP server.
// A full SMTP handshake is not performed — this validates host/port reachability.
func TestSMTPConnection(ctx context.Context, req SmtpTestRequest) error {
	if req.Host == "" {
		return fmt.Errorf("smtp: host is required")
	}
	port := req.Port
	if port == 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", req.Host, port)

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp: cannot connect to %s: %w", addr, err)
	}
	return conn.Close()
}

// SetProviderHTTPClient replaces the shared HTTP client used by provider functions.
// Pass nil to restore the default. Used in tests.
func SetProviderHTTPClient(c *http.Client) {
	if c == nil {
		providerHTTPClient = &http.Client{Timeout: 15 * time.Second}
	} else {
		providerHTTPClient = c
	}
}

func protectedProviderHTTPClient() *http.Client {
	client := providerHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	protected := *client
	previousCheckRedirect := client.CheckRedirect
	protected.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := ssrf.ValidateURL(req.URL.String()); err != nil {
			return errProviderRedirectURLNotAllowed
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		return nil
	}
	return &protected
}
