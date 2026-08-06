package pkg

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

func TestHTTPQueryEmbedder_UsesEmbeddingServiceResponse(t *testing.T) {
	_, err := NewHTTPQueryEmbedder("http://127.0.0.1:8092").Embed(context.Background(), "incident recovery")
	require.Error(t, err, "the configured embedding endpoint must not bypass SSRF checks")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/embed", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"embedding":[0.1,0.2],"model":"e5-test","dimension":2}`)
	}))
	defer server.Close()
	ssrf.AllowHost("127.0.0.1")

	result, err := NewHTTPQueryEmbedder(server.URL).Embed(context.Background(), "incident recovery")
	require.NoError(t, err)
	require.Equal(t, "e5-test", result.Model)
	require.Equal(t, []float32{0.1, 0.2}, result.Vector)
}

func TestHTTPQueryEmbedder_RejectsUnsafeEndpoint(t *testing.T) {
	_, err := NewHTTPQueryEmbedder("ftp://embedding.invalid").Embed(context.Background(), "query")
	require.Error(t, err)
}

func TestHTTPQueryEmbedder_BlocksRedirectToPrivateEndpoint(t *testing.T) {
	embedder := NewHTTPQueryEmbedder("https://1.1.1.1")
	requests := 0
	embedder.client = &http.Client{Transport: embedderRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://169.254.169.254/latest/meta-data"}},
				Body:       io.NopCloser(strings.NewReader("redirect")),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"embedding":[0.1],"model":"e5-test","dimension":1}`)),
			Request:    req,
		}, nil
	})}

	_, err := embedder.Embed(context.Background(), "incident recovery")

	require.Error(t, err)
	require.Equal(t, 1, requests, "a redirect to metadata must not make a second request")
}

type embedderRoundTripper func(*http.Request) (*http.Response, error)

func (f embedderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
