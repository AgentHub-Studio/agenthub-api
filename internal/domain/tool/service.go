package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// SettingsReader is a minimal interface for reading tenant settings.
type SettingsReader interface {
	FindSettingByKey(ctx context.Context, key string) ([]byte, error)
}

// DatasourceReader is a minimal interface for reading datasource credentials.
type DatasourceReader interface {
	GetDatasourceCreds(ctx context.Context, tenantID string, id uuid.UUID) (DatasourceCreds, error)
}

// KBExister verifies a knowledge-base exists in the tenant (bug 231).
type KBExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

type skillInstructionReferenceLister interface {
	ListSkillInstructionReferencesByTool(ctx context.Context, toolID uuid.UUID) ([]SkillInstructionReference, error)
}

// DatasourceCreds holds the connection parameters for a datasource.
type DatasourceCreds struct {
	Type     string
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

// Service holds business logic for tools.
type Service struct {
	repo             ToolRepository
	settingsRdr      SettingsReader
	dsRdr            DatasourceReader
	sqlTestExecutor  SQLTestExecutor
	kbRdr            KBExister
	tenantIDFn       func(ctx context.Context) string
	httpURLValidator func(string) error
	httpBackendBase  string
}

// NewService creates a new Service.
func NewService(repo ToolRepository) *Service {
	return &Service{
		repo:            repo,
		httpBackendBase: normalizeHTTPBaseURL(os.Getenv("BACKEND_BASE_URL")),
	}
}

// WithSettings attaches a settings reader for LLM generation features.
func (s *Service) WithSettings(r SettingsReader) *Service {
	s.settingsRdr = r
	return s
}

// WithDatasource attaches a datasource reader for schema introspection and SQL test.
func (s *Service) WithDatasource(r DatasourceReader, tenantIDFn func(ctx context.Context) string) *Service {
	s.dsRdr = r
	s.tenantIDFn = tenantIDFn
	return s
}

// WithSQLTestExecutor replaces the SQL test executor. It is intended for
// isolated tests; production uses the PostgreSQL executor by default.
func (s *Service) WithSQLTestExecutor(executor SQLTestExecutor) *Service {
	s.sqlTestExecutor = executor
	return s
}

// WithKBExister attaches a KB existence checker (bug 231).
func (s *Service) WithKBExister(r KBExister) *Service {
	s.kbRdr = r
	return s
}

// WithHTTPURLValidator overrides runtime HTTP URL validation. It is intended
// for local unit tests that use httptest servers; production uses ssrf.ValidateURL.
func (s *Service) WithHTTPURLValidator(fn func(string) error) *Service {
	s.httpURLValidator = fn
	return s
}

// WithHTTPBackendBaseURL sets the trusted backend base URL used to resolve
// relative HTTP tool URLs, matching the skill-runtime BACKEND_BASE_URL behavior.
func (s *Service) WithHTTPBackendBaseURL(baseURL string) *Service {
	s.httpBackendBase = normalizeHTTPBaseURL(baseURL)
	return s
}

// List returns a paginated list of tools.
func (s *Service) List(ctx context.Context, req pagination.PageRequest, toolType string) (pagination.Page[Response], error) {
	tools, total, err := s.repo.List(ctx, req, toolType)
	if err != nil {
		return pagination.Page[Response]{}, err
	}
	content := make([]Response, len(tools))
	for i, t := range tools {
		content[i] = ResponseFrom(t)
	}
	return pagination.NewPage(content, total, req), nil
}

// ListLabels returns all distinct labels used across tools for the tenant.
func (s *Service) ListLabels(ctx context.Context) ([]string, error) {
	return s.repo.ListLabels(ctx)
}

// Create creates a new tool.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Response, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return Response{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if sanitize.ContainsHTML(req.Name) {
		return Response{}, fmt.Errorf("%w: name must not contain HTML tags", ErrValidation)
	}
	// Bug 129: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return Response{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	if !IsValidToolType(req.Type) {
		return Response{}, fmt.Errorf("%w: unsupported type: %s", ErrValidation, req.Type)
	}
	// P-C220-1 / P-C221-1 / P-C254-1: HTTP tools require a non-empty URL.
	if req.Type == ToolTypeHTTP {
		u := extractURLFromConfig(req.Config)
		if u == "" {
			return Response{}, fmt.Errorf("%w: url is required for HTTP tools", ErrValidation)
		}
		if err := s.validateHTTPToolURL(httpURLForValidationFromConfig(req.Config, u, s.httpBackendBase)); err != nil {
			return Response{}, fmt.Errorf("%w: invalid URL (%v)", ErrValidation, err)
		}
		if err := validateHTTPURLTemplates(req.Config); err != nil {
			return Response{}, err
		}
		if err := validateHTTPMethodAndTimeout(req.Config); err != nil {
			return Response{}, err
		}
	}
	// P-C249-1: normalize SQL tool config — accept both datasourceId and datasource_id.
	config := req.Config
	if req.Type == ToolTypeHTTP {
		config = normalizeHTTPMethodConfig(config)
	}
	if isSQLToolType(req.Type) {
		if err := validateSQLDataSourceAliasContract(config); err != nil {
			return Response{}, err
		}
		config = normalizeDataSourceID(config)
		var cfgMap map[string]any
		_ = json.Unmarshal(config, &cfgMap)
		q, _ := cfgMap["query"].(string)
		if strings.TrimSpace(q) == "" {
			return Response{}, fmt.Errorf("%w: query is required for SQL tools", ErrValidation)
		}
		dsID, _ := cfgMap["datasource_id"].(string)
		if strings.TrimSpace(dsID) == "" {
			if v, _ := cfgMap["datasourceId"].(string); strings.TrimSpace(v) != "" {
				dsID = v
			}
		}
		if strings.TrimSpace(dsID) == "" {
			return Response{}, fmt.Errorf("%w: datasourceId is required for SQL tools", ErrValidation)
		}
		dsUUID, err := uuid.Parse(strings.TrimSpace(dsID))
		if err != nil {
			return Response{}, fmt.Errorf("%w: datasourceId must be a valid UUID (got %q)", ErrValidation, dsID)
		}
		// Bug 233: validar existência do datasource no tenant (mesmo padrão
		// do bug 231 com KB). Antes, qualquer UUID passava — SQL tool ficava
		// silenciosamente quebrada (executor falhava com FK ou not found
		// quando o LLM tentava chamar).
		if s.dsRdr != nil && s.tenantIDFn != nil {
			tenantID := s.tenantIDFn(ctx)
			if _, err := s.dsRdr.GetDatasourceCreds(ctx, tenantID, dsUUID); err != nil {
				return Response{}, fmt.Errorf("%w: datasourceId not found", ErrValidation)
			}
		}
	}
	// Bug 147 + 231: DOCUMENT_SEARCH tool requer kbId — UUID válido E
	// existente. Antes da bug 231, qualquer UUID passava (mesmo bogus
	// ou zero), criando tools quebradas silenciosamente.
	if req.Type == ToolTypeDocumentSearch || req.Type == ToolTypeDocuments {
		var cfgMap map[string]any
		_ = json.Unmarshal(req.Config, &cfgMap)
		kbID, _ := cfgMap["kbId"].(string)
		if strings.TrimSpace(kbID) == "" {
			if v, _ := cfgMap["knowledgeBaseId"].(string); strings.TrimSpace(v) != "" {
				kbID = v
			}
		}
		if strings.TrimSpace(kbID) == "" {
			return Response{}, fmt.Errorf("%w: kbId is required for DOCUMENT_SEARCH tools", ErrValidation)
		}
		kbUUID, err := uuid.Parse(strings.TrimSpace(kbID))
		if err != nil {
			return Response{}, fmt.Errorf("%w: kbId must be a valid UUID (got %q)", ErrValidation, kbID)
		}
		// Bug 231: verificar existência da KB no tenant atual.
		if s.kbRdr != nil {
			if err := s.kbRdr.GetByID(ctx, kbUUID); err != nil {
				return Response{}, fmt.Errorf("%w: kbId not found", ErrValidation)
			}
		}
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = ToSlug(req.Name)
	} else if !sanitize.ValidSlug(slug) {
		return Response{}, fmt.Errorf("%w: slug must match %s (got %q)", ErrValidation, sanitize.CanonicalSlugPattern, slug)
	}
	// Bug 159: cap description em 32KB.
	if len(req.Description) > 32000 {
		return Response{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	}
	// Bug 179: strip HTML do description (XSS prevention cross-cutting).
	req.Description = sanitize.StripHTML(req.Description)
	// Bug 166: cap config + inputSchema em 64KB cada (JSONB raw bytes).
	// Config real (HTTP/SQL/DOCUMENT_SEARCH) cabe em <2KB; inputSchema
	// JSON Schema cabe em <16KB; 500KB+ é storage waste e perf hit.
	if len(req.Config) > 64*1024 {
		return Response{}, fmt.Errorf("%w: config exceeds maximum size of 65536 bytes (got %d)", ErrValidation, len(req.Config))
	}
	if len(req.InputSchema) > 64*1024 {
		return Response{}, fmt.Errorf("%w: inputSchema exceeds maximum size of 65536 bytes (got %d)", ErrValidation, len(req.InputSchema))
	}
	t := Tool{
		Name:        req.Name,
		Slug:        slug,
		Type:        req.Type,
		Config:      config,
		InputSchema: req.InputSchema,
		Description: req.Description,
		Labels:      req.Labels,
	}
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return Response{}, err
	}
	// Auto-bind to skill if skillId was provided in the request.
	if req.SkillID != nil && *req.SkillID != uuid.Nil {
		active := true
		if _, bindErr := s.repo.BindToSkill(ctx, *req.SkillID, BindRequest{
			ToolID:   created.ID,
			Priority: 0,
			IsActive: &active,
		}); bindErr != nil {
			// Non-fatal: tool was created; log and continue.
			_ = bindErr
		}
	}
	return ResponseFrom(created), nil
}

// GetByID returns a tool by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Response, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(t), nil
}

// Update applies a partial update to a tool, preserving fields that are not
// included in the request (P-C196-1: PATCH must not overwrite unset fields).
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}

	// Merge only the fields that were explicitly included in the request.
	if req.Name != nil {
		// Bug 136: name varchar(255) — gate length em Update.
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return Response{}, fmt.Errorf("%w: name is required", ErrValidation)
		}
		if len(trimmed) > 255 {
			return Response{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(trimmed))
		}
		if sanitize.ContainsHTML(trimmed) {
			return Response{}, fmt.Errorf("%w: name must not contain HTML tags", ErrValidation)
		}
		existing.Name = trimmed
	}
	if req.Slug != nil {
		trimmed := strings.TrimSpace(*req.Slug)
		if trimmed != "" {
			// Bug 121: Update precisa do mesmo gate que Create —
			// Sem isso admin podia
			// salvar slug="INVALID!" via PATCH e quebrar lookups.
			if !sanitize.ValidSlug(trimmed) {
				return Response{}, fmt.Errorf("%w: slug must match %s (got %q)", ErrValidation, sanitize.CanonicalSlugPattern, trimmed)
			}
			existing.Slug = trimmed
		}
	}
	if req.Type != nil {
		// Bug 119c: type aceita só HTTP/SQL/DATABASE/DOCUMENT_SEARCH/
		// CUSTOM. Sem este gate admin podia salvar type="INVALID"
		// e o tool executor não saberia que executor montar.
		if !IsValidToolType(*req.Type) {
			return Response{}, fmt.Errorf("%w: unsupported type: %s", ErrValidation, *req.Type)
		}
		existing.Type = *req.Type
	}
	if len(req.Config) > 0 {
		// Bug 166: cap em Update também (cross-cutting com Create).
		if len(req.Config) > 64*1024 {
			return Response{}, fmt.Errorf("%w: config exceeds maximum size of 65536 bytes (got %d)", ErrValidation, len(req.Config))
		}
		existing.Config = req.Config
	}
	if len(req.InputSchema) > 0 {
		if len(req.InputSchema) > 64*1024 {
			return Response{}, fmt.Errorf("%w: inputSchema exceeds maximum size of 65536 bytes (got %d)", ErrValidation, len(req.InputSchema))
		}
		existing.InputSchema = req.InputSchema
	}
	if req.Description != nil {
		// Bug 174: cap em Update (cross-cutting com Create — bug 159).
		if len(*req.Description) > 32000 {
			return Response{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(*req.Description))
		}
		// Bug 179: strip HTML do description (XSS prevention).
		existing.Description = sanitize.StripHTML(*req.Description)
	}
	if req.Labels != nil {
		existing.Labels = req.Labels
	}
	if req.ReadOnly != nil {
		existing.ReadOnly = *req.ReadOnly
	}

	// P-C249-1: normalize SQL tool config on update as well.
	if isSQLToolType(existing.Type) {
		if err := validateSQLDataSourceAliasContract(existing.Config); err != nil {
			return Response{}, err
		}
		existing.Config = normalizeDataSourceID(existing.Config)
		var cfgMap map[string]any
		_ = json.Unmarshal(existing.Config, &cfgMap)
		dsID, _ := cfgMap["datasource_id"].(string)
		if strings.TrimSpace(dsID) == "" {
			if v, _ := cfgMap["datasourceId"].(string); strings.TrimSpace(v) != "" {
				dsID = v
			}
		}
		if strings.TrimSpace(dsID) != "" {
			dsUUID, err := uuid.Parse(strings.TrimSpace(dsID))
			if err != nil {
				return Response{}, fmt.Errorf("%w: datasourceId must be a valid UUID (got %q)", ErrValidation, dsID)
			}
			// Bug 233: gate UPDATE também valida existência.
			if s.dsRdr != nil && s.tenantIDFn != nil {
				tenantID := s.tenantIDFn(ctx)
				if _, err := s.dsRdr.GetDatasourceCreds(ctx, tenantID, dsUUID); err != nil {
					return Response{}, fmt.Errorf("%w: datasourceId not found", ErrValidation)
				}
			}
		}
	}

	// Bug 148 + 231: DOCUMENT_SEARCH UPDATE com kbId existência também.
	if existing.Type == ToolTypeDocumentSearch || existing.Type == ToolTypeDocuments {
		var cfgMap map[string]any
		_ = json.Unmarshal(existing.Config, &cfgMap)
		kbID, _ := cfgMap["kbId"].(string)
		if strings.TrimSpace(kbID) == "" {
			if v, _ := cfgMap["knowledgeBaseId"].(string); strings.TrimSpace(v) != "" {
				kbID = v
			}
		}
		if strings.TrimSpace(kbID) == "" {
			return Response{}, fmt.Errorf("%w: kbId is required for DOCUMENT_SEARCH tools", ErrValidation)
		}
		kbUUID, err := uuid.Parse(strings.TrimSpace(kbID))
		if err != nil {
			return Response{}, fmt.Errorf("%w: kbId must be a valid UUID (got %q)", ErrValidation, kbID)
		}
		// Bug 231: verificar existência da KB no tenant atual.
		if s.kbRdr != nil {
			if err := s.kbRdr.GetByID(ctx, kbUUID); err != nil {
				return Response{}, fmt.Errorf("%w: kbId not found", ErrValidation)
			}
		}
	}

	// P-C220-1 / P-C221-1 / P-C254-1: re-validate URL when type or config changed.
	// HTTP tools require a non-empty URL; reject updates that would leave it absent.
	if existing.Type == ToolTypeHTTP {
		u := extractURLFromConfig(existing.Config)
		if u == "" {
			return Response{}, fmt.Errorf("%w: url is required for HTTP tools", ErrValidation)
		}
		if err := s.validateHTTPToolURL(httpURLForValidationFromConfig(existing.Config, u, s.httpBackendBase)); err != nil {
			return Response{}, fmt.Errorf("%w: invalid URL (%v)", ErrValidation, err)
		}
		if err := validateHTTPURLTemplates(existing.Config); err != nil {
			return Response{}, err
		}
		// Bug 108: Update precisa do mesmo gate de method enum + timeout
		// que Create — senão admin podia criar tool são e depois PATCH
		// com method=BLAH ou timeoutSeconds=99999 (nunca executa, ou
		// trava o request por 27h).
		if err := validateHTTPMethodAndTimeout(existing.Config); err != nil {
			return Response{}, err
		}
		existing.Config = normalizeHTTPMethodConfig(existing.Config)
	}

	t, err := s.repo.Update(ctx, id, existing)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(t), nil
}

// validateHTTPURLTemplates rejects traversal and encoded slash payloads in URL templates.
func validateHTTPURLTemplates(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	for _, field := range []string{"url", "urlTemplate"} {
		v, _ := cfg[field].(string)
		if v == "" {
			continue
		}
		if containsUnsafeURLPathToken(v) {
			return fmt.Errorf("%w: %s must not contain path traversal or encoded slashes", ErrValidation, field)
		}
	}
	return nil
}

func containsUnsafeURLPathToken(raw string) bool {
	candidate := strings.TrimSpace(raw)
	for i := 0; i < 3; i++ {
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, "../") ||
			strings.Contains(lower, `..\`) ||
			strings.Contains(lower, "%2f") ||
			strings.Contains(lower, "%5c") {
			return true
		}
		decoded, err := url.PathUnescape(candidate)
		if err != nil || decoded == candidate {
			break
		}
		candidate = decoded
	}
	return false
}

const (
	httpToolMaxURLLength          = 2048
	httpToolMaxBodyTemplateLength = 32000
	httpToolMaxHeaderCount        = 50
)

// validateHTTPMethodAndTimeout checks optional HTTP runtime fields. Bug
// 95/97/108: same rules used by both Create and Update so PATCH can't bypass.
func validateHTTPMethodAndTimeout(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	return validateHTTPRuntimeConfigFields(cfg)
}

func validateHTTPRuntimeConfigFields(cfg map[string]any) error {
	if err := validateHTTPMethodField(cfg); err != nil {
		return err
	}
	if err := validateHTTPTimeoutFields(cfg); err != nil {
		return err
	}
	if err := validateHTTPRuntimeScalarFields(cfg); err != nil {
		return err
	}
	if err := validateHTTPRuntimeAliasContract(cfg); err != nil {
		return err
	}
	if err := validateHTTPConfigSizeLimits(cfg); err != nil {
		return err
	}
	if err := validateHTTPBodyTemplatePlaceholders(cfg); err != nil {
		return err
	}
	if err := validateHTTPHeadersConfig(cfg); err != nil {
		return err
	}
	return nil
}

func validateHTTPConfigSizeLimits(cfg map[string]any) error {
	for _, key := range []string{"url", "urlTemplate"} {
		rawValue, ok := cfg[key]
		if !ok || rawValue == nil {
			continue
		}
		value, ok := rawValue.(string)
		if !ok {
			return fmt.Errorf("%w: %s must be a string", ErrValidation, key)
		}
		if len(value) > httpToolMaxURLLength {
			return fmt.Errorf("%w: %s exceeds maximum length of %d chars (got %d)", ErrValidation, key, httpToolMaxURLLength, len(value))
		}
	}
	for _, key := range []string{"bodyTemplate", "body_template", "body"} {
		rawValue, ok := cfg[key]
		if !ok || rawValue == nil {
			continue
		}
		value := httpBodyTemplateValue(rawValue)
		if len(value) > httpToolMaxBodyTemplateLength {
			return fmt.Errorf("%w: %s exceeds maximum length of %d chars (got %d)", ErrValidation, key, httpToolMaxBodyTemplateLength, len(value))
		}
	}
	return nil
}

func validateHTTPBodyTemplatePlaceholders(cfg map[string]any) error {
	for _, key := range []string{"bodyTemplate", "body_template", "body"} {
		rawValue, ok := cfg[key]
		if !ok || rawValue == nil {
			continue
		}
		bodyTemplate := httpBodyTemplateValue(rawValue)
		if placeholder, ok := sensitiveHTTPBodyTemplatePlaceholder(bodyTemplate); ok {
			return fmt.Errorf("%w: %s contains sensitive placeholder %q", ErrValidation, key, placeholder)
		}
	}
	return nil
}

func validateHTTPHeadersConfig(cfg map[string]any) error {
	// Bug 251: validar header names (cluster XSS via map key — bugs 248/249/250).
	// Mesmo pattern do integration.httpHeaderNamePattern: RFC 7230 token subset.
	if rawHeaders, ok := cfg["headers"]; ok && rawHeaders != nil {
		hdrs, ok := rawHeaders.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: headers must be an object", ErrValidation)
		}
		if len(hdrs) > httpToolMaxHeaderCount {
			return fmt.Errorf("%w: headers exceeds maximum of %d entries (got %d)", ErrValidation, httpToolMaxHeaderCount, len(hdrs))
		}
		for k, v := range hdrs {
			if !toolHTTPHeaderNamePattern.MatchString(k) {
				return fmt.Errorf("%w: header name %q invalid — must match RFC 7230 token (alphanumeric + -_)", ErrValidation, k)
			}
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%w: header value for %q must be a string", ErrValidation, k)
			}
		}
	}
	return nil
}

func validateHTTPRuntimeScalarFields(cfg map[string]any) error {
	for _, key := range []string{"authType", "auth_type", "authToken", "auth_token", "baseUrl"} {
		rawValue, ok := cfg[key]
		if !ok || rawValue == nil {
			continue
		}
		if _, ok := rawValue.(string); !ok {
			return fmt.Errorf("%w: %s must be a string", ErrValidation, key)
		}
	}
	for _, key := range []string{"authType", "auth_type"} {
		rawValue, ok := cfg[key]
		if !ok || rawValue == nil {
			continue
		}
		authType, _ := rawValue.(string)
		if authType == "" {
			continue
		}
		if strings.TrimSpace(authType) != authType {
			return fmt.Errorf("%w: %s must be one of none, bearer, basic (got %q)", ErrValidation, key, authType)
		}
		switch strings.ToLower(authType) {
		case "none", "bearer", "basic":
		default:
			return fmt.Errorf("%w: %s must be one of none, bearer, basic (got %q)", ErrValidation, key, authType)
		}
	}
	for _, key := range []string{"useCallerToken", "use_caller_token"} {
		rawValue, ok := cfg[key]
		if !ok || rawValue == nil {
			continue
		}
		if _, ok := rawValue.(bool); !ok {
			return fmt.Errorf("%w: %s must be a bool", ErrValidation, key)
		}
	}
	return nil
}

// validateHTTPRuntimeAliasContract rejects ambiguous aliases before the config
// reaches the executor. Legacy aliases remain supported when they produce the
// same effective runtime value; an order-dependent winner is never selected.
func validateHTTPRuntimeAliasContract(cfg map[string]any) error {
	if err := validateHTTPStringAliases(cfg, "url", "urlTemplate", func(value string) string { return value }); err != nil {
		return err
	}
	if err := validateHTTPBodyTemplateAliases(cfg); err != nil {
		return err
	}
	if err := validateHTTPTimeoutAliases(cfg); err != nil {
		return err
	}
	if err := validateHTTPStringAliases(cfg, "authType", "auth_type", strings.ToLower); err != nil {
		return err
	}
	if err := validateHTTPStringAliases(cfg, "authToken", "auth_token", func(value string) string { return value }); err != nil {
		return err
	}
	if err := validateHTTPBoolAliases(cfg, "useCallerToken", "use_caller_token"); err != nil {
		return err
	}
	return nil
}

func validateHTTPBodyTemplateAliases(cfg map[string]any) error {
	return validateHTTPAliases(cfg, []string{"bodyTemplate", "body_template", "body"}, canonicalHTTPBodyTemplateAlias)
}

func validateHTTPTimeoutAliases(cfg map[string]any) error {
	canonical := 0
	canonicalField := ""
	for _, field := range []struct {
		key      string
		toSecond func(int) int
	}{
		{key: "timeoutSeconds", toSecond: func(value int) int { return value }},
		{key: "timeout_seconds", toSecond: func(value int) int { return value }},
		{key: "timeoutMs", toSecond: func(value int) int { return (value + 999) / 1000 }},
	} {
		value, ok := positiveHTTPConfigInt(cfg[field.key]), cfg[field.key] != nil
		if !ok || value == 0 {
			continue
		}
		value = field.toSecond(value)
		if canonicalField != "" && canonical != value {
			return conflictingHTTPAliasError(canonicalField, field.key)
		}
		canonical = value
		canonicalField = field.key
	}
	return nil
}

func validateHTTPStringAliases(cfg map[string]any, camelKey, snakeKey string, normalize func(string) string) error {
	camel, camelSet := cfg[camelKey].(string)
	snake, snakeSet := cfg[snakeKey].(string)
	if !camelSet || camel == "" || !snakeSet || snake == "" {
		return nil
	}
	if normalize(camel) != normalize(snake) {
		return conflictingHTTPAliasError(camelKey, snakeKey)
	}
	return nil
}

func validateHTTPBoolAliases(cfg map[string]any, camelKey, snakeKey string) error {
	camel, camelSet := cfg[camelKey].(bool)
	snake, snakeSet := cfg[snakeKey].(bool)
	if !camelSet || !snakeSet || camel == snake {
		return nil
	}
	return conflictingHTTPAliasError(camelKey, snakeKey)
}

func validateHTTPAliases(cfg map[string]any, keys []string, canonicalize func(any) string) error {
	canonical := ""
	canonicalField := ""
	for _, key := range keys {
		value, ok := cfg[key]
		if !ok || value == nil {
			continue
		}
		valueCanonical := canonicalize(value)
		if canonicalField != "" && canonical != valueCanonical {
			return conflictingHTTPAliasError(canonicalField, key)
		}
		canonical = valueCanonical
		canonicalField = key
	}
	return nil
}

func canonicalHTTPBodyTemplateAlias(value any) string {
	body := httpBodyTemplateValue(value)
	var decoded any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return body
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return body
	}
	return string(canonical)
}

func conflictingHTTPAliasError(first, second string) error {
	return fmt.Errorf("%w: conflicting HTTP config aliases %s and %s", ErrValidation, first, second)
}

func validateHTTPMethodField(cfg map[string]any) error {
	rawMethod, ok := cfg["method"]
	if !ok || rawMethod == nil {
		return nil
	}
	m, ok := rawMethod.(string)
	if !ok {
		return fmt.Errorf("%w: method must be a string", ErrValidation)
	}
	if m == "" {
		return nil
	}
	switch strings.ToUpper(strings.TrimSpace(m)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return nil
	default:
		return fmt.Errorf("%w: method must be one of GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS (got %q)", ErrValidation, m)
	}
}

func validateHTTPTimeoutFields(cfg map[string]any) error {
	fields := []struct {
		key string
		max float64
	}{
		{key: "timeoutSeconds", max: 600},
		{key: "timeout_seconds", max: 600},
		{key: "timeoutMs", max: 600000},
	}
	for _, field := range fields {
		v, ok := cfg[field.key]
		if !ok || v == nil {
			continue
		}
		n, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%w: %s must be a number", ErrValidation, field.key)
		}
		if math.Trunc(n) != n {
			return fmt.Errorf("%w: %s must be an integer", ErrValidation, field.key)
		}
		if n < 1 || n > field.max {
			return fmt.Errorf("%w: %s must be between 1 and %.0f (got %.0f)", ErrValidation, field.key, field.max, n)
		}
	}
	return nil
}

func normalizeHTTPMethodConfig(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return raw
	}
	methodRaw, ok := cfg["method"]
	if !ok {
		return raw
	}
	var method string
	if err := json.Unmarshal(methodRaw, &method); err != nil {
		return raw
	}
	normalized := strings.ToUpper(strings.TrimSpace(method))
	if normalized == "" || normalized == method {
		return raw
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return raw
	}
	cfg["method"] = encoded
	out, err := json.Marshal(cfg)
	if err != nil {
		return raw
	}
	return out
}

// toolHTTPHeaderNamePattern subset prático do RFC 7230 token. Bug 251:
// sem este gate, headers com keys "<script>"/""/com espaços eram persistidas
// e enviadas como header HTTP inválido para o upstream.
var toolHTTPHeaderNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

var httpBodyTemplatePlaceholderPattern = regexp.MustCompile(`\{\{\s*(?:input|args)\.([A-Za-z0-9_-]+)\s*\}\}|\{\{\s*([A-Za-z0-9_-]+)\s*\}\}|\{\s*([A-Za-z0-9_-]+)\s*\}`)

var sensitiveHTTPBodyTemplatePlaceholderKeys = map[string]bool{
	"api_key":    true,
	"apikey":     true,
	"auth_token": true,
	"authtoken":  true,
	"password":   true,
	"secret":     true,
}

func sensitiveHTTPBodyTemplatePlaceholder(tmpl string) (string, bool) {
	for _, match := range httpBodyTemplatePlaceholderPattern.FindAllStringSubmatch(tmpl, -1) {
		for _, candidate := range match[1:] {
			if candidate == "" {
				continue
			}
			key := strings.ToLower(strings.ReplaceAll(candidate, "-", "_"))
			if sensitiveHTTPBodyTemplatePlaceholderKeys[key] {
				return candidate, true
			}
		}
	}
	return "", false
}

// extractURLFromConfig extracts the "url" or "urlTemplate" field from a JSON config blob.
// Returns empty string when the config is nil, unparseable, or has no URL field.
func extractURLFromConfig(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var cfg struct {
		URL         string `json:"url"`
		URLTemplate string `json:"urlTemplate"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	if strings.TrimSpace(cfg.URL) != "" {
		return strings.TrimSpace(cfg.URL)
	}
	return strings.TrimSpace(cfg.URLTemplate)
}

func httpURLForValidationFromConfig(raw json.RawMessage, rawURL, backendBaseURL string) string {
	if len(raw) == 0 {
		return rawURL
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return rawURL
	}
	return httpURLWithBaseFromConfig(cfg, rawURL, backendBaseURL)
}

// isSQLToolType returns true for SQL / DATABASE tool types.
func isSQLToolType(t ToolType) bool {
	return t == ToolTypeSQL || t == ToolTypeDatabase
}

// normalizeDataSourceID normalises camelCase `dataSourceId` to snake_case `datasource_id`
// in a SQL tool config blob. P-C249-1: the skill-runtime SQL executor expects
// datasource_id; frontend / integrations may produce datasourceId.
// Returns the original slice unchanged when the key is already absent or the
// JSON cannot be parsed.
func normalizeDataSourceID(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return raw
	}
	val, hasCamel := cfg["dataSourceId"]
	if !hasCamel {
		return raw // nothing to do
	}
	// Preserve the canonical field when both are present. Callers validate that
	// the aliases agree before normalizing, so this can never change the target
	// datasource by field order.
	if _, hasSnake := cfg["datasource_id"]; !hasSnake {
		cfg["datasource_id"] = val
	}
	delete(cfg, "dataSourceId")
	normalized, err := json.Marshal(cfg)
	if err != nil {
		return raw
	}
	return normalized
}

func validateSQLDataSourceAliasContract(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	snake, hasSnake := cfg["datasource_id"]
	camel, hasCamel := cfg["dataSourceId"]
	if !hasSnake || !hasCamel {
		return nil
	}

	var snakeID, camelID string
	if err := json.Unmarshal(snake, &snakeID); err != nil {
		return conflictingSQLDataSourceAliasesError()
	}
	if err := json.Unmarshal(camel, &camelID); err != nil {
		return conflictingSQLDataSourceAliasesError()
	}
	snakeUUID, snakeErr := uuid.Parse(strings.TrimSpace(snakeID))
	camelUUID, camelErr := uuid.Parse(strings.TrimSpace(camelID))
	if snakeErr == nil && camelErr == nil && snakeUUID == camelUUID {
		return nil
	}
	if strings.TrimSpace(snakeID) == strings.TrimSpace(camelID) {
		return nil
	}
	return conflictingSQLDataSourceAliasesError()
}

func conflictingSQLDataSourceAliasesError() error {
	return fmt.Errorf("%w: conflicting SQL config aliases datasource_id and dataSourceId", ErrValidation)
}

// Delete deletes a tool.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	existing, getErr := s.repo.GetByID(ctx, id)
	if getErr != nil {
		return s.repo.Delete(ctx, id)
	}

	refs, refsErr := s.listSkillInstructionReferencesByTool(ctx, id)
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if refsErr != nil {
		slog.WarnContext(ctx, "tool: failed to inspect skill instruction references before delete",
			"tool_id", id,
			"err", refsErr,
		)
		return nil
	}
	warnDeletedToolInstructionReferences(ctx, existing, refs)
	return nil
}

func (s *Service) listSkillInstructionReferencesByTool(ctx context.Context, toolID uuid.UUID) ([]SkillInstructionReference, error) {
	lister, ok := s.repo.(skillInstructionReferenceLister)
	if !ok {
		return nil, nil
	}
	return lister.ListSkillInstructionReferencesByTool(ctx, toolID)
}

func warnDeletedToolInstructionReferences(ctx context.Context, t Tool, refs []SkillInstructionReference) {
	for _, ref := range refs {
		if !instructionsReferenceTool(ref.Instructions, t) {
			continue
		}
		slog.WarnContext(ctx, "tool: deleted tool referenced by skill instructions",
			"tool_id", t.ID,
			"tool_name", t.Name,
			"tool_slug", t.Slug,
			"skill_id", ref.SkillID,
			"skill_name", ref.SkillName,
		)
	}
}

func instructionsReferenceTool(instructions string, t Tool) bool {
	lowerInstructions := strings.ToLower(instructions)
	for _, candidate := range toolReferenceCandidates(t) {
		if strings.Contains(lowerInstructions, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func toolReferenceCandidates(t Tool) []string {
	candidates := make([]string, 0, 5)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		candidates = append(candidates, value)
	}
	add(t.Slug)
	add(strings.ReplaceAll(t.Slug, "-", "_"))
	add(strings.ReplaceAll(t.Slug, "_", "-"))
	add(t.Name)
	add(ToSlug(t.Name))
	return candidates
}

// BindToSkill binds a tool to a skill.
func (s *Service) BindToSkill(ctx context.Context, skillID uuid.UUID, req BindRequest) (SkillToolResponse, error) {
	st, err := s.repo.BindToSkill(ctx, skillID, req)
	if err != nil {
		return SkillToolResponse{}, err
	}
	t, err := s.repo.GetByID(ctx, st.ToolID)
	if err != nil {
		return SkillToolResponse{}, err
	}
	return SkillToolResponse{
		ID:        st.ID,
		SkillID:   st.SkillID,
		Tool:      ResponseFrom(t),
		Priority:  st.Priority,
		IsActive:  st.IsActive,
		CreatedAt: st.CreatedAt,
	}, nil
}

// UnbindFromSkill removes a tool binding from a skill.
func (s *Service) UnbindFromSkill(ctx context.Context, skillID, toolID uuid.UUID) error {
	return s.repo.UnbindFromSkill(ctx, skillID, toolID)
}

// ListBySkill returns all tools bound to a skill.
func (s *Service) ListBySkill(ctx context.Context, skillID uuid.UUID) ([]SkillToolResponse, error) {
	bindings, tools, err := s.repo.ListBySkill(ctx, skillID)
	if err != nil {
		return nil, err
	}
	resp := make([]SkillToolResponse, len(bindings))
	for i, st := range bindings {
		resp[i] = SkillToolResponse{
			ID:        st.ID,
			SkillID:   st.SkillID,
			Tool:      ResponseFrom(tools[i]),
			Priority:  st.Priority,
			IsActive:  st.IsActive,
			CreatedAt: st.CreatedAt,
		}
	}
	return resp, nil
}

// TestTool executes a supported tool with the given inputs and returns the result as a string.
func (s *Service) TestTool(ctx context.Context, id uuid.UUID, inputs map[string]any) (string, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	var cfg map[string]any
	if err := json.Unmarshal(t.Config, &cfg); err != nil {
		return "", fmt.Errorf("tool: invalid config JSON: %w", err)
	}

	switch t.Type {
	case ToolTypeHTTP:
		return s.testHTTPTool(ctx, cfg, t.InputSchema, inputs)
	case ToolTypeSQL, ToolTypeDatabase:
		return s.testSQLTool(ctx, cfg, inputs)
	default:
		return "", fmt.Errorf("tool: test not supported for type %s", t.Type)
	}
}

func (s *Service) testHTTPTool(ctx context.Context, cfg map[string]any, inputSchema json.RawMessage, inputs map[string]any) (string, error) {
	rawURL, _ := cfg["url"].(string)
	if rawURL == "" {
		rawURL, _ = cfg["urlTemplate"].(string)
	}
	if rawURL == "" {
		return "", fmt.Errorf("tool: HTTP tool has no url configured")
	}
	if err := validateHTTPRuntimeConfigFields(cfg); err != nil {
		return "", fmt.Errorf("tool: %w", err)
	}
	method, _ := cfg["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}

	// Render URL placeholders with the same aliases accepted by the
	// skill-runtime HTTP executor so the admin test endpoint matches runtime.
	rawURL = httpURLWithBaseFromConfig(cfg, rawURL, s.httpBackendBase)
	rawTemplate := rawURL
	rawURL = renderURLPlaceholders(rawURL, inputs)
	if shouldAppendUnusedHTTPInput(method) {
		rawURL = appendUnusedHTTPInputAsQuery(rawTemplate, rawURL, inputs, httpInputSchemaKeys(cfg, inputSchema))
	}
	if containsUnsafeURLPathToken(rawURL) {
		return "", fmt.Errorf("tool: path traversal blocked in URL")
	}
	if err := s.validateHTTPToolURL(rawURL); err != nil {
		return "", fmt.Errorf("tool: blocked URL (%w)", err)
	}

	var reqBody io.Reader
	hasBody := false
	if bodyTemplate := httpBodyTemplateFromConfig(cfg); bodyTemplate != "" {
		reqBody = strings.NewReader(renderPlaceholders(bodyTemplate, inputs))
		hasBody = true
	}

	timeoutSeconds := httpTimeoutSecondsFromConfig(cfg)
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	client := &http.Client{
		CheckRedirect: func(redirect *http.Request, _ []*http.Request) error {
			rawRedirectURL := redirect.URL.String()
			if containsUnsafeURLPathToken(rawRedirectURL) {
				return errBlockedHTTPToolRedirect
			}
			if err := s.validateHTTPToolURL(rawRedirectURL); err != nil {
				return errBlockedHTTPToolRedirect
			}
			return nil
		},
	}
	// #nosec G704 -- rawURL was checked by containsUnsafeURLPathToken and validateHTTPToolURL before request creation.
	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, reqBody)
	if err != nil {
		return "", fmt.Errorf("tool: create HTTP request: %w", err)
	}

	if rawHeaders, ok := cfg["headers"]; ok && rawHeaders != nil {
		headers, ok := rawHeaders.(map[string]any)
		if !ok {
			return "", fmt.Errorf("tool: headers must be an object")
		}
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, renderPlaceholders(vs, inputs))
				continue
			}
			return "", fmt.Errorf("tool: header value for %q must be a string", k)
		}
	}
	if hasBody && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply runtime-compatible authentication. P-C239-1: authToken is never
	// returned to the client, only sent on the outbound request.
	authType, _ := cfg["authType"].(string)
	if authType == "" {
		authType, _ = cfg["auth_type"].(string)
	}
	authToken, _ := cfg["authToken"].(string)
	if authToken == "" {
		authToken, _ = cfg["auth_token"].(string)
	}
	callerToken := tenant.TokenFromContext(ctx)
	useCallerToken, _ := cfg["useCallerToken"].(bool)
	if _, hasPreferred := cfg["useCallerToken"]; !hasPreferred {
		useCallerToken, _ = cfg["use_caller_token"].(bool)
	}
	switch {
	case useCallerToken && callerToken != "":
		req.Header.Set("Authorization", "Bearer "+callerToken)
	case strings.EqualFold(authType, "bearer"):
		if authToken != "" {
			req.Header.Set("Authorization", "Bearer "+authToken)
		}
	case strings.EqualFold(authType, "basic"):
		if authToken != "" {
			req.Header.Set("Authorization", "Basic "+authToken)
		}
	}

	// #nosec G704 -- request URL was validated for HTTP tool SSRF constraints before this call.
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, errBlockedHTTPToolRedirect) {
			return "", fmt.Errorf("tool: blocked redirect URL")
		}
		return "", fmt.Errorf("%w: HTTP request failed", ErrUpstream)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%w: read HTTP response body", ErrUpstream)
	}
	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		if body := strings.TrimSpace(redactHTTPToolTestResponseBody(string(respBytes), cfg, inputs, req.Header)); body != "" {
			msg = fmt.Sprintf("%s: %s", msg, body)
		}
		return "", fmt.Errorf("%w: %s", ErrUpstream, msg)
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "Status: %d %s\n\n", resp.StatusCode, resp.Status)
	buf.WriteString(redactHTTPToolTestResponseBody(string(respBytes), cfg, inputs, req.Header))

	return buf.String(), nil
}

var sensitiveToolTestResponseHeaderPattern = regexp.MustCompile(`(?im)\b(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|x-api-token|x-auth-token|x-access-token|x-secret)\s*:\s*[^\r\n]*`)

var sensitiveToolTestResponsePairPattern = regexp.MustCompile(`(?i)\b((?:api[-_]?key|api[-_]?token|auth(?:orization)?|access[-_]?token|password|secret|token)\s*=\s*)[^\s&;,]+`)

var errBlockedHTTPToolRedirect = errors.New("blocked HTTP tool redirect")

func redactHTTPToolTestResponseBody(raw string, cfg map[string]any, inputs map[string]any, headers http.Header) string {
	redacted := raw
	secrets := make(map[string]struct{})
	collectSensitiveToolTestValues(cfg, secrets)
	collectSensitiveToolTestValues(inputs, secrets)
	for name, values := range headers {
		if !sensitiveHeaderKeys[strings.ToLower(name)] {
			continue
		}
		for _, value := range values {
			if value != "" {
				secrets[value] = struct{}{}
			}
		}
	}
	for secret := range secrets {
		redacted = strings.ReplaceAll(redacted, secret, "***")
	}

	var body any
	if err := json.Unmarshal([]byte(redacted), &body); err == nil {
		if encoded, err := json.Marshal(redactSensitiveToolTestValue(body)); err == nil {
			redacted = string(encoded)
		}
	}
	redacted = sensitiveToolTestResponseHeaderPattern.ReplaceAllString(redacted, "[REDACTED]")
	return sensitiveToolTestResponsePairPattern.ReplaceAllString(redacted, "${1}***")
}

func collectSensitiveToolTestValues(value any, secrets map[string]struct{}) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if isSensitiveToolConfigKey(key) {
				collectToolTestSecretStrings(child, secrets)
				continue
			}
			if isToolConfigURLKey(key) {
				collectSensitiveToolTestURLValues(child, secrets)
				continue
			}
			collectSensitiveToolTestValues(child, secrets)
		}
	case []any:
		for _, child := range v {
			collectSensitiveToolTestValues(child, secrets)
		}
	}
}

func collectSensitiveToolTestURLValues(value any, secrets map[string]struct{}) {
	rawURL, ok := value.(string)
	if !ok || rawURL == "" {
		return
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	if parsed.User != nil {
		if username := parsed.User.Username(); username != "" {
			secrets[username] = struct{}{}
		}
		if password, ok := parsed.User.Password(); ok && password != "" {
			secrets[password] = struct{}{}
		}
	}

	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return
	}
	for key, values := range query {
		if !isSensitiveToolConfigKey(key) {
			continue
		}
		for _, value := range values {
			if value != "" {
				secrets[value] = struct{}{}
			}
		}
	}
}

func collectToolTestSecretStrings(value any, secrets map[string]struct{}) {
	switch v := value.(type) {
	case string:
		if v != "" {
			secrets[v] = struct{}{}
		}
	case map[string]any:
		for _, child := range v {
			collectToolTestSecretStrings(child, secrets)
		}
	case []any:
		for _, child := range v {
			collectToolTestSecretStrings(child, secrets)
		}
	}
}

func redactSensitiveToolTestValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveToolConfigKey(key) {
				out[key] = "***"
				continue
			}
			out[key] = redactSensitiveToolTestValue(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactSensitiveToolTestValue(child)
		}
		return out
	default:
		return value
	}
}

func (s *Service) validateHTTPToolURL(rawURL string) error {
	if isTrustedHTTPBackendURL(rawURL, s.httpBackendBase) {
		return nil
	}
	validator := ssrf.ValidateURL
	if s.httpURLValidator != nil {
		validator = s.httpURLValidator
	}
	return validator(rawURL)
}

func normalizeHTTPBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func httpBodyTemplateFromConfig(cfg map[string]any) string {
	for _, key := range []string{"bodyTemplate", "body_template", "body"} {
		v, ok := cfg[key]
		if !ok || v == nil {
			continue
		}
		return httpBodyTemplateValue(v)
	}
	return ""
}

func httpBodyTemplateValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func httpURLWithBaseFromConfig(cfg map[string]any, rawURL, backendBaseURL string) string {
	if strings.HasPrefix(rawURL, "http") {
		return rawURL
	}
	baseURL, _ := cfg["baseUrl"].(string)
	if strings.TrimSpace(baseURL) == "" {
		baseURL = backendBaseURL
	}
	if strings.TrimSpace(baseURL) == "" {
		return rawURL
	}
	return strings.TrimRight(baseURL, "/") + rawURL
}

func isTrustedHTTPBackendURL(rawURL, backendBaseURL string) bool {
	trustedBase := normalizeHTTPBaseURL(backendBaseURL)
	if trustedBase == "" {
		return false
	}
	return rawURL == trustedBase || strings.HasPrefix(rawURL, trustedBase+"/")
}

func httpTimeoutSecondsFromConfig(cfg map[string]any) int {
	if seconds := positiveHTTPConfigInt(cfg["timeoutSeconds"]); seconds > 0 {
		return seconds
	}
	if seconds := positiveHTTPConfigInt(cfg["timeout_seconds"]); seconds > 0 {
		return seconds
	}
	if timeoutMs := positiveHTTPConfigInt(cfg["timeoutMs"]); timeoutMs > 0 {
		seconds := (timeoutMs + 999) / 1000
		if seconds < 1 {
			return 1
		}
		return seconds
	}
	return 30
}

func positiveHTTPConfigInt(v any) int {
	switch n := v.(type) {
	case int:
		if n > 0 {
			return n
		}
	case int64:
		if n > 0 {
			return int(n)
		}
	case float64:
		if n > 0 {
			return int(n)
		}
	case json.Number:
		i, err := n.Int64()
		if err == nil && i > 0 {
			return int(i)
		}
	}
	return 0
}

func shouldAppendUnusedHTTPInput(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodDelete, http.MethodHead:
		return true
	default:
		return false
	}
}

func appendUnusedHTTPInputAsQuery(rawTemplate, renderedURL string, input map[string]any, allowedKeys map[string]bool) string {
	if len(input) == 0 {
		return renderedURL
	}

	used := usedHTTPTemplateKeys(rawTemplate, input)
	query := url.Values{}
	for key, value := range input {
		if used[key] {
			continue
		}
		if allowedKeys != nil && !allowedKeys[key] {
			continue
		}
		query.Set(key, fmt.Sprintf("%v", value))
	}
	if len(query) == 0 {
		return renderedURL
	}

	separator := "?"
	if strings.Contains(renderedURL, "?") {
		separator = "&"
	}
	return renderedURL + separator + query.Encode()
}

func usedHTTPTemplateKeys(rawTemplate string, input map[string]any) map[string]bool {
	used := make(map[string]bool, len(input))
	for key := range input {
		if strings.Contains(rawTemplate, "{"+key+"}") ||
			strings.Contains(rawTemplate, "{{"+key+"}}") ||
			strings.Contains(rawTemplate, "{{input."+key+"}}") ||
			strings.Contains(rawTemplate, "{{args."+key+"}}") {
			used[key] = true
		}
	}
	return used
}

func httpInputSchemaKeys(cfg map[string]any, inputSchema json.RawMessage) map[string]bool {
	raw := []byte(inputSchema)
	if len(raw) == 0 {
		v, ok := cfg["inputSchema"]
		if !ok || v == nil {
			return nil
		}
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		raw = data
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}
	if len(schema.Properties) == 0 {
		return nil
	}

	keys := make(map[string]bool, len(schema.Properties))
	for key := range schema.Properties {
		keys[key] = true
	}
	return keys
}

// renderPlaceholders substitutes HTTP tool template aliases in s with the
// corresponding string value from inputs. Missing keys are left untouched so
// the caller can see the failure in the outbound header/body.
func renderPlaceholders(s string, inputs map[string]any) string {
	if len(inputs) == 0 {
		return s
	}
	pairs := make([]string, 0, len(inputs)*8)
	for k, v := range inputs {
		val := fmt.Sprintf("%v", v)
		pairs = append(pairs,
			"{{"+k+"}}", val,
			"{"+k+"}", val,
			"{{input."+k+"}}", val,
			"{{args."+k+"}}", val,
		)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

func renderURLPlaceholders(s string, inputs map[string]any) string {
	if len(inputs) == 0 {
		return s
	}
	pairs := make([]string, 0, len(inputs)*8)
	for k, v := range inputs {
		val := url.QueryEscape(fmt.Sprintf("%v", v))
		pairs = append(pairs,
			"{{"+k+"}}", val,
			"{"+k+"}", val,
			"{{input."+k+"}}", val,
			"{{args."+k+"}}", val,
		)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// GenerateCode uses the configured LLM to generate code for a tool.
func (s *Service) GenerateCode(ctx context.Context, prompt, language string) (string, error) {
	cfg, err := s.readLLMConfig(ctx)
	if err != nil {
		return "", err
	}

	systemPrompt := fmt.Sprintf(
		"You are an expert %s programmer. Generate clean, well-commented code for the given task. "+
			"Return ONLY the code, no markdown fences, no explanation.",
		language,
	)
	return callLLM(ctx, cfg, systemPrompt, prompt)
}

// GenerateBlockly uses the configured LLM to generate a Blockly workspace JSON.
func (s *Service) GenerateBlockly(ctx context.Context, prompt string) (any, error) {
	cfg, err := s.readLLMConfig(ctx)
	if err != nil {
		return nil, err
	}

	systemPrompt := `You are an expert at creating Blockly workspace definitions.
Generate a valid Blockly workspace JSON that implements the described logic.
Return ONLY valid JSON, no markdown fences, no explanation.`

	raw, err := callLLM(ctx, cfg, systemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	// Parse and re-encode to ensure valid JSON.
	var result any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("tool: LLM did not return valid JSON: %w", err)
	}
	return result, nil
}

// GetDatabaseSchema connects to the datasource and returns its schema.
func (s *Service) GetDatabaseSchema(ctx context.Context, dataSourceID string) (DatabaseSchema, error) {
	if s.dsRdr == nil || s.tenantIDFn == nil {
		return DatabaseSchema{}, fmt.Errorf("tool: database schema feature not configured")
	}

	dsID, err := uuid.Parse(dataSourceID)
	if err != nil {
		return DatabaseSchema{}, fmt.Errorf("%w: %v", ErrInvalidDataSourceID, err)
	}

	tenantID := s.tenantIDFn(ctx)
	creds, err := s.dsRdr.GetDatasourceCreds(ctx, tenantID, dsID)
	if err != nil {
		if errors.Is(err, datasource.ErrNotFound) {
			return DatabaseSchema{}, ErrDataSourceNotFound
		}
		return DatabaseSchema{}, fmt.Errorf("tool: get datasource: %w", err)
	}

	switch strings.ToUpper(creds.Type) {
	case "POSTGRESQL":
		dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
			creds.Host, creds.Port, creds.Database, creds.User, creds.Password)
		return fetchPostgresSchema(ctx, dsn)
	default:
		return DatabaseSchema{}, fmt.Errorf("tool: schema introspection not supported for %s", creds.Type)
	}
}

// readLLMConfig reads the LLM configuration from tenant settings.
func (s *Service) readLLMConfig(ctx context.Context) (llmConfig, error) {
	if s.settingsRdr == nil {
		return llmConfig{}, errors.New("tool: LLM feature not configured — set llm.tool.config in settings")
	}

	raw, err := s.settingsRdr.FindSettingByKey(ctx, "llm.tool.config")
	if err != nil {
		return llmConfig{}, fmt.Errorf("tool: llm.tool.config not found in settings — configure provider, apiKey and model")
	}

	var cfg llmConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return llmConfig{}, fmt.Errorf("tool: invalid llm.tool.config value: %w", err)
	}
	if cfg.Model == "" {
		return llmConfig{}, errors.New("tool: llm.tool.config.model is required")
	}
	return cfg, nil
}
