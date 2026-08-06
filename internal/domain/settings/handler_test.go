package settings_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

// mockSettingsSvc implements settings.Service for handler tests.
type mockSettingsSvc struct {
	data map[string]settings.SettingResponse
}

func newMockSettingsSvc() *mockSettingsSvc {
	return &mockSettingsSvc{data: make(map[string]settings.SettingResponse)}
}

func (m *mockSettingsSvc) List(_ context.Context) ([]settings.SettingResponse, error) {
	items := make([]settings.SettingResponse, 0, len(m.data))
	for _, v := range m.data {
		items = append(items, v)
	}
	return items, nil
}

func (m *mockSettingsSvc) Get(_ context.Context, key string) (settings.SettingResponse, error) {
	s, ok := m.data[key]
	if !ok {
		return settings.SettingResponse{}, settings.ErrNotFound
	}
	return s, nil
}

func (m *mockSettingsSvc) Upsert(_ context.Context, key string, req settings.UpdateSettingRequest) (settings.SettingResponse, error) {
	s := settings.SettingResponse{Key: key, Value: req.Value}
	m.data[key] = s
	return s, nil
}

func (m *mockSettingsSvc) Delete(_ context.Context, key string) error {
	if _, ok := m.data[key]; !ok {
		return settings.ErrNotFound
	}
	delete(m.data, key)
	return nil
}

func setupSettings() (*chi.Mux, *mockSettingsSvc) {
	return setupSettingsWithRoles("admin")
}

func setupSettingsWithRoles(roles ...string) (*chi.Mux, *mockSettingsSvc) {
	svc := newMockSettingsSvc()
	h := settings.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(middleware.ContextWithRoles(req.Context(), roles...)))
		})
	})
	h.RegisterProtectedRoutes(r)
	return r, svc
}

func TestSettingsHandler_List_Success(t *testing.T) {
	r, svc := setupSettings()
	svc.data["feature.x"] = settings.SettingResponse{Key: "feature.x", Value: json.RawMessage(`true`)}

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []settings.SettingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
}

func TestSettingsHandler_List_Empty(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_Get_Success(t *testing.T) {
	r, svc := setupSettings()
	svc.data["my.key"] = settings.SettingResponse{Key: "my.key", Value: json.RawMessage(`"hello"`)}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/my.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_Get_NotFound(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/missing.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSettingsHandler_Upsert_Success(t *testing.T) {
	r, _ := setupSettings()
	body, _ := json.Marshal(settings.UpdateSettingRequest{Value: json.RawMessage(`42`)})
	req := httptest.NewRequest(http.MethodPut, "/api/settings/my.key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_Upsert_InvalidBody(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodPut, "/api/settings/my.key", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("admin upsert", func(t *testing.T) {
		r, svc := setupSettings()
		req := httptest.NewRequest(
			http.MethodPut,
			"/api/settings/llm.defaultProvider",
			bytes.NewBufferString(`{"value":"openrouter"}{"value":"openai"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.data)
	})

	t.Run("onboarding", func(t *testing.T) {
		r, svc := setupSettingsWithRoles("user")
		req := httptest.NewRequest(
			http.MethodPut,
			"/api/settings/onboarding.completed",
			bytes.NewBufferString(`{"value":true}{"value":false}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.data)
	})
}

func TestSettingsHandler_ListOllamaModels_BlocksSSRFBaseURLBeforeRequest(t *testing.T) {
	called := false
	withProviderHTTPClient(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"models":[{"name":"unexpected"}]}`)),
		}, nil
	})})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/ollama/models?baseUrl="+url.QueryEscape("http://localhost:11434"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.JSONEq(t, `{"status":422,"message":"ollama baseUrl is not allowed"}`, w.Body.String())
	assert.False(t, called)
}

func TestSettingsHandler_ListOllamaModels_UsesFixedDefaultWhenBaseURLOmitted(t *testing.T) {
	var requestedURL string
	withProviderHTTPClient(t, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedURL = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"models":[{"name":"llama"}]}`)),
		}, nil
	})})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/ollama/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:11434/api/tags", requestedURL)
}

func TestSettingsHandler_TestSMTPRequiresAdminRole(t *testing.T) {
	r, _ := setupSettingsWithRoles("user")
	req := httptest.NewRequest(http.MethodPost, "/api/settings/smtp/test?to=operator@example.com", strings.NewReader(`{"host":"smtp.example.com","port":587}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "missing required role")
}

func TestSettingsHandler_TestSMTPAllowsAdminRole(t *testing.T) {
	r, _ := setupSettingsWithRoles("admin")
	req := httptest.NewRequest(http.MethodPost, "/api/settings/smtp/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "to query parameter is required")
}

func TestSettingsHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupSettingsWithRoles("user")
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/settings"},
		{name: "get", method: http.MethodGet, path: "/api/settings/llm.defaultProvider"},
		{name: "upsert", method: http.MethodPut, path: "/api/settings/llm.defaultProvider", body: `{"value":"openai"}`},
		{name: "delete", method: http.MethodDelete, path: "/api/settings/llm.defaultProvider"},
		{name: "providers", method: http.MethodGet, path: "/api/settings/providers"},
		{name: "openai models", method: http.MethodGet, path: "/api/settings/openai/models"},
		{name: "anthropic models", method: http.MethodGet, path: "/api/settings/anthropic/models"},
		{name: "ollama models", method: http.MethodGet, path: "/api/settings/ollama/models"},
		{name: "openrouter models", method: http.MethodGet, path: "/api/settings/openrouter/models"},
		{name: "openrouter embedding models", method: http.MethodGet, path: "/api/settings/openrouter/embedding-models"},
		{name: "provider impact", method: http.MethodGet, path: "/api/settings/provider-impact"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
}

func TestSettingsHandler_CompleteOnboardingAllowsAuthenticatedUser(t *testing.T) {
	r, svc := setupSettingsWithRoles("user")
	req := httptest.NewRequest(http.MethodPut, "/api/settings/onboarding.completed", strings.NewReader(`{"value":true}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `true`, string(svc.data["onboarding.completed"].Value))
}

func TestSettingsHandler_CompleteOnboardingRejectsNonCompletion(t *testing.T) {
	r, svc := setupSettingsWithRoles("user")
	for _, body := range []string{`{"value":false}`, `{"value":"true"}`, `{}`} {
		req := httptest.NewRequest(http.MethodPut, "/api/settings/onboarding.completed", strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Empty(t, svc.data)
	}
}

func TestSettingsHandler_Delete_Success(t *testing.T) {
	r, svc := setupSettings()
	svc.data["del.key"] = settings.SettingResponse{Key: "del.key"}

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/del.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestSettingsHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodDelete, "/api/settings/missing.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// mockImpactAssessor implements impactAssessor for handler tests.
type mockImpactAssessor struct {
	count int64
	err   error
}

func (m *mockImpactAssessor) CountPublishedWithoutProvider(_ context.Context) (int64, error) {
	return m.count, m.err
}

func setupSettingsWithAssessor(assessor settings.ImpactAssessor) (*chi.Mux, *mockSettingsSvc) {
	svc := &mockSettingsSvc{data: make(map[string]settings.SettingResponse)}
	h := settings.NewHandler(svc).WithImpactAssessor(assessor)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(middleware.ContextWithRoles(req.Context(), "admin")))
		})
	})
	h.RegisterProtectedRoutes(r)
	return r, svc
}

// TestSettingsHandler_ProviderImpact_WithAssessor verifies the endpoint returns
// the count from the assessor (ACT-F3-14).
func TestSettingsHandler_ProviderImpact_WithAssessor(t *testing.T) {
	assessor := &mockImpactAssessor{count: 7}
	r, _ := setupSettingsWithAssessor(assessor)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/provider-impact", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp settings.ProviderImpactResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, int64(7), resp.AffectedAgents)
}

// TestSettingsHandler_ProviderImpact_NoAssessor returns zero when no assessor is set.
func TestSettingsHandler_ProviderImpact_NoAssessor(t *testing.T) {
	r, _ := setupSettings()

	req := httptest.NewRequest(http.MethodGet, "/api/settings/provider-impact", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp settings.ProviderImpactResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, int64(0), resp.AffectedAgents)
}

func TestSettingsHandler_ListOpenAIModels_UpstreamErrorReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		http.Error(w, "provider body should stay server-side", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/openai/models", nil)
	req.Header.Set("X-OpenAI-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"openai: upstream unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), "provider body should stay server-side")
}

func TestSettingsHandler_ListOpenAIModels_MalformedUpstreamReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/openai/models", nil)
	req.Header.Set("X-OpenAI-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"openai: upstream unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), `{"data":[`)
}

func TestSettingsHandler_ListOllamaModels_MalformedUpstreamReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/tags", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/ollama/models?baseUrl="+url.QueryEscape("https://1.1.1.1"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"ollama: upstream unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), `{"models":[`)
}

func TestSettingsHandler_ListOpenRouterModels_UpstreamErrorReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/models", r.URL.Path)
		http.Error(w, "provider body should stay server-side", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/openrouter/models", nil)
	req.Header.Set("X-OpenRouter-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"openrouter: upstream unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), "provider body should stay server-side")
}

func TestSettingsHandler_ListOpenRouterModels_MalformedUpstreamReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/openrouter/models", nil)
	req.Header.Set("X-OpenRouter-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"openrouter: upstream unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), `{"data":[`)
}

func TestSettingsHandler_ListProviderModels_NetworkErrorReturnsBadGateway(t *testing.T) {
	cases := []struct {
		name   string
		route  string
		header string
		want   string
	}{
		{
			name:   "openai",
			route:  "/api/settings/openai/models",
			header: "X-OpenAI-API-Key",
			want:   `{"error":"openai: upstream unavailable"}`,
		},
		{
			name:  "ollama",
			route: "/api/settings/ollama/models?baseUrl=http://ollama.invalid",
			want:  `{"error":"ollama: upstream unavailable"}`,
		},
		{
			name:   "openrouter",
			route:  "/api/settings/openrouter/models",
			header: "X-OpenRouter-API-Key",
			want:   `{"error":"openrouter: upstream unavailable"}`,
		},
		{
			name:   "openrouter_embedding",
			route:  "/api/settings/openrouter/embedding-models",
			header: "X-OpenRouter-API-Key",
			want:   `{"error":"openrouter: upstream unavailable"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withProviderHTTPClient(t, failingProviderClient(errors.New("provider network unavailable")))

			r, _ := setupSettings()
			req := httptest.NewRequest(http.MethodGet, tc.route, nil)
			if tc.header != "" {
				req.Header.Set(tc.header, "test-key")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadGateway, w.Code)
			assert.JSONEq(t, tc.want, w.Body.String())
			assert.NotContains(t, w.Body.String(), "provider network unavailable")
		})
	}
}

func TestSettingsHandler_ListProviderModels_RateLimitReturnsBadGateway(t *testing.T) {
	cases := []struct {
		name            string
		upstreamPath    string
		route           func(string) string
		header          string
		configureClient func(t *testing.T, ts *httptest.Server)
		want            string
	}{
		{
			name:         "openai",
			upstreamPath: "/v1/models",
			route: func(_ string) string {
				return "/api/settings/openai/models"
			},
			header: "X-OpenAI-API-Key",
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})
			},
			want: `{"error":"openai: upstream unavailable"}`,
		},
		{
			name:         "ollama",
			upstreamPath: "/api/tags",
			route: func(_ string) string {
				return "/api/settings/ollama/models?baseUrl=" + url.QueryEscape("https://1.1.1.1")
			},
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})
			},
			want: `{"error":"ollama: upstream unavailable"}`,
		},
		{
			name:         "openrouter",
			upstreamPath: "/api/v1/models",
			route: func(_ string) string {
				return "/api/settings/openrouter/models"
			},
			header: "X-OpenRouter-API-Key",
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})
			},
			want: `{"error":"openrouter: upstream unavailable"}`,
		},
		{
			name:         "openrouter_embedding",
			upstreamPath: "/api/v1/models",
			route: func(_ string) string {
				return "/api/settings/openrouter/embedding-models"
			},
			header: "X-OpenRouter-API-Key",
			configureClient: func(t *testing.T, ts *httptest.Server) {
				withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})
			},
			want: `{"error":"openrouter: upstream unavailable"}`,
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

			r, _ := setupSettings()
			req := httptest.NewRequest(http.MethodGet, tc.route(ts.URL), nil)
			if tc.header != "" {
				req.Header.Set(tc.header, "test-key")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadGateway, w.Code)
			assert.JSONEq(t, tc.want, w.Body.String())
			assert.NotContains(t, w.Body.String(), "rate limit body should stay server-side")
		})
	}
}

func TestSettingsHandler_ListOpenRouterEmbeddingModels_UpstreamErrorReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/models", r.URL.Path)
		http.Error(w, "provider body should stay server-side", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/openrouter/embedding-models", nil)
	req.Header.Set("X-OpenRouter-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"openrouter: upstream unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), "provider body should stay server-side")
}

func TestSettingsHandler_ListOpenRouterEmbeddingModels_MalformedUpstreamReturnsBadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[`))
	}))
	defer ts.Close()

	withProviderHTTPClient(t, &http.Client{Transport: &redirectTransport{baseURL: ts.URL}})

	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/openrouter/embedding-models", nil)
	req.Header.Set("X-OpenRouter-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"openrouter: upstream unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), `{"data":[`)
}
