// Package openrouter provides a ChatModel implementation backed by the OpenRouter API.
// OpenRouter exposes an OpenAI-compatible endpoint, so this provider delegates to the
// OpenAI provider with the OpenRouter base URL and adds the required extra headers.
package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

// Provider implements ai.ChatModel for the OpenRouter multi-model gateway.
type Provider struct {
	apiKey  string
	baseURL string
	appName string // sent as X-Title header (optional, identifies the caller)
	client  *http.Client
}

// New creates a new OpenRouter Provider.
// appName is optional (sent as X-Title header per OpenRouter docs).
func New(apiKey, baseURL, appName string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		appName: appName,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// GetProviderName returns the provider identifier.
func (p *Provider) GetProviderName() string { return "openrouter" }

// chatRequest mirrors the OpenAI-compatible request body expected by OpenRouter.
type chatRequest struct {
	Model       string       `json:"model"`
	Messages    []ai.Message `json:"messages"`
	Stream      bool         `json:"stream,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
	TopP        float64      `json:"top_p,omitempty"`
	Tools       []ai.Tool    `json:"tools,omitempty"`
}

type chatChoice struct {
	Message      ai.Message `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type usageResponse struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatResponse struct {
	Choices []chatChoice  `json:"choices"`
	Usage   usageResponse `json:"usage"`
	Model   string        `json:"model"`
}

// Chat sends a non-streaming chat request to OpenRouter.
func (p *Provider) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	msgs := messages
	if opts.SystemMsg != "" {
		msgs = append([]ai.Message{{Role: ai.RoleSystem, Content: opts.SystemMsg}}, messages...)
	}

	reqBody := chatRequest{
		Model:       opts.Model,
		Messages:    msgs,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		TopP:        opts.TopP,
		Tools:       opts.Tools,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openrouter: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("openrouter: build request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openrouter: status %d", resp.StatusCode)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("openrouter: decode response: %w", err)
	}

	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("openrouter: empty choices")
	}

	choice := cr.Choices[0]
	return &ai.ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
		Model:        cr.Model,
		Usage: ai.Usage{
			PromptTokens:     cr.Usage.PromptTokens,
			CompletionTokens: cr.Usage.CompletionTokens,
			TotalTokens:      cr.Usage.TotalTokens,
		},
	}, nil
}

// ChatStream sends a streaming chat request to OpenRouter via SSE.
func (p *Provider) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	msgs := messages
	if opts.SystemMsg != "" {
		msgs = append([]ai.Message{{Role: ai.RoleSystem, Content: opts.SystemMsg}}, messages...)
	}

	reqBody := chatRequest{
		Model:       opts.Model,
		Messages:    msgs,
		Stream:      true,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		TopP:        opts.TopP,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openrouter: marshal stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("openrouter: build stream request: %w", err)
	}
	p.setHeaders(req)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req) //nolint:bodyclose // body is closed in the goroutine below
	if err != nil {
		return nil, fmt.Errorf("openrouter: stream request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("openrouter: stream status %d", resp.StatusCode)
	}

	ch := make(chan ai.StreamChunk, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				return
			}

			var event struct {
				Choices []struct {
					Delta        ai.Message `json:"delta"`
					FinishReason string     `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				ch <- ai.StreamChunk{Error: fmt.Errorf("openrouter: decode chunk: %w", err)}
				return
			}
			if len(event.Choices) == 0 {
				continue
			}
			c := event.Choices[0]
			ch <- ai.StreamChunk{
				Delta:        c.Delta.Content,
				FinishReason: c.FinishReason,
			}
		}
	}()

	return ch, nil
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("HTTP-Referer", "https://agenthub.dev")
	if p.appName != "" {
		req.Header.Set("X-Title", p.appName)
	}
}
