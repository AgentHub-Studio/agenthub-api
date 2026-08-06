package embedding

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_PythonProviderUsesExternalServiceWhenDefaultIsPython(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/embed", r.URL.Path)
		_, _ = fmt.Fprint(w, `{"embedding":[0.1,0.2,0.3],"model":"intfloat/multilingual-e5-large","dimension":3}`)
	}))
	t.Cleanup(srv.Close)

	svc := NewService(Config{
		DefaultProvider: ProviderPython,
		PythonURL:       srv.URL,
		PythonTimeout:   time.Second,
	})

	result, err := svc.Embed(context.Background(), "hello world", ProviderPython)
	require.NoError(t, err)
	require.Equal(t, ProviderPython, result.Provider)
	require.Equal(t, ProviderPython, result.SourceProvider)
	require.Equal(t, "intfloat/multilingual-e5-large", result.Model)
	require.Equal(t, []float32{0.1, 0.2, 0.3}, result.Vector)
	require.Equal(t, 3, result.Dimension)
}

func TestPythonClient_BlocksRedirectToPrivateEndpoint(t *testing.T) {
	client := newPythonClient("https://1.1.1.1", time.Second)
	requests := 0
	client.httpClient = &http.Client{Transport: pythonEmbedderRoundTripper(func(req *http.Request) (*http.Response, error) {
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
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"embedding":[0.1],"model":"test","dimension":1}`)),
			Request:    req,
		}, nil
	})}

	_, err := client.Embed(context.Background(), "hello")

	require.Error(t, err)
	require.Equal(t, 1, requests, "a redirect to metadata must not make a second request")
}

type pythonEmbedderRoundTripper func(*http.Request) (*http.Response, error)

func (f pythonEmbedderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
