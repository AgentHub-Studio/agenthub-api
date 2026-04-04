// Package ollama provides a ChatModel implementation backed by the Ollama OpenAI-compatible API.
package ollama

import (
	"context"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
)

const defaultBaseURL = "http://localhost:11434/v1"

// Provider implements ai.ChatModel using Ollama's OpenAI-compatible endpoint.
// It delegates all HTTP work to the openai.Provider with an empty API key.
type Provider struct {
	inner *openai.Provider
}

// New creates a new Ollama Provider.
// If baseURL is empty, http://localhost:11434/v1 is used.
func New(baseURL string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	// Ollama's OpenAI-compatible endpoint requires no API key.
	return &Provider{inner: openai.New("", baseURL)}
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
