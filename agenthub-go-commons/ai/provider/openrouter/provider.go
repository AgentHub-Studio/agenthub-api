// Package openrouter provides a ChatModel implementation backed by the OpenRouter API.
// OpenRouter exposes an OpenAI-compatible endpoint, so this provider wraps the OpenAI
// provider with the OpenRouter base URL and adds the required extra headers.
package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

// Provider implements ai.ChatModel for the OpenRouter multi-model gateway.
// It wraps the OpenAI provider so tool calls and tool results use the exact
// OpenAI-compatible wire fields expected by OpenRouter.
type Provider struct {
	inner *openai.Provider
}

// New creates a new OpenRouter Provider.
// appName is optional (sent as X-Title header per OpenRouter docs).
func New(apiKey, baseURL, appName string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	base := &http.Transport{
		ResponseHeaderTimeout: 600 * time.Second,
	}
	inner := openai.New(apiKey, strings.TrimRight(baseURL, "/"))
	inner.WithTransport(&orTransport{base: base, appName: appName})
	return &Provider{inner: inner}
}

// GetProviderName returns the provider identifier.
func (p *Provider) GetProviderName() string { return "openrouter" }

// Chat sends a non-streaming chat request via the OpenAI-compatible OpenRouter API.
func (p *Provider) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	resp, err := p.inner.Chat(ctx, messages, opts)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}
	return resp, nil
}

// ChatStream sends a streaming chat request via the OpenAI-compatible OpenRouter API.
func (p *Provider) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	ch, err := p.inner.ChatStream(ctx, messages, opts)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}
	return ch, nil
}

// orTransport adds OpenRouter-specific headers to OpenAI-compatible requests.
type orTransport struct {
	base    http.RoundTripper
	appName string
}

func (t *orTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("HTTP-Referer", "https://agenthub.dev")
	if t.appName != "" {
		req.Header.Set("X-Title", t.appName)
	}
	return t.base.RoundTrip(req)
}
