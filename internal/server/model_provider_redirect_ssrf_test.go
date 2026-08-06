package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/anthropic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openairesponses"
)

func TestModelProviders_BlockRedirectsToPrivateEndpoint(t *testing.T) {
	cases := []struct {
		name          string
		newModel      func(baseURL string) ai.ChatModel
		targetPayload string
	}{
		{
			name:          "openai chat completions",
			newModel:      func(baseURL string) ai.ChatModel { return openai.New("test-key", baseURL) },
			targetPayload: `{"id":"chatcmpl_1","model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"redirected"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
		},
		{
			name:          "anthropic messages",
			newModel:      func(baseURL string) ai.ChatModel { return anthropic.New("test-key", baseURL) },
			targetPayload: `{"id":"msg_01","model":"test-model","stop_reason":"end_turn","content":[{"type":"text","text":"redirected"}],"usage":{"input_tokens":1,"output_tokens":1}}`,
		},
		{
			name:          "openai responses",
			newModel:      func(baseURL string) ai.ChatModel { return openairesponses.New("test-key", baseURL) },
			targetPayload: `{"id":"resp_01","model":"test-model","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"redirected"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var targetRequests atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				targetRequests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.targetPayload))
			}))
			defer target.Close()

			redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/private-target", http.StatusFound)
			}))
			defer redirector.Close()

			model := tc.newModel(redirector.URL)
			messages := []ai.Message{{Role: ai.RoleUser, Content: "hello"}}
			opts := ai.ChatOptions{Model: "test-model", MaxTokens: 16}

			_, err := model.Chat(
				context.Background(),
				messages,
				opts,
			)

			require.Error(t, err, "redirect to a private endpoint must be blocked for chat")
			assert.Zero(t, targetRequests.Load(), "chat must not request the redirect target")

			_, err = model.ChatStream(context.Background(), messages, opts)

			require.Error(t, err, "redirect to a private endpoint must be blocked for streaming")
			assert.Zero(t, targetRequests.Load(), "streaming must not request the redirect target")
		})
	}
}
