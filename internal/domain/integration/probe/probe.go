// Package probe implements lightweight connection tests for integrations
// (HTTP APIs, databases, MCP servers). Used by the admin wizard so the
// user sees ok/error feedback before saving the integration.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

const (
	probeTimeout   = 10 * time.Second
	sampleBodyMax  = 512
	errorHintLimit = 240
)

// HTTPRequest is the body of POST /api/integrations/http/test.
type HTTPRequest struct {
	URL       string          `json:"url"`
	Method    string          `json:"method,omitempty"`
	Headers   json.RawMessage `json:"headers,omitempty"`
	AuthType  string          `json:"authType,omitempty"`
	AuthToken string          `json:"authToken,omitempty"`
}

// DatabaseRequest is the body of POST /api/integrations/database/test.
type DatabaseRequest struct {
	Type       datasource.DataSourceType `json:"type"`
	Host       string                    `json:"host"`
	Port       int                       `json:"port"`
	Database   string                    `json:"database"`
	DBUser     string                    `json:"dbUser"`
	DBPassword string                    `json:"dbPassword"`
}

// MCPRequest is the body of POST /api/integrations/mcp/test.
type MCPRequest struct {
	ServerURL string          `json:"serverUrl"`
	Headers   json.RawMessage `json:"headers,omitempty"`
}

// Result is the response payload. Success is OK=true with Status/LatencyMs set;
// failure is OK=false with ErrorHint set to a PT-BR, user-friendly message.
type Result struct {
	OK         bool   `json:"ok"`
	Status     int    `json:"status,omitempty"`
	LatencyMs  int64  `json:"latencyMs"`
	SampleBody string `json:"sampleBody,omitempty"`
	ErrorHint  string `json:"errorHint,omitempty"`
}

// URLValidator checks whether an outbound URL is allowed. Production uses
// ssrf.ValidateURL; tests can swap in a no-op.
type URLValidator func(string) error

// Service encapsulates probe operations. Stateless, safe to share across requests.
type Service struct {
	httpClient  *http.Client
	validateURL URLValidator
}

// NewService returns a probe Service with default timeouts and SSRF validation.
func NewService() *Service {
	return &Service{
		httpClient:  &http.Client{Timeout: probeTimeout},
		validateURL: ssrf.ValidateURL,
	}
}

// WithHTTPClient overrides the HTTP client used for HTTP/MCP probes.
// Useful in tests to avoid real network calls.
func (s *Service) WithHTTPClient(c *http.Client) *Service {
	s.httpClient = c
	return s
}

// WithURLValidator overrides the SSRF validator. Use probe.AllowAnyURL in
// tests that hit a local httptest.Server.
func (s *Service) WithURLValidator(v URLValidator) *Service {
	s.validateURL = v
	return s
}

// AllowAnyURL is a URLValidator that permits every URL. Tests only.
func AllowAnyURL(string) error { return nil }

// HTTP performs a GET (or specified method) against the URL and reports the outcome.
// SSRF-validates the URL before firing the request.
func (s *Service) HTTP(ctx context.Context, req HTTPRequest) Result {
	start := time.Now()
	if strings.TrimSpace(req.URL) == "" {
		return fail(start, "Informe a URL do serviço.")
	}
	if err := s.validateURL(req.URL); err != nil {
		return fail(start, "URL não permitida: "+truncate(err.Error(), errorHintLimit))
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, nil)
	if err != nil {
		return fail(start, "Não foi possível montar a requisição: "+truncate(err.Error(), errorHintLimit))
	}

	// Apply arbitrary headers.
	if len(req.Headers) > 0 {
		var hm map[string]string
		if err := json.Unmarshal(req.Headers, &hm); err == nil {
			for k, v := range hm {
				httpReq.Header.Set(k, v)
			}
		}
	}

	// Apply auth.
	switch strings.ToLower(strings.TrimSpace(req.AuthType)) {
	case "bearer":
		if req.AuthToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+req.AuthToken)
		}
	case "basic":
		if req.AuthToken != "" {
			httpReq.Header.Set("Authorization", "Basic "+req.AuthToken)
		}
	}

	resp, err := s.httpClient.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{OK: false, LatencyMs: latency, ErrorHint: mapHTTPError(err)}
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, sampleBodyMax))
	sample := string(body)

	// 2xx = green. 3xx/4xx = reachable but with client error — surface as ok=true
	// plus status so the UI can color-code. 5xx = server error, still reachable.
	return Result{
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 500,
		Status:     resp.StatusCode,
		LatencyMs:  latency,
		SampleBody: sample,
		ErrorHint:  hintForStatus(resp.StatusCode),
	}
}

// Database opens a connection, runs SELECT 1, and reports the outcome.
// Only PostgreSQL is implemented in-process; MySQL/SQL Server probes
// currently return an unsupported-hint until the vpn-proxy integration lands.
func (s *Service) Database(ctx context.Context, req DatabaseRequest) Result {
	start := time.Now()
	if strings.TrimSpace(req.Host) == "" {
		return fail(start, "Informe o host do banco.")
	}
	if req.Port == 0 {
		return fail(start, "Informe a porta do banco.")
	}

	switch req.Type {
	case datasource.DataSourceTypePostgreSQL:
		return s.probePostgres(ctx, req, start)
	case datasource.DataSourceTypeMySQL, datasource.DataSourceTypeSQLServer:
		return fail(start, "Teste de "+string(req.Type)+" ainda requer roteamento via VPN proxy. Salve a conexão e use o assistente para validá-la.")
	default:
		return fail(start, "Tipo de banco não suportado: "+string(req.Type))
	}
}

func (s *Service) probePostgres(ctx context.Context, req DatabaseRequest, start time.Time) Result {
	connCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=prefer&connect_timeout=5",
		req.DBUser, req.DBPassword, req.Host, req.Port, req.Database)
	conn, err := pgx.Connect(connCtx, dsn)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{OK: false, LatencyMs: latency, ErrorHint: mapPostgresError(err)}
	}
	defer func() { _ = conn.Close(connCtx) }()

	if err := conn.Ping(connCtx); err != nil {
		return Result{OK: false, LatencyMs: time.Since(start).Milliseconds(), ErrorHint: mapPostgresError(err)}
	}
	return Result{OK: true, LatencyMs: time.Since(start).Milliseconds()}
}

// MCP sends an MCP initialize JSON-RPC message to the server URL and
// reports whether the handshake succeeded.
func (s *Service) MCP(ctx context.Context, req MCPRequest) Result {
	start := time.Now()
	if strings.TrimSpace(req.ServerURL) == "" {
		return fail(start, "Informe a URL do servidor MCP.")
	}
	if err := s.validateURL(req.ServerURL); err != nil {
		return fail(start, "URL não permitida: "+truncate(err.Error(), errorHintLimit))
	}

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "agenthub-probe",
				"version": "1.0.0",
			},
		},
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.ServerURL, bytes.NewReader(body))
	if err != nil {
		return fail(start, "Não foi possível montar a requisição MCP: "+truncate(err.Error(), errorHintLimit))
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	if len(req.Headers) > 0 {
		var hm map[string]string
		if err := json.Unmarshal(req.Headers, &hm); err == nil {
			for k, v := range hm {
				httpReq.Header.Set(k, v)
			}
		}
	}

	resp, err := s.httpClient.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{OK: false, LatencyMs: latency, ErrorHint: mapHTTPError(err)}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, sampleBodyMax))
	sample := string(respBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{
			OK:         false,
			Status:     resp.StatusCode,
			LatencyMs:  latency,
			SampleBody: sample,
			ErrorHint:  hintForStatus(resp.StatusCode),
		}
	}

	// Parse the JSON-RPC response to confirm initialize was handled.
	var rpc struct {
		Result *json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpc); err != nil {
		return Result{OK: false, Status: resp.StatusCode, LatencyMs: latency, SampleBody: sample, ErrorHint: "Resposta MCP não é JSON-RPC válido."}
	}
	if rpc.Error != nil {
		return Result{OK: false, Status: resp.StatusCode, LatencyMs: latency, SampleBody: sample, ErrorHint: "MCP retornou erro: " + rpc.Error.Message}
	}
	return Result{OK: true, Status: resp.StatusCode, LatencyMs: latency, SampleBody: sample}
}

// --- helpers ---

func fail(start time.Time, hint string) Result {
	return Result{OK: false, LatencyMs: time.Since(start).Milliseconds(), ErrorHint: hint}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func hintForStatus(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return "Credencial inválida ou ausente."
	case code == http.StatusForbidden:
		return "Acesso negado — verifique escopos/permissões."
	case code == http.StatusNotFound:
		return "Endpoint não encontrado — confira a URL."
	case code >= 500:
		return "Serviço respondeu com erro interno — tente novamente."
	}
	return ""
}

func mapHTTPError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such host"):
		return "Host não encontrado — verifique o endereço ou DNS."
	case strings.Contains(msg, "connection refused"):
		return "Conexão recusada — o serviço está ativo? A porta está correta?"
	case strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "context deadline exceeded"):
		return "Tempo esgotado — o serviço pode estar fora do ar ou atrás de firewall/VPN."
	case strings.Contains(msg, "x509"):
		return "Certificado TLS inválido."
	}
	return truncate(msg, errorHintLimit)
}

func mapPostgresError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "password authentication failed"):
		return "Usuário ou senha inválidos."
	case strings.Contains(msg, "does not exist"):
		return "Banco ou schema não encontrado."
	case strings.Contains(msg, "no such host"):
		return "Host não encontrado — verifique o endereço ou DNS."
	case strings.Contains(msg, "connection refused"):
		return "Conexão recusada — o banco está ativo? A porta está correta?"
	case strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "context deadline exceeded"):
		return "Tempo esgotado — o banco pode estar fora do ar ou atrás de VPN."
	case errors.Is(err, context.DeadlineExceeded):
		return "Tempo esgotado."
	}
	return truncate(msg, errorHintLimit)
}
