package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/ollama"
)

func TestOllamaProvider_GetProviderName(t *testing.T) {
	p := ollama.New("")
	assert.Equal(t, "ollama", p.GetProviderName())
}

func TestOllamaProvider_Chat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message":       map[string]any{"role": "assistant", "content": "Ola!"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := ollama.New(srv.URL)
	res, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Oi"}},
		ai.ChatOptions{Model: "llama3.2"})
	require.NoError(t, err)
	assert.Equal(t, "Ola!", res.Content)
	assert.Equal(t, 8, res.Usage.TotalTokens)
}

func TestOllamaProvider_Chat_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := ollama.New(srv.URL)
	_, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "llama3.2"})
	require.Error(t, err)
}

func TestOllamaProvider_ChatStream_Tokens(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"Ola\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" mundo\"},\"finish_reason\":null}]}\n\n" +
		"data: [DONE]\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	p := ollama.New(srv.URL)
	ch, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Oi"}},
		ai.ChatOptions{Model: "llama3.2"})
	require.NoError(t, err)

	var got string
	for chunk := range ch {
		require.NoError(t, chunk.Error)
		got += chunk.Delta
	}
	assert.Equal(t, "Ola mundo", got)
}

func TestOllamaProvider_DefaultBaseURL(t *testing.T) {
	// Verify New("") doesn't panic and returns a valid provider.
	p := ollama.New("")
	assert.NotNil(t, p)
	assert.Equal(t, "ollama", p.GetProviderName())
}
