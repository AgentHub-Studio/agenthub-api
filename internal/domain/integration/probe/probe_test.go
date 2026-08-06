package probe_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration/probe"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// --- HTTP probe ---

func TestHTTP_OK_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "hello")
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: upstream.URL})

	assert.True(t, got.OK)
	assert.Equal(t, 200, got.Status)
	assert.Contains(t, got.SampleBody, "hello")
}

func TestHTTP_Unauthorized_401(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: upstream.URL})

	// 4xx is still "reachable" → OK=true, but ErrorHint tells the user.
	assert.True(t, got.OK)
	assert.Equal(t, 401, got.Status)
	assert.Contains(t, got.ErrorHint, "Credencial")
}

func TestHTTP_ServerError_500_MarkedNotOK(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: upstream.URL})

	assert.False(t, got.OK)
	assert.Equal(t, 500, got.Status)
	assert.Contains(t, got.ErrorHint, "erro interno")
}

func TestHTTP_AppliesBearerAuth(t *testing.T) {
	var seenAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	_ = svc.HTTP(context.Background(), probe.HTTPRequest{
		URL:       upstream.URL,
		AuthType:  "bearer",
		AuthToken: "secret-token",
	})

	assert.Equal(t, "Bearer secret-token", seenAuth)
}

func TestHTTP_AppliesBasicAuth(t *testing.T) {
	var seenAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	got := probe.NewService().WithURLValidator(probe.AllowAnyURL).HTTP(context.Background(), probe.HTTPRequest{
		URL:       upstream.URL,
		AuthType:  "basic",
		AuthToken: "dXNlcjpwYXNz",
	})

	assert.True(t, got.OK)
	assert.Equal(t, "Basic dXNlcjpwYXNz", seenAuth)
}

func TestHTTP_RedactsAuthMaterialFromSampleBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"authorization":"`+r.Header.Get("Authorization")+`","apiKey":"`+r.Header.Get("X-API-Key")+`","safe":"visible"}`)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	got := svc.HTTP(context.Background(), probe.HTTPRequest{
		URL:       upstream.URL,
		AuthType:  "bearer",
		AuthToken: "probe-bearer-secret",
		Headers:   json.RawMessage(`{"X-API-Key":"probe-api-key-secret"}`),
	})

	assert.True(t, got.OK)
	assert.NotContains(t, got.SampleBody, "probe-bearer-secret")
	assert.NotContains(t, got.SampleBody, "probe-api-key-secret")
	assert.Contains(t, got.SampleBody, `"authorization":"***"`)
	assert.Contains(t, got.SampleBody, `"apiKey":"***"`)
	assert.Contains(t, got.SampleBody, `"safe":"visible"`)
}

func TestHTTP_AppliesCustomHeaders(t *testing.T) {
	var seenX string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenX = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	_ = svc.HTTP(context.Background(), probe.HTTPRequest{
		URL:     upstream.URL,
		Headers: json.RawMessage(`{"X-Custom":"abc"}`),
	})

	assert.Equal(t, "abc", seenX)
}

func TestHTTP_InvalidHeadersAreIgnored(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("X-Custom"))
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	got := probe.NewService().WithURLValidator(probe.AllowAnyURL).HTTP(context.Background(), probe.HTTPRequest{
		URL:     upstream.URL,
		Headers: json.RawMessage(`{`),
	})

	assert.True(t, got.OK)
}

func TestHTTP_EmptyURL_ReturnsHint(t *testing.T) {
	svc := probe.NewService()
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: ""})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "Informe a URL")
}

func TestHTTP_InvalidRequestURL_ReturnsFriendlyHint(t *testing.T) {
	got := probe.NewService().WithURLValidator(probe.AllowAnyURL).HTTP(context.Background(), probe.HTTPRequest{URL: "http://[::1"})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "Não foi possível montar a requisição")
}

func TestHTTP_SSRFBlocked(t *testing.T) {
	svc := probe.NewService()
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: "http://127.0.0.1:8080/"})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "URL não permitida")
}

func TestHTTP_BlocksRedirectToSSRFURL(t *testing.T) {
	const originURL = "https://public.example.test/integration"
	const blockedURL = "http://localhost/internal"

	var blockedTargetReached bool
	svc := probe.NewService().
		WithURLValidator(func(rawURL string) error {
			if rawURL == originURL {
				return nil
			}
			return ssrf.ValidateURL(rawURL)
		}).
		WithHTTPClient(&http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case originURL:
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{blockedURL}},
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case blockedURL:
					blockedTargetReached = true
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("blocked target")),
					}, nil
				default:
					return nil, errors.New("unexpected request")
				}
			}),
		})

	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: originURL})

	assert.False(t, got.OK)
	assert.False(t, blockedTargetReached)
	assert.Contains(t, got.ErrorHint, "URL não permitida")
}

func TestHTTP_UnknownHost_ReturnsFriendlyError(t *testing.T) {
	svc := probe.NewService()
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: "http://this-host-should-not-exist.example.invalid/"})

	assert.False(t, got.OK)
	assert.NotEmpty(t, got.ErrorHint)
}

func TestHTTP_StatusHints(t *testing.T) {
	tests := []struct {
		name   string
		status int
		hint   string
	}{
		{name: "forbidden", status: http.StatusForbidden, hint: "Acesso negado"},
		{name: "not found", status: http.StatusNotFound, hint: "Endpoint não encontrado"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer upstream.Close()

			got := probe.NewService().WithURLValidator(probe.AllowAnyURL).HTTP(context.Background(), probe.HTTPRequest{URL: upstream.URL})

			assert.True(t, got.OK)
			assert.Equal(t, tt.status, got.Status)
			assert.Contains(t, got.ErrorHint, tt.hint)
		})
	}
}

func TestHTTP_MapsNetworkErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		hint string
	}{
		{name: "unknown host", err: errors.New("no such host"), hint: "Host não encontrado"},
		{name: "connection refused", err: errors.New("connection refused"), hint: "Conexão recusada"},
		{name: "timeout", err: errors.New("i/o timeout"), hint: "Tempo esgotado"},
		{name: "invalid certificate", err: errors.New("x509: certificate signed by unknown authority"), hint: "Certificado TLS inválido"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := probe.NewService().WithURLValidator(probe.AllowAnyURL).WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
					return nil, tt.err
				}),
			})

			got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: "https://example.com"})

			assert.False(t, got.OK)
			assert.Contains(t, got.ErrorHint, tt.hint)
		})
	}
}

func TestHTTP_ReturnsUnmappedNetworkError(t *testing.T) {
	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL).WithHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("unexpected upstream failure")
		}),
	})

	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: "https://example.com"})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "unexpected upstream failure")
}

// --- Database probe ---

func TestDatabase_EmptyHost_Rejected(t *testing.T) {
	svc := probe.NewService()
	got := svc.Database(context.Background(), probe.DatabaseRequest{
		Type: datasource.DataSourceTypePostgreSQL,
		Port: 5432,
	})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "host")
}

func TestDatabase_EmptyPort_Rejected(t *testing.T) {
	svc := probe.NewService()
	got := svc.Database(context.Background(), probe.DatabaseRequest{
		Type: datasource.DataSourceTypePostgreSQL,
		Host: "localhost",
	})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "porta")
}

func TestDatabase_MySQL_NotYetSupported(t *testing.T) {
	svc := probe.NewService()
	got := svc.Database(context.Background(), probe.DatabaseRequest{
		Type: datasource.DataSourceTypeMySQL,
		Host: "mysql.internal", Port: 3306,
		Database: "db", DBUser: "u", DBPassword: "p",
	})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "VPN proxy")
}

func TestDatabase_UnsupportedType(t *testing.T) {
	svc := probe.NewService()
	got := svc.Database(context.Background(), probe.DatabaseRequest{
		Type: "ORACLE",
		Host: "ora", Port: 1521,
	})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "não suportado")
}

// --- MCP probe ---

func TestMCP_SuccessfulHandshake(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.Contains(t, string(body), `"method":"initialize"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	got := svc.MCP(context.Background(), probe.MCPRequest{ServerURL: upstream.URL})

	assert.True(t, got.OK)
	assert.Equal(t, 200, got.Status)
}

func TestMCP_ServerReturnsJSONRPCError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	got := svc.MCP(context.Background(), probe.MCPRequest{ServerURL: upstream.URL})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "Method not found")
}

func TestMCP_RedactsSensitiveHeadersFromSampleAndError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"bad token `+r.Header.Get("Authorization")+`"},"echo":"`+r.Header.Get("Authorization")+`"}`)
	}))
	defer upstream.Close()

	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL)
	got := svc.MCP(context.Background(), probe.MCPRequest{
		ServerURL: upstream.URL,
		Headers:   json.RawMessage(`{"Authorization":"Bearer mcp-probe-secret"}`),
	})

	assert.False(t, got.OK)
	assert.NotContains(t, got.SampleBody, "mcp-probe-secret")
	assert.NotContains(t, got.ErrorHint, "mcp-probe-secret")
	assert.Contains(t, got.SampleBody, `"echo":"***"`)
	assert.Contains(t, got.ErrorHint, "bad token ***")
}

func TestMCP_EmptyURL(t *testing.T) {
	svc := probe.NewService()
	got := svc.MCP(context.Background(), probe.MCPRequest{})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "URL")
}

func TestMCP_SSRFBlocked(t *testing.T) {
	svc := probe.NewService()
	got := svc.MCP(context.Background(), probe.MCPRequest{ServerURL: "http://127.0.0.1:8080/mcp"})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "URL não permitida")
}

func TestMCP_BlocksRedirectToSSRFURL(t *testing.T) {
	const originURL = "https://public.example.test/mcp"
	const blockedURL = "http://localhost/internal"

	var blockedTargetReached bool
	svc := probe.NewService().
		WithURLValidator(func(rawURL string) error {
			if rawURL == originURL {
				return nil
			}
			return ssrf.ValidateURL(rawURL)
		}).
		WithHTTPClient(&http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case originURL:
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{blockedURL}},
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case blockedURL:
					blockedTargetReached = true
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("blocked target")),
					}, nil
				default:
					return nil, errors.New("unexpected request")
				}
			}),
		})

	got := svc.MCP(context.Background(), probe.MCPRequest{ServerURL: originURL})

	assert.False(t, got.OK)
	assert.False(t, blockedTargetReached)
	assert.Contains(t, got.ErrorHint, "URL não permitida")
}

func TestMCP_HTTPStatusFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "missing")
	}))
	defer upstream.Close()

	got := probe.NewService().WithURLValidator(probe.AllowAnyURL).MCP(context.Background(), probe.MCPRequest{ServerURL: upstream.URL})

	assert.False(t, got.OK)
	assert.Equal(t, http.StatusNotFound, got.Status)
	assert.Equal(t, "missing", got.SampleBody)
	assert.Contains(t, got.ErrorHint, "Endpoint não encontrado")
}

func TestMCP_InvalidJSONResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "not-json")
	}))
	defer upstream.Close()

	got := probe.NewService().WithURLValidator(probe.AllowAnyURL).MCP(context.Background(), probe.MCPRequest{ServerURL: upstream.URL})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "não é JSON-RPC válido")
}

func TestMCP_InvalidRequestURL_ReturnsFriendlyHint(t *testing.T) {
	got := probe.NewService().WithURLValidator(probe.AllowAnyURL).MCP(context.Background(), probe.MCPRequest{ServerURL: "http://[::1"})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "Não foi possível montar a requisição MCP")
}

func TestMCP_MapsNetworkFailure(t *testing.T) {
	svc := probe.NewService().WithURLValidator(probe.AllowAnyURL).WithHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		}),
	})

	got := svc.MCP(context.Background(), probe.MCPRequest{ServerURL: "https://example.com/mcp"})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "Conexão recusada")
}

func TestMCP_InvalidHeadersAreIgnored(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("X-Custom"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}))
	defer upstream.Close()

	got := probe.NewService().WithURLValidator(probe.AllowAnyURL).MCP(context.Background(), probe.MCPRequest{
		ServerURL: upstream.URL,
		Headers:   json.RawMessage(`{`),
	})

	assert.True(t, got.OK)
}

// --- Handler wiring ---

func newHandlerRouter() *chi.Mux {
	r := chi.NewRouter()
	probe.NewHandler(probe.NewService().WithURLValidator(probe.AllowAnyURL)).RegisterRoutes(r)
	return r
}

func TestHandler_HTTP_Endpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	r := newHandlerRouter()
	body := strings.NewReader(`{"url":"` + upstream.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/http/test", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got probe.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.True(t, got.OK)
}

func TestHandler_HTTP_RedactsAuthMaterial(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"authorization":"`+r.Header.Get("Authorization")+`"}`)
	}))
	defer upstream.Close()

	r := newHandlerRouter()
	body := strings.NewReader(`{"url":"` + upstream.URL + `","authType":"bearer","authToken":"handler-probe-secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/http/test", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got probe.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.True(t, got.OK)
	assert.NotContains(t, got.SampleBody, "handler-probe-secret")
	assert.Contains(t, got.SampleBody, `"authorization":"***"`)
}

func TestHandler_InvalidJSON_400(t *testing.T) {
	r := newHandlerRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/http/test", bytes.NewReader([]byte("{invalid")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerRejectsTrailingJSONWithoutProbing(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		calls := 0
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusOK)
		}))
		defer upstream.Close()

		r := newHandlerRouter()
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/http/test", strings.NewReader(`{"url":"`+upstream.URL+`"}{"url":"https://ignored.example"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, calls)
	})

	t.Run("database", func(t *testing.T) {
		r := newHandlerRouter()
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/database/test", strings.NewReader(`{"type":"MYSQL","host":"db.example.com","port":3306}{"host":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	})

	t.Run("mcp", func(t *testing.T) {
		calls := 0
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
		}))
		defer upstream.Close()

		r := newHandlerRouter()
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/mcp/test", strings.NewReader(`{"serverUrl":"`+upstream.URL+`"}{"serverUrl":"https://ignored.example"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, calls)
	})
}

func TestHandler_DatabaseEndpoint_Rejects_MissingHost(t *testing.T) {
	r := newHandlerRouter()
	body := strings.NewReader(`{"type":"POSTGRESQL","port":5432}`)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/database/test", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // probe returns 200 with OK=false
	var got probe.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.False(t, got.OK)
}

func TestHandler_DatabaseEndpoint_RejectsInvalidJSON(t *testing.T) {
	r := newHandlerRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/database/test", strings.NewReader("{"))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_MCP_Endpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}))
	defer upstream.Close()

	r := newHandlerRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/mcp/test", strings.NewReader(`{"serverUrl":"`+upstream.URL+`"}`))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got probe.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.True(t, got.OK)
}

func TestHandler_MCP_RejectsInvalidJSON(t *testing.T) {
	r := newHandlerRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/mcp/test", strings.NewReader("{"))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
