package probe_test

import (
	"bytes"
	"context"
	"encoding/json"
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
)

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

func TestHTTP_EmptyURL_ReturnsHint(t *testing.T) {
	svc := probe.NewService()
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: ""})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "Informe a URL")
}

func TestHTTP_SSRFBlocked(t *testing.T) {
	svc := probe.NewService()
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: "http://127.0.0.1:8080/"})

	assert.False(t, got.OK)
	assert.Contains(t, got.ErrorHint, "URL não permitida")
}

func TestHTTP_UnknownHost_ReturnsFriendlyError(t *testing.T) {
	svc := probe.NewService()
	got := svc.HTTP(context.Background(), probe.HTTPRequest{URL: "http://this-host-should-not-exist.example.invalid/"})

	assert.False(t, got.OK)
	assert.NotEmpty(t, got.ErrorHint)
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

func TestHandler_InvalidJSON_400(t *testing.T) {
	r := newHandlerRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/http/test", bytes.NewReader([]byte("{invalid")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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
