// Package ollama provides a ChatModel implementation backed by the Ollama OpenAI-compatible API.
package ollama

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
)

const defaultBaseURL = "http://localhost:11434/v1"

// defaultOllamaTimeout gives the model time to load into memory *and* emit
// its first response headers. Large models (gpt-oss:20b and up) running on
// CPU were observed taking >2 minutes for a short reply on the cluster,
// so the generic openai 120s default caused Client.Timeout to fire before
// the run ever produced a token. 600s matches the practical upper bound
// for cold CPU inference of 20B-parameter models; tune via
// OLLAMA_TIMEOUT_SECONDS at deploy time.
const defaultOllamaTimeout = 600 * time.Second

// Provider implements ai.ChatModel using Ollama's OpenAI-compatible endpoint.
// It delegates all HTTP work to the openai.Provider with an empty API key.
type Provider struct {
	inner *openai.Provider
}

// New creates a new Ollama Provider.
// If baseURL is empty, http://localhost:11434/v1 is used.
// The per-request timeout defaults to 10 minutes, overridable via the
// OLLAMA_TIMEOUT_SECONDS environment variable.
func New(baseURL string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	// Ollama's OpenAI-compatible endpoint requires no API key.
	return &Provider{inner: openai.NewWithTimeout("", baseURL, resolveTimeout())}
}

// resolveTimeout reads OLLAMA_TIMEOUT_SECONDS from the environment and
// falls back to defaultOllamaTimeout when unset or malformed.
func resolveTimeout() time.Duration {
	raw := os.Getenv("OLLAMA_TIMEOUT_SECONDS")
	if raw == "" {
		return defaultOllamaTimeout
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return defaultOllamaTimeout
	}
	return time.Duration(secs) * time.Second
}

// GetProviderName returns "ollama".
func (p *Provider) GetProviderName() string { return "ollama" }

// Chat delegates to the underlying OpenAI-compatible provider.
func (p *Provider) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	resp, err := p.inner.Chat(ctx, messages, opts)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ChatStream delegates to the underlying OpenAI-compatible provider.
func (p *Provider) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	return p.inner.ChatStream(ctx, messages, opts)
}
