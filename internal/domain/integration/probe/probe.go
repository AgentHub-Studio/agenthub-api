// Package probe implements lightweight connection tests for integrations
// (HTTP APIs, databases, MCP servers). Used by the admin wizard so the
// user sees ok/error feedback before saving the integration.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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

var errRedirectURLNotAllowed = errors.New("redirect URL is not allowed")

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

// validatedHTTPClient revalidates every redirect before another request is sent.
func (s *Service) validatedHTTPClient() *http.Client {
	client := *s.httpClient
	previousCheckRedirect := s.httpClient.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := s.validateURL(req.URL.String()); err != nil {
			return errRedirectURLNotAllowed
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		return nil
	}
	return &client
}

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
	redactor := newProbeSecretRedactor(req.AuthType, req.AuthToken, req.Headers)

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

	resp, err := s.validatedHTTPClient().Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if errors.Is(err, errRedirectURLNotAllowed) {
			return fail(start, "URL não permitida.")
		}
		return Result{OK: false, LatencyMs: latency, ErrorHint: redactor.redact(mapHTTPError(err))}
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, sampleBodyMax))
	sample := redactor.redact(string(body))

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

	conn, err := pgx.Connect(connCtx, postgresProbeDSN(req))
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

func postgresProbeDSN(req DatabaseRequest) string {
	values := url.Values{}
	values.Set("sslmode", "prefer")
	values.Set("connect_timeout", "5")

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(req.DBUser, req.DBPassword),
		Host:     net.JoinHostPort(req.Host, strconv.Itoa(req.Port)),
		Path:     "/" + req.Database,
		RawPath:  "/" + url.PathEscape(req.Database),
		RawQuery: values.Encode(),
	}
	return dsn.String()
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
	redactor := newProbeSecretRedactor("", "", req.Headers)
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

	resp, err := s.validatedHTTPClient().Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if errors.Is(err, errRedirectURLNotAllowed) {
			return fail(start, "URL não permitida.")
		}
		return Result{OK: false, LatencyMs: latency, ErrorHint: redactor.redact(mapHTTPError(err))}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, sampleBodyMax))
	sample := redactor.redact(string(respBody))

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
		return Result{OK: false, Status: resp.StatusCode, LatencyMs: latency, SampleBody: sample, ErrorHint: "MCP retornou erro: " + redactor.redact(rpc.Error.Message)}
	}
	return Result{OK: true, Status: resp.StatusCode, LatencyMs: latency, SampleBody: sample}
}

// --- helpers ---

type probeSecretRedactor struct {
	values []string
}

func newProbeSecretRedactor(authType, authToken string, headers json.RawMessage) probeSecretRedactor {
	var redactor probeSecretRedactor
	token := strings.TrimSpace(authToken)
	if token != "" {
		switch strings.ToLower(strings.TrimSpace(authType)) {
		case "bearer":
			redactor.add("Bearer " + token)
		case "basic":
			redactor.add("Basic " + token)
		}
		redactor.add(token)
	}

	if len(headers) > 0 {
		var hm map[string]string
		if err := json.Unmarshal(headers, &hm); err == nil {
			for name, value := range hm {
				if !isSensitiveProbeHeader(name) {
					continue
				}
				redactor.add(value)
				if _, token, ok := strings.Cut(strings.TrimSpace(value), " "); ok {
					redactor.add(token)
				}
			}
		}
	}

	sort.Slice(redactor.values, func(i, j int) bool {
		return len(redactor.values[i]) > len(redactor.values[j])
	})
	return redactor
}

func (r *probeSecretRedactor) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, existing := range r.values {
		if existing == value {
			return
		}
	}
	r.values = append(r.values, value)
}

func (r probeSecretRedactor) redact(text string) string {
	if text == "" {
		return text
	}
	for _, value := range r.values {
		text = strings.ReplaceAll(text, value, "***")
	}
	return text
}

func isSensitiveProbeHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization", "x-api-key", "x-api-token", "x-auth-token", "x-access-token", "x-secret", "cookie", "set-cookie":
		return true
	default:
		return false
	}
}

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
