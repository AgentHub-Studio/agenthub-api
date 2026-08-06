package agentic_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

type capturingSummaryChatModel struct {
	response     string
	calls        int
	lastOpts     ai.ChatOptions
	lastMessages []ai.Message
}

func (m *capturingSummaryChatModel) Chat(_ context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	m.calls++
	m.lastOpts = opts
	m.lastMessages = append([]ai.Message(nil), messages...)
	return &ai.ChatResponse{Content: m.response, FinishReason: "stop"}, nil
}

func (m *capturingSummaryChatModel) ChatStream(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (m *capturingSummaryChatModel) GetProviderName() string { return "mock" }

type promptTemplateResolverStub struct {
	templates map[string]string
}

func (s *promptTemplateResolverStub) ResolvePromptTemplate(_ context.Context, _ uuid.UUID, slug string) (string, bool, error) {
	content, ok := s.templates[slug]
	return content, ok, nil
}

func TestToolUseSummaryGenerator_Generate(t *testing.T) {
	model := &capturingSummaryChatModel{response: "Searched auth module"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	tools := []agentic.ToolSummaryInfo{
		{
			Name:   "document_search",
			Input:  json.RawMessage(`{"query":"auth middleware"}`),
			Output: json.RawMessage(`{"results":[{"title":"Auth.go"}]}`),
		},
	}

	summary := gen.Generate(context.Background(), uuid.New(), tools, "Let me search for auth info")
	assert.Equal(t, "Searched auth module", summary)
	assert.Equal(t, 1, model.calls)
}

func TestToolUseSummaryGenerator_StripsLeadingDash(t *testing.T) {
	model := &capturingSummaryChatModel{response: "- Read config files"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	tools := []agentic.ToolSummaryInfo{
		{Name: "read_file", Input: json.RawMessage(`{"path":"config.json"}`)},
	}

	summary := gen.Generate(context.Background(), uuid.New(), tools, "")
	assert.Equal(t, "Read config files", summary)
}

func TestToolUseSummaryGenerator_NilModel(t *testing.T) {
	gen := agentic.NewToolUseSummaryGenerator(nil, "test-model")
	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{Name: "test"},
	}, "")
	assert.Empty(t, summary)
}

func TestToolUseSummaryGenerator_NilGenerator(t *testing.T) {
	var gen *agentic.ToolUseSummaryGenerator
	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{Name: "test"},
	}, "")
	assert.Empty(t, summary)
}

func TestToolUseSummaryGenerator_EmptyTools(t *testing.T) {
	model := &capturingSummaryChatModel{response: "something"}
	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	summary := gen.Generate(context.Background(), uuid.New(), nil, "")
	assert.Empty(t, summary)
	assert.Equal(t, 0, model.calls, "should not call model for empty tools")
}

func TestToolUseSummaryGenerator_WithError(t *testing.T) {
	model := &capturingSummaryChatModel{response: "Failed SQL query"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	errMsg := "connection refused"
	tools := []agentic.ToolSummaryInfo{
		{
			Name:  "execute_sql",
			Input: json.RawMessage(`{"query":"SELECT 1"}`),
			Error: &errMsg,
		},
	}

	summary := gen.Generate(context.Background(), uuid.New(), tools, "")
	assert.Equal(t, "Failed SQL query", summary)
}

func TestToolUseSummaryGenerator_RedactsToolDataBeforeModelCall(t *testing.T) {
	model := &capturingSummaryChatModel{response: "Fetched safe metadata"}
	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")
	const inputSecret = "sk-ant-abcdefghijklmnopqrstuvwxyz123456"
	const outputSecret = "output-secret-value"
	const errorSecret = "error-secret-value"
	errMsg := "Authorization: Bearer " + errorSecret + "\nprovider=http://agenthub-provider:8080/v1/chat"

	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{
			Name:   "http_request",
			Input:  json.RawMessage(`{"headers":{"Authorization":"Bearer ` + inputSecret + `"},"request_id":"safe-input"}`),
			Output: json.RawMessage(`{"headers":{"Cookie":"session=` + outputSecret + `"},"request_id":"safe-output"}`),
		},
		{
			Name:  "retry_request",
			Error: &errMsg,
		},
	}, "Assistant saw api_key="+inputSecret)

	assert.Equal(t, "Fetched safe metadata", summary)
	if assert.Len(t, model.lastMessages, 1) {
		prompt := model.lastMessages[0].Content
		assert.NotContains(t, prompt, inputSecret)
		assert.NotContains(t, prompt, outputSecret)
		assert.NotContains(t, prompt, errorSecret)
		assert.NotContains(t, prompt, "http://agenthub-provider:8080/v1/chat")
		assert.Contains(t, prompt, "safe-input")
		assert.Contains(t, prompt, "safe-output")
	}
}

func TestToolUseSummaryGenerator_RedactsGeneratedSummary(t *testing.T) {
	const secret = "sk-ant-abcdefghijklmnopqrstuvwxyz123456"
	model := &capturingSummaryChatModel{response: "Fetched api_key=" + secret + " from http://agenthub-provider:8080/v1/chat"}
	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{Name: "document_search", Input: json.RawMessage(`{"query":"safe"}`)},
	}, "")

	assert.NotContains(t, summary, secret)
	assert.NotContains(t, summary, "http://agenthub-provider:8080/v1/chat")
	assert.Contains(t, summary, "[REDACTED]")
	assert.Contains(t, summary, "<upstream>")
}

func FuzzToolUseSummaryGeneratorRedactsToolDataBeforeModelCall(f *testing.F) {
	f.Add("input", "output", "error", "safe")
	f.Add("Authorization", "Cookie", "X-API-Key", "request-42")

	f.Fuzz(func(t *testing.T, inputSeed, outputSeed, errorSeed, safeSeed string) {
		secret := "summary-secret-" + summarySafeSuffix(errorSeed)
		safeInput := "safe-input-" + summarySafeSuffix(inputSeed)
		safeOutput := "safe-output-" + summarySafeSuffix(outputSeed)
		errMsg := "Authorization: Bearer " + secret + "\nprovider=http://agenthub-provider:8080/v1/chat"
		input, err := json.Marshal(map[string]any{
			"headers":    map[string]string{"Authorization": "Bearer " + secret},
			"request_id": safeInput,
		})
		if err != nil {
			t.Fatalf("marshal input: %v", err)
		}
		output, err := json.Marshal(map[string]any{
			"headers":    map[string]string{"Cookie": "session=" + secret},
			"request_id": safeOutput,
		})
		if err != nil {
			t.Fatalf("marshal output: %v", err)
		}

		model := &capturingSummaryChatModel{response: "safe summary"}
		gen := agentic.NewToolUseSummaryGenerator(model, "test-model")
		gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
			{Name: "request", Input: input, Output: output},
			{Name: "retry", Error: &errMsg},
		}, "")

		if len(model.lastMessages) != 1 {
			t.Fatalf("expected one summary prompt, got %d", len(model.lastMessages))
		}
		prompt := model.lastMessages[0].Content
		if strings.Contains(prompt, secret) {
			t.Fatalf("summary prompt leaked secret: %q", prompt)
		}
		if strings.Contains(prompt, "http://agenthub-provider:8080/v1/chat") {
			t.Fatalf("summary prompt leaked internal topology: %q", prompt)
		}
		if !strings.Contains(prompt, safeInput) || !strings.Contains(prompt, safeOutput) {
			t.Fatalf("summary prompt removed safe markers: %q", prompt)
		}
	})
}

func summarySafeSuffix(seed string) string {
	if seed == "" {
		return "empty"
	}
	var b strings.Builder
	for _, r := range seed {
		if ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			b.WriteRune(r)
		}
		if b.Len() == 24 {
			break
		}
	}
	if b.Len() == 0 {
		return "safe"
	}
	return b.String()
}

func TestToolUseSummaryGenerator_UsesPromptTemplateOverride(t *testing.T) {
	model := &capturingSummaryChatModel{response: "Summarized tool batch"}
	gen := agentic.NewToolUseSummaryGenerator(model, "test-model").
		WithPromptTemplateResolver(&promptTemplateResolverStub{
			templates: map[string]string{
				"agentic-tool-use-summary-system-prompt": "Custom summary prompt",
			},
		})

	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{Name: "document_search", Input: json.RawMessage(`{"query":"billing"}`)},
	}, "")

	assert.Equal(t, "Summarized tool batch", summary)
	assert.Equal(t, "Custom summary prompt", model.lastOpts.SystemMsg)
}

func TestToolUseSummaryData_JSON(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventToolUseSummary, agentic.ToolUseSummaryData{
		TurnIndex: 3,
		Summary:   "Searched docs",
	})
	assert.Equal(t, agentic.EventToolUseSummary, evt.Type)

	var data agentic.ToolUseSummaryData
	assert.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, 3, data.TurnIndex)
	assert.Equal(t, "Searched docs", data.Summary)
}
