package copilot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	commonsai "github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- mocks -------------------------------------------------------------------

type fakeChatModel struct {
	mu              sync.Mutex
	lastSystem      string
	lastMessages    []commonsai.Message
	lastOpts        commonsai.ChatOptions
	response        string
	err             error
	providerName    string
}

func (m *fakeChatModel) Chat(_ context.Context, messages []commonsai.Message, opts commonsai.ChatOptions) (*commonsai.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSystem = opts.SystemMsg
	m.lastMessages = messages
	m.lastOpts = opts
	if m.err != nil {
		return nil, m.err
	}
	return &commonsai.ChatResponse{Content: m.response}, nil
}

func (m *fakeChatModel) ChatStream(_ context.Context, _ []commonsai.Message, _ commonsai.ChatOptions) (<-chan commonsai.StreamChunk, error) {
	return nil, errors.New("ChatStream not supported in test")
}

func (m *fakeChatModel) GetProviderName() string {
	if m.providerName == "" {
		return "mock"
	}
	return m.providerName
}

type fakeFactory struct {
	model       commonsai.ChatModel
	buildErr    error
	provider    string
	modelName   string
}

func (f *fakeFactory) Build(_ context.Context, _, _ string) (commonsai.ChatModel, error) {
	return f.model, f.buildErr
}

func (f *fakeFactory) ResolveModel(_ context.Context, _ string) string {
	return f.modelName
}

func (f *fakeFactory) ResolveDefaultProvider(_ context.Context) string {
	return f.provider
}

// --- promptText --------------------------------------------------------------

func TestPromptText_FullTextWhenCursorOutOfRange(t *testing.T) {
	if got := promptText("hello world", 0); got != "hello world" {
		t.Errorf("cursor=0 → %q", got)
	}
	if got := promptText("hello world", 9999); got != "hello world" {
		t.Errorf("cursor out of range → %q", got)
	}
	if got := promptText("hello world", -5); got != "hello world" {
		t.Errorf("cursor negative → %q", got)
	}
}

func TestPromptText_SlicesAtCursor(t *testing.T) {
	if got := promptText("hello world", 5); got != "hello" {
		t.Errorf("cursor=5 → %q", got)
	}
}

// --- BuildSystemPrompt -------------------------------------------------------

func TestBuildSystemPrompt_DefaultsOnly(t *testing.T) {
	got := BuildSystemPrompt("", nil)
	if !strings.Contains(got, "autocompletion assistant") {
		t.Errorf("missing baseline instructions:\n%s", got)
	}
	if strings.Contains(got, "<app_state>") {
		t.Errorf("unexpected app_state in empty context")
	}
}

func TestBuildSystemPrompt_AppendsUserInstructions(t *testing.T) {
	got := BuildSystemPrompt("Seja conciso em PT-BR.", nil)
	if !strings.Contains(got, "Additional instructions") {
		t.Errorf("missing user instructions section:\n%s", got)
	}
	if !strings.Contains(got, "Seja conciso em PT-BR.") {
		t.Errorf("user text not embedded")
	}
}

func TestBuildSystemPrompt_EscapesXMLInContext(t *testing.T) {
	got := BuildSystemPrompt("", []CompletionContextItem{
		{ID: `it's "x"`, Description: "<x>", Value: "ok"},
	})
	if !strings.Contains(got, `id="it&apos;s &quot;x&quot;"`) {
		t.Errorf("ID not escaped:\n%s", got)
	}
	if !strings.Contains(got, `description="&lt;x&gt;"`) {
		t.Errorf("description not escaped")
	}
}

func TestBuildSystemPrompt_RendersValueOrNull(t *testing.T) {
	got := BuildSystemPrompt("", []CompletionContextItem{
		{ID: "a", Value: "v"},
		{ID: "b"},
	})
	if !strings.Contains(got, "v") {
		t.Errorf("value missing")
	}
	if !strings.Contains(got, "null") {
		t.Errorf("null fallback missing for empty value")
	}
}

// --- Suggest -----------------------------------------------------------------

func TestSuggest_EmptyText_ReturnsErrEmptyText(t *testing.T) {
	svc := NewService(&fakeFactory{model: &fakeChatModel{response: "x"}})
	_, err := svc.Suggest(context.Background(), CompletionRequest{Text: ""})
	if !errors.Is(err, ErrEmptyText) {
		t.Errorf("want ErrEmptyText, got %v", err)
	}
}

func TestSuggest_FactoryReturnsNilModel_IsUnavailable(t *testing.T) {
	svc := NewService(&fakeFactory{model: nil})
	_, err := svc.Suggest(context.Background(), CompletionRequest{Text: "hello"})
	if !errors.Is(err, ErrLLMUnavailable) {
		t.Errorf("want ErrLLMUnavailable, got %v", err)
	}
}

func TestSuggest_HappyPath(t *testing.T) {
	model := &fakeChatModel{response: "world!"}
	svc := NewService(&fakeFactory{model: model})

	got, err := svc.Suggest(context.Background(), CompletionRequest{
		Text:         "hello ",
		Instructions: "Curto e simpático.",
		Context: []CompletionContextItem{
			{ID: "currentRoute", Description: "rota ativa", Value: "/x"},
		},
		MaxTokens: 16,
	})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if got.Suggestion != "world!" {
		t.Errorf("suggestion = %q", got.Suggestion)
	}

	// System prompt deve conter baseline + instruções + app_state.
	if !strings.Contains(model.lastSystem, "autocompletion assistant") {
		t.Errorf("missing baseline in system: %s", model.lastSystem)
	}
	if !strings.Contains(model.lastSystem, "Curto e simpático.") {
		t.Errorf("missing user instructions")
	}
	if !strings.Contains(model.lastSystem, `id="currentRoute"`) {
		t.Errorf("missing readable in system")
	}

	// Opts respeitam max tokens e baixa temperature.
	if model.lastOpts.MaxTokens != 16 {
		t.Errorf("maxTokens = %d, want 16", model.lastOpts.MaxTokens)
	}
	if model.lastOpts.Temperature <= 0 || model.lastOpts.Temperature > 0.5 {
		t.Errorf("temperature = %f, want low", model.lastOpts.Temperature)
	}

	// User message é o text até o cursor.
	if len(model.lastMessages) != 1 || model.lastMessages[0].Role != "user" {
		t.Errorf("unexpected messages: %+v", model.lastMessages)
	}
	if model.lastMessages[0].Content != "hello " {
		t.Errorf("user content = %q", model.lastMessages[0].Content)
	}
}

func TestSuggest_ClampsMaxTokens(t *testing.T) {
	model := &fakeChatModel{response: "ok"}
	svc := NewService(&fakeFactory{model: model})

	// 0 → default 32
	_, _ = svc.Suggest(context.Background(), CompletionRequest{
		Text:      "hi",
		MaxTokens: 0,
	})
	if model.lastOpts.MaxTokens != 32 {
		t.Errorf("zero clamp → %d, want 32", model.lastOpts.MaxTokens)
	}

	// 500 → cap 128
	_, _ = svc.Suggest(context.Background(), CompletionRequest{
		Text:      "hi",
		MaxTokens: 500,
	})
	if model.lastOpts.MaxTokens != 128 {
		t.Errorf("over-cap → %d, want 128", model.lastOpts.MaxTokens)
	}
}

func TestSuggest_TrimsWhitespace(t *testing.T) {
	model := &fakeChatModel{response: "  the world  \n"}
	svc := NewService(&fakeFactory{model: model})

	got, err := svc.Suggest(context.Background(), CompletionRequest{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Suggestion != "the world" {
		t.Errorf("trim failed: %q", got.Suggestion)
	}
}

func TestSuggest_PropagatesLLMError(t *testing.T) {
	model := &fakeChatModel{err: errors.New("api down")}
	svc := NewService(&fakeFactory{model: model})

	_, err := svc.Suggest(context.Background(), CompletionRequest{Text: "hi"})
	if err == nil || !strings.Contains(err.Error(), "api down") {
		t.Errorf("expected wrapped LLM error, got %v", err)
	}
}

func TestSuggest_HonorsCursorPos(t *testing.T) {
	model := &fakeChatModel{response: "ok"}
	svc := NewService(&fakeFactory{model: model})

	_, err := svc.Suggest(context.Background(), CompletionRequest{
		Text:      "hello world",
		CursorPos: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.lastMessages[0].Content != "hello" {
		t.Errorf("user content = %q, want %q", model.lastMessages[0].Content, "hello")
	}
}
