package settings_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
)

// redirectTransport redirects all requests to the given base URL (used to intercept hardcoded URLs in tests).
type redirectTransport struct {
	baseURL string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, _ := url.Parse(t.baseURL + req.URL.Path)
	u.RawQuery = req.URL.RawQuery
	req2 := req.Clone(req.Context())
	req2.URL = u
	return http.DefaultTransport.RoundTrip(req2)
}

func withProviderHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	settings.SetProviderHTTPClient(client)
	t.Cleanup(func() {
		settings.SetProviderHTTPClient(nil)
	})
}

func failingProviderClient(err error) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, err
		}),
	}
}

func TestListProviders_Returns4Providers(t *testing.T) {
	providers := settings.ListProviders()
	assert.Len(t, providers, 4)
	ids := make([]string, len(providers))
	for i, p := range providers {
		ids[i] = p.ID
	}
	assert.Contains(t, ids, "openai")
	assert.Contains(t, ids, "anthropic")
	assert.Contains(t, ids, "ollama")
	assert.Contains(t, ids, "openrouter")
}

func TestListAnthropicModels_ReturnsHardcoded(t *testing.T) {
	models, err := settings.ListAnthropicModels(context.Background(), "any-key")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
	// Claude 4.x models must be present
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	assert.Contains(t, ids, "claude-sonnet-4-6")
}

func TestListOpenAIModels_CallsAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "gpt-4o"},
				{"id": "gpt-3.5-turbo"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	t.Setenv("OPENAI_BASE_URL", "https://models.example.test/v1")
	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	models, err := settings.ListOpenAIModels(context.Background(), "test-key")
	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, "gpt-4o", models[0].ID)
}

func TestListOllamaModels_CallsLocalEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama3.2"},
				{"name": "mistral"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Inject test server URL as baseURL
	withProviderHTTPClient(t, ts.Client())

	models, err := settings.ListOllamaModels(context.Background(), ts.URL)
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "llama3.2", models[0].ID)
}

func TestListOllamaModels_PreservesEscapedBasePath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ollama root/api/tags", r.URL.Path)
		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama3.2"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	withProviderHTTPClient(t, ts.Client())

	models, err := settings.ListOllamaModels(context.Background(), ts.URL+"/ollama%20root")
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "llama3.2", models[0].ID)
}

func TestListOllamaModels_RejectsUnsafeBaseURL(t *testing.T) {
	cases := []string{
		"ftp://ollama.local",
		"http://ollama.local?debug=true",
		"http://ollama.local/#fragment",
		"http://user@ollama.local",
		"http:///missing-host",
		"http://ollama.local/\napi",
	}

	for _, baseURL := range cases {
		t.Run(baseURL, func(t *testing.T) {
			called := false
			withProviderHTTPClient(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				called = true
				return nil, errors.New("unexpected request")
			})})

			_, err := settings.ListOllamaModels(context.Background(), baseURL)

			require.Error(t, err)
			assert.False(t, called)
		})
	}
}

func TestListOllamaModels_BlocksRedirectToSSRFURL(t *testing.T) {
	const originURL = "https://1.1.1.1"
	const blockedURL = "http://localhost/api/tags"

	var blockedTargetReached bool
	withProviderHTTPClient(t, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case originURL + "/api/tags":
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{blockedURL}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		case blockedURL:
			blockedTargetReached = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"models":[{"name":"unexpected"}]}`)),
			}, nil
		default:
			return nil, errors.New("unexpected request")
		}
	})})

	_, err := settings.ListOllamaModels(context.Background(), originURL)

	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream))
	assert.False(t, blockedTargetReached)
}

func TestListOpenRouterModels_CallsAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/models", r.URL.Path)
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "openai/gpt-4o", "name": "GPT-4o"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	t.Setenv("OPENROUTER_BASE_URL", "https://models.example.test/api/v1")
	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	models, err := settings.ListOpenRouterModels(context.Background(), "test-key")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
}

func TestListProviderModels_RejectsUnsafeConfiguredBaseURL(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "http://127.0.0.1:11434/v1")

	_, err := settings.ListOpenAIModels(context.Background(), "test-key")

	require.Error(t, err)
	assert.ErrorIs(t, err, settings.ErrUpstream)
}

func TestListOpenAIModels_UpstreamStatusWrapsErrUpstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		http.Error(w, "temporary provider outage", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	_, err := settings.ListOpenAIModels(context.Background(), "test-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
}

func TestListProviderModels_NetworkErrorWrapsErrUpstream(t *testing.T) {
	providerErr := errors.New("provider network unavailable")
	cases := []struct {
		name string
		call func(context.Context) error
	}{
		{
			name: "openai",
			call: func(ctx context.Context) error {
				_, err := settings.ListOpenAIModels(ctx, "test-key")
				return err
			},
		},
		{
			name: "ollama",
			call: func(ctx context.Context) error {
				_, err := settings.ListOllamaModels(ctx, "http://ollama.invalid")
				return err
			},
		},
		{
			name: "openrouter",
			call: func(ctx context.Context) error {
				_, err := settings.ListOpenRouterModels(ctx, "test-key")
				return err
			},
		},
		{
			name: "openrouter_embedding",
			call: func(ctx context.Context) error {
				_, err := settings.ListOpenRouterEmbeddingModels(ctx, "test-key")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withProviderHTTPClient(t, failingProviderClient(providerErr))

			err := tc.call(context.Background())

			require.Error(t, err)
			assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
		})
	}
}

func TestListProviderModels_RateLimitWrapsErrUpstream(t *testing.T) {
	cases := []struct {
		name            string
		upstreamPath    string
		configureClient func(t *testing.T, ts *httptest.Server)
		call            func(context.Context, *httptest.Server) error
	}{
		{
			name:         "openai",
			upstreamPath: "/v1/models",
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})
			},
			call: func(ctx context.Context, _ *httptest.Server) error {
				_, err := settings.ListOpenAIModels(ctx, "test-key")
				return err
			},
		},
		{
			name:         "ollama",
			upstreamPath: "/api/tags",
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, ts.Client())
			},
			call: func(ctx context.Context, ts *httptest.Server) error {
				_, err := settings.ListOllamaModels(ctx, ts.URL)
				return err
			},
		},
		{
			name:         "openrouter",
			upstreamPath: "/api/v1/models",
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})
			},
			call: func(ctx context.Context, _ *httptest.Server) error {
				_, err := settings.ListOpenRouterModels(ctx, "test-key")
				return err
			},
		},
		{
			name:         "openrouter_embedding",
			upstreamPath: "/api/v1/models",
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})
			},
			call: func(ctx context.Context, _ *httptest.Server) error {
				_, err := settings.ListOpenRouterEmbeddingModels(ctx, "test-key")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.upstreamPath, r.URL.Path)
				http.Error(w, "rate limit body should stay server-side", http.StatusTooManyRequests)
			}))
			t.Cleanup(ts.Close)
			tc.configureClient(t, ts)

			err := tc.call(context.Background(), ts)

			require.Error(t, err)
			assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
			assert.Contains(t, err.Error(), "status 429")
		})
	}
}

func TestListOpenAIModels_MalformedResponseWrapsErrUpstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	_, err := settings.ListOpenAIModels(context.Background(), "test-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
}

func TestListOllamaModels_MalformedResponseWrapsErrUpstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, ts.Client())

	_, err := settings.ListOllamaModels(context.Background(), ts.URL)
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
}

func TestListOpenRouterModels_UpstreamStatusWrapsErrUpstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/models", r.URL.Path)
		http.Error(w, "temporary provider outage", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	_, err := settings.ListOpenRouterModels(context.Background(), "test-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
}

func TestListOpenRouterModels_MalformedResponseWrapsErrUpstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	_, err := settings.ListOpenRouterModels(context.Background(), "test-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
}

func TestListOpenRouterEmbeddingModels_UpstreamStatusWrapsErrUpstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/models", r.URL.Path)
		http.Error(w, "temporary provider outage", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	_, err := settings.ListOpenRouterEmbeddingModels(context.Background(), "test-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
}

func TestListOpenRouterEmbeddingModels_MalformedResponseWrapsErrUpstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	_, err := settings.ListOpenRouterEmbeddingModels(context.Background(), "test-key")
	require.Error(t, err)
	assert.True(t, errors.Is(err, settings.ErrUpstream), "expected ErrUpstream, got %v", err)
}
