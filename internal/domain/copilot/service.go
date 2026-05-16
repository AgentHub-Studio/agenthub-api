// Package copilot exposes CopilotKit-aware helpers that don't fit the
// chat-session lifecycle — currently only the lightweight completions
// endpoint used by the Flutter CopilotTextField for ghost-text autocompletion.
//
// Unlike the chat package, this service is stateless: no DB rows, no SSE
// streaming, no agentic loop. It just resolves the tenant's LLM provider via
// the existing ChatModelFactory and runs a single short chat call.
package copilot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonsai "github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// ErrEmptyText is returned when the caller asks for a completion without any
// preceding text. The frontend should not call the endpoint in that state.
var ErrEmptyText = errors.New("copilot: text cannot be empty")

// ErrLLMUnavailable is returned when no provider is configured for the tenant
// and no env-based fallback exists. The client should fall back to disabling
// autocompletion silently.
var ErrLLMUnavailable = errors.New("copilot: no LLM provider available")

// ChatModelFactory mirrors agentic.ChatModelFactory locally to avoid an
// import cycle. The chat domain wires its own implementation; we just need
// `Build` here.
type ChatModelFactory interface {
	Build(ctx context.Context, provider, model string) (commonsai.ChatModel, error)
	ResolveModel(ctx context.Context, provider string) string
	ResolveDefaultProvider(ctx context.Context) string
}

// CompletionContextItem is a single readable forwarded by the client so the
// LLM can ground its suggestion. Matches the wire shape used by
// `POST /api/chat/sessions/{id}/client-state`.
type CompletionContextItem struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
}

// CompletionRequest is the body of `POST /api/copilot/completions`.
type CompletionRequest struct {
	// Text is the full editor content. The service uses [Text] up to
	// [CursorPos] as the prompt; characters after [CursorPos] are ignored.
	Text string `json:"text"`

	// CursorPos is the offset where the caret sits. When <= 0 or beyond the
	// length of Text, len(Text) is used.
	CursorPos int `json:"cursorPos"`

	// Instructions guides the LLM (tone, length, language). Optional —
	// defaults to the built-in autocompletion preset.
	Instructions string `json:"instructions,omitempty"`

	// Context is a list of readables the client wants to expose for grounding
	// (currentRoute, selected entity, etc.). Rendered into the system prompt
	// as `<app_state>` block.
	Context []CompletionContextItem `json:"context,omitempty"`

	// MaxTokens caps the suggestion length. Clamped to 1..128. Default 32.
	MaxTokens int `json:"maxTokens,omitempty"`
}

// CompletionResponse is what the endpoint returns.
type CompletionResponse struct {
	Suggestion string `json:"suggestion"`
}

// Service runs short LLM completion calls for the CopilotTextField.
type Service struct {
	factory ChatModelFactory
}

// NewService creates a Service backed by [factory].
func NewService(factory ChatModelFactory) *Service {
	return &Service{factory: factory}
}

// defaultInstructions is the baseline behavioural prompt. Always appended to
// any caller-supplied [Instructions] so the model returns just the
// continuation text — no quotes, no rewriting, no explanation.
const defaultInstructions = `You are an autocompletion assistant. Given a piece of text the user is typing, ` +
	`output ONLY the continuation that would naturally follow from the cursor position. ` +
	`Rules: ` +
	`(1) Do NOT repeat any of the input text. ` +
	`(2) Do NOT wrap the output in quotes, code fences, or labels. ` +
	`(3) Do NOT explain or rewrite. ` +
	`(4) Stop at the end of a single short sentence, clause, or thought (typically 3–10 words). ` +
	`(5) Match the language and tone of the user's input. ` +
	`(6) Return an empty string when you can't suggest anything useful.`

// Suggest runs the completion request and returns the LLM output trimmed of
// leading/trailing whitespace. Returns [ErrEmptyText] if the prompt has no
// content, or [ErrLLMUnavailable] if the tenant has no provider configured.
func (s *Service) Suggest(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	prompt := promptText(req.Text, req.CursorPos)
	if prompt == "" {
		return CompletionResponse{}, ErrEmptyText
	}

	provider := s.factory.ResolveDefaultProvider(ctx)
	model, err := s.factory.Build(ctx, provider, s.factory.ResolveModel(ctx, provider))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("copilot: build model: %w", err)
	}
	if model == nil {
		return CompletionResponse{}, ErrLLMUnavailable
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 32
	}
	if maxTokens > 128 {
		maxTokens = 128
	}

	systemMsg := BuildSystemPrompt(req.Instructions, req.Context)

	resp, err := model.Chat(ctx, []commonsai.Message{
		{Role: "user", Content: prompt},
	}, commonsai.ChatOptions{
		MaxTokens:   maxTokens,
		Temperature: 0.3,
		SystemMsg:   systemMsg,
	})
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("copilot: chat: %w", err)
	}

	return CompletionResponse{Suggestion: strings.TrimSpace(resp.Content)}, nil
}

// promptText returns the slice of [text] preceding [cursorPos]. Falls back to
// the full text when [cursorPos] is out of range.
func promptText(text string, cursorPos int) string {
	if cursorPos <= 0 || cursorPos > len(text) {
		return text
	}
	return text[:cursorPos]
}

// BuildSystemPrompt composes the system message: the baseline autocompletion
// rules, the caller-supplied [instructions], and an `<app_state>` block built
// from [context]. Exported for testability.
func BuildSystemPrompt(instructions string, context []CompletionContextItem) string {
	var b strings.Builder
	b.WriteString(defaultInstructions)
	trimmed := strings.TrimSpace(instructions)
	if trimmed != "" {
		b.WriteString("\n\n# Additional instructions\n\n")
		b.WriteString(trimmed)
	}
	if len(context) > 0 {
		b.WriteString("\n\n<app_state>\n")
		for _, item := range context {
			b.WriteString(`  <readable id="`)
			b.WriteString(xmlEscape(item.ID))
			b.WriteString(`"`)
			if item.Description != "" {
				b.WriteString(` description="`)
				b.WriteString(xmlEscape(item.Description))
				b.WriteString(`"`)
			}
			b.WriteString(">\n    ")
			if item.Value != "" {
				b.WriteString(item.Value)
			} else {
				b.WriteString("null")
			}
			b.WriteString("\n  </readable>\n")
		}
		b.WriteString("</app_state>")
	}
	return b.String()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, `&amp;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
		`"`, `&quot;`,
		`'`, `&apos;`,
	)
	return r.Replace(s)
}
