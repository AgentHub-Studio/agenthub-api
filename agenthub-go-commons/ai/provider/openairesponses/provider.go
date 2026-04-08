// Package openairesponses provides a ChatModel implementation backed by the
// OpenAI Responses API (POST /v1/responses). This API is used by newer models
// like gpt-5, gpt-5.4, gpt-5.4-pro that require the Responses endpoint
// instead of the legacy Chat Completions endpoint.
package openairesponses

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Provider implements ai.ChatModel using the OpenAI Responses API.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New creates a new Responses API Provider.
func New(apiKey, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 180 * time.Second},
	}
}

// GetProviderName returns "openai-responses".
func (p *Provider) GetProviderName() string { return "openai-responses" }

// ---- Responses API request types ----

type responsesRequest struct {
	Model              string      `json:"model"`
	Input              interface{} `json:"input"` // string or []inputItem
	Instructions       string      `json:"instructions,omitempty"`
	Tools              []rTool     `json:"tools,omitempty"`
	MaxTokens          int         `json:"max_output_tokens,omitempty"`
	Temperature        float64     `json:"temperature,omitempty"`
	TopP               float64     `json:"top_p,omitempty"`
	Stream             bool        `json:"stream,omitempty"`
	PreviousResponseID string      `json:"previous_response_id,omitempty"`
}

// inputItem represents an item in the Responses API input array.
type inputItem struct {
	Type    string `json:"type"`              // "message", "function_call_output"
	Role    string `json:"role,omitempty"`     // for message items
	Content any    `json:"content,omitempty"`  // string or []contentPart for message items
	CallID  string `json:"call_id,omitempty"`  // for function_call_output items
	Output  string `json:"output,omitempty"`   // for function_call_output items
	ID      string `json:"id,omitempty"`       // for function_call items (assistant tool calls in history)
	Name    string `json:"name,omitempty"`     // for function_call items
	Args    string `json:"arguments,omitempty"` // for function_call items
}

type rTool struct {
	Type        string         `json:"type"` // "function"
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ---- Responses API response types (non-streaming) ----

type responsesResponse struct {
	ID     string       `json:"id"`
	Model  string       `json:"model"`
	Output []outputItem `json:"output"`
	Usage  rUsage       `json:"usage"`
	Status string       `json:"status"` // "completed", "failed", etc.
	Error  *rError      `json:"error,omitempty"`
}

type outputItem struct {
	Type      string          `json:"type"` // "message", "function_call"
	ID        string          `json:"id,omitempty"`
	Role      string          `json:"role,omitempty"`
	Content   []contentPart   `json:"content,omitempty"`
	Name      string          `json:"name,omitempty"`      // function_call
	CallID    string          `json:"call_id,omitempty"`   // function_call
	Arguments string          `json:"arguments,omitempty"` // function_call
	Status    string          `json:"status,omitempty"`
}

type contentPart struct {
	Type string `json:"type"` // "output_text", "refusal"
	Text string `json:"text,omitempty"`
}

type rUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type rError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---- Streaming event types ----

type streamEvent struct {
	Type     string          `json:"type"`
	RawData json.RawMessage `json:"-"` // the full event data
}

type outputTextDelta struct {
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type functionCallArgsDelta struct {
	Delta  string `json:"delta"`
	CallID string `json:"call_id,omitempty"`
	Name   string `json:"name,omitempty"`
}

type outputItemAdded struct {
	Item outputItem `json:"item"`
}

type responseCompleted struct {
	Response responsesResponse `json:"response"`
}

// ---- ChatModel implementation ----

// Chat sends a non-streaming request to the Responses API.
func (p *Provider) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	req := p.buildRequest(messages, opts, false)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai-responses: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai-responses: create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai-responses: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseHTTPError(resp)
	}

	var rResp responsesResponse
	if err := json.NewDecoder(resp.Body).Decode(&rResp); err != nil {
		return nil, fmt.Errorf("openai-responses: decode response: %w", err)
	}

	return p.convertResponse(&rResp), nil
}

// ChatStream sends a streaming request to the Responses API.
func (p *Provider) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	req := p.buildRequest(messages, opts, true)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai-responses: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai-responses: create stream request: %w", err)
	}
	p.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq) //nolint:bodyclose // closed in goroutine
	if err != nil {
		return nil, fmt.Errorf("openai-responses: do stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, p.parseHTTPError(resp)
	}

	ch := make(chan ai.StreamChunk, 32)
	go p.consumeStream(resp, ch)

	return ch, nil
}

func (p *Provider) consumeStream(resp *http.Response, ch chan<- ai.StreamChunk) {
	defer close(ch)
	defer resp.Body.Close()

	// Track current function call being streamed.
	var currentToolCallID string
	var currentToolCallName string
	var toolCallArgsBuffer strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	// Increase scanner buffer for large events.
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var eventType string

	for scanner.Scan() {
		line := scanner.Text()

		// Parse SSE event type.
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		// Parse SSE data.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}

		switch eventType {
		case "response.output_text.delta":
			var delta outputTextDelta
			if err := json.Unmarshal([]byte(data), &delta); err == nil {
				ch <- ai.StreamChunk{Delta: delta.Delta}
			}

		case "response.output_item.added":
			var item outputItemAdded
			if err := json.Unmarshal([]byte(data), &item); err == nil {
				if item.Item.Type == "function_call" {
					currentToolCallID = item.Item.CallID
					currentToolCallName = item.Item.Name
					toolCallArgsBuffer.Reset()
					// Emit the initial tool call delta with name.
					ch <- ai.StreamChunk{
						ToolCallDelta: &ai.ToolCall{
							ID:   currentToolCallID,
							Type: "function",
							Function: ai.ToolFunction{
								Name: currentToolCallName,
							},
						},
					}
				}
			}

		case "response.function_call_arguments.delta":
			var delta functionCallArgsDelta
			if err := json.Unmarshal([]byte(data), &delta); err == nil {
				toolCallArgsBuffer.WriteString(delta.Delta)
				ch <- ai.StreamChunk{
					ToolCallDelta: &ai.ToolCall{
						ID:   currentToolCallID,
						Type: "function",
						Function: ai.ToolFunction{
							Name:      currentToolCallName,
							Arguments: delta.Delta,
						},
					},
				}
			}

		case "response.function_call_arguments.done":
			// Function call arguments complete — reset state for next tool call.
			currentToolCallID = ""
			currentToolCallName = ""
			toolCallArgsBuffer.Reset()

		case "response.output_item.done":
			// Individual output item finished. If it was the last message item,
			// the model might be done or might have more items.

		case "response.completed":
			var completed responseCompleted
			if err := json.Unmarshal([]byte(data), &completed); err == nil {
				r := &completed.Response
				finishReason := "stop"
				// Check if there are function_call items — means tool_calls finish reason.
				for _, item := range r.Output {
					if item.Type == "function_call" {
						finishReason = "tool_calls"
						break
					}
				}
				ch <- ai.StreamChunk{
					FinishReason: finishReason,
					ResponseID:   r.ID,
					Usage: &ai.Usage{
						PromptTokens:     r.Usage.InputTokens,
						CompletionTokens: r.Usage.OutputTokens,
						TotalTokens:      r.Usage.TotalTokens,
					},
				}
			}
			return

		case "response.failed":
			var completed responseCompleted
			if err := json.Unmarshal([]byte(data), &completed); err == nil && completed.Response.Error != nil {
				ch <- ai.StreamChunk{
					Error: fmt.Errorf("openai-responses: %s: %s",
						completed.Response.Error.Code,
						completed.Response.Error.Message),
				}
			} else {
				ch <- ai.StreamChunk{Error: fmt.Errorf("openai-responses: stream failed")}
			}
			return

		case "error":
			var errData rError
			if err := json.Unmarshal([]byte(data), &errData); err == nil {
				ch <- ai.StreamChunk{Error: fmt.Errorf("openai-responses: %s: %s", errData.Code, errData.Message)}
			} else {
				ch <- ai.StreamChunk{Error: fmt.Errorf("openai-responses: unknown error: %s", data)}
			}
			return

		// Events we don't need to handle:
		// response.created, response.in_progress, response.content_part.added,
		// response.content_part.done, response.output_text.done, response.refusal.delta
		default:
			// Ignore unknown events.
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- ai.StreamChunk{Error: fmt.Errorf("openai-responses: read stream: %w", err)}
	}
}

// ---- helpers ----

func (p *Provider) buildRequest(messages []ai.Message, opts ai.ChatOptions, stream bool) responsesRequest {
	req := responsesRequest{
		Model:              opts.Model,
		Temperature:        opts.Temperature,
		TopP:               opts.TopP,
		Stream:             stream,
		PreviousResponseID: opts.PreviousResponseID,
	}

	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}

	// Convert ai.Tool → rTool (Responses API uses flat structure, not nested function).
	for _, t := range opts.Tools {
		req.Tools = append(req.Tools, rTool{
			Type:        "function",
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}

	// Convert ai.Message → Responses API input items.
	// System message becomes top-level "instructions".
	if opts.SystemMsg != "" {
		req.Instructions = opts.SystemMsg
	}

	items := make([]inputItem, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case ai.RoleSystem:
			// System messages after the first are appended as instructions.
			if req.Instructions == "" {
				req.Instructions = m.Content
			} else {
				req.Instructions += "\n\n" + m.Content
			}

		case ai.RoleUser:
			items = append(items, inputItem{
				Type:    "message",
				Role:    "user",
				Content: m.Content,
			})

		case ai.RoleAssistant:
			if len(m.ToolCalls) > 0 {
				// When response chaining is active the server already holds the
				// function_call items from the previous response — re-sending them
				// would cause a 400 "Expected an ID that begins with 'fc'" error
				// because our stored IDs use the call_xxx format returned by the
				// Responses API streaming, not the fc_xxx item-ID format the
				// Responses API expects in input arrays.
				// Skip these items entirely; only function_call_output items
				// (RoleTool) are needed to complete the chained turn.
				if req.PreviousResponseID != "" {
					continue
				}
				// Full-history path (no chaining): emit function_call items.
				if m.Content != "" {
					items = append(items, inputItem{
						Type:    "message",
						Role:    "assistant",
						Content: m.Content,
					})
				}
				for _, tc := range m.ToolCalls {
					items = append(items, inputItem{
						Type:   "function_call",
						ID:     tc.ID,
						Name:   tc.Function.Name,
						Args:   tc.Function.Arguments,
						CallID: tc.ID,
					})
				}
			} else {
				items = append(items, inputItem{
					Type:    "message",
					Role:    "assistant",
					Content: m.Content,
				})
			}

		case ai.RoleTool:
			// Tool result → function_call_output item.
			items = append(items, inputItem{
				Type:   "function_call_output",
				CallID: m.ToolCallID,
				Output: m.Content,
			})
		}
	}

	req.Input = items
	return req
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

func (p *Provider) convertResponse(r *responsesResponse) *ai.ChatResponse {
	resp := &ai.ChatResponse{
		Model:        r.Model,
		ResponseID:   r.ID,
		FinishReason: "stop",
		Usage: ai.Usage{
			PromptTokens:     r.Usage.InputTokens,
			CompletionTokens: r.Usage.OutputTokens,
			TotalTokens:      r.Usage.TotalTokens,
		},
	}

	var textParts []string
	for _, item := range r.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					textParts = append(textParts, part.Text)
				}
			}
		case "function_call":
			resp.FinishReason = "tool_calls"
			resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: ai.ToolFunction{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
		}
	}

	resp.Content = strings.Join(textParts, "")
	return resp
}

func (p *Provider) parseHTTPError(resp *http.Response) error {
	var errBody struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	msg := fmt.Sprintf("status %d", resp.StatusCode)
	if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil && errBody.Error.Message != "" {
		msg = errBody.Error.Message
	}

	sentinel := p.sentinelFor(resp.StatusCode, errBody.Error.Code)
	ae := &ai.APIError{StatusCode: resp.StatusCode, Message: msg, Err: sentinel, RetryAfter: -1}

	if resp.StatusCode == http.StatusTooManyRequests {
		ae.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	}

	return ae
}

func (p *Provider) sentinelFor(statusCode int, code string) error {
	switch statusCode {
	case http.StatusTooManyRequests:
		return ai.ErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return ai.ErrInvalidAPIKey
	case http.StatusBadRequest:
		if code == "context_length_exceeded" {
			return ai.ErrContextLengthExceeded
		}
		if code == "invalid_previous_response_id" || code == "previous_response_not_found" {
			return ai.ErrInvalidResponseID
		}
		return ai.ErrInvalidRequest
	}
	if statusCode >= 500 {
		return ai.ErrProviderUnavailable
	}
	return ai.ErrInvalidRequest
}

func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return -1
	}
	secs, err := strconv.Atoi(s)
	if err != nil || secs < 0 {
		return -1
	}
	return time.Duration(secs) * time.Second
}
