package tool

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallLLM_RejectsConfiguredPrivateBaseURLBeforeRequest(t *testing.T) {
	previousClient := llmHTTPClient
	called := false
	llmHTTPClient = &http.Client{Transport: llmRoundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return llmSuccessResponse(), nil
	})}
	t.Cleanup(func() { llmHTTPClient = previousClient })

	_, err := callLLM(context.Background(), llmConfig{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  "http://169.254.169.254/latest",
		Model:    "test-model",
	}, "system", "user")

	require.Error(t, err)
	assert.False(t, called)
}

func TestCallLLM_BlocksRedirectToPrivateURL(t *testing.T) {
	previousClient := llmHTTPClient
	var requestedURLs []string
	llmHTTPClient = &http.Client{Transport: llmRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedURLs = append(requestedURLs, req.URL.String())
		if len(requestedURLs) == 1 {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://127.0.0.1/internal"}},
				Body:       io.NopCloser(strings.NewReader("redirect")),
			}, nil
		}
		return llmSuccessResponse(), nil
	})}
	t.Cleanup(func() { llmHTTPClient = previousClient })

	_, err := callLLM(context.Background(), llmConfig{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  "https://1.1.1.1/v1",
		Model:    "test-model",
	}, "system", "user")

	require.Error(t, err)
	assert.Len(t, requestedURLs, 1)
}

func TestCallLLM_AllowsFixedOllamaDefault(t *testing.T) {
	previousClient := llmHTTPClient
	var requestedURL string
	llmHTTPClient = &http.Client{Transport: llmRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedURL = req.URL.String()
		return llmSuccessResponse(), nil
	})}
	t.Cleanup(func() { llmHTTPClient = previousClient })

	output, err := callLLM(context.Background(), llmConfig{
		Provider: "ollama",
		Model:    "test-model",
	}, "system", "user")

	require.NoError(t, err)
	assert.Equal(t, "ok", output)
	assert.Equal(t, "http://localhost:11434/v1/chat/completions", requestedURL)
}

type llmRoundTripFunc func(*http.Request) (*http.Response, error)

func (f llmRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func llmSuccessResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
	}
}
