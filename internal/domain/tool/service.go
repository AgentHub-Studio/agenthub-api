package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

// SettingsReader is a minimal interface for reading tenant settings.
type SettingsReader interface {
	FindSettingByKey(ctx context.Context, key string) ([]byte, error)
}

// DatasourceReader is a minimal interface for reading datasource credentials.
type DatasourceReader interface {
	GetDatasourceCreds(ctx context.Context, tenantID string, id uuid.UUID) (DatasourceCreds, error)
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
	repo        ToolRepository
	settingsRdr SettingsReader
	dsRdr       DatasourceReader
	tenantIDFn  func(ctx context.Context) string
}

// NewService creates a new Service.
func NewService(repo ToolRepository) *Service {
	return &Service{repo: repo}
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
	if !IsValidToolType(req.Type) {
		return Response{}, fmt.Errorf("%w: unsupported type: %s", ErrValidation, req.Type)
	}
	// P-C220-1 / P-C221-1 / P-C254-1: HTTP tools require a non-empty URL.
	if req.Type == ToolTypeHTTP {
		u := extractURLFromConfig(req.Config)
		if u == "" {
			return Response{}, fmt.Errorf("%w: url is required for HTTP tools", ErrValidation)
		}
		if err := ssrf.ValidateURL(u); err != nil {
			return Response{}, fmt.Errorf("%w: invalid URL (%v)", ErrValidation, err)
		}
	}
	// P-C249-1: normalize SQL tool config — accept both datasourceId and datasource_id.
	config := req.Config
	if isSQLToolType(req.Type) {
		config = normalizeDataSourceID(config)
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = ToSlug(req.Name)
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
		existing.Name = *req.Name
	}
	if req.Slug != nil {
		trimmed := strings.TrimSpace(*req.Slug)
		if trimmed != "" {
			existing.Slug = trimmed
		}
	}
	if req.Type != nil {
		existing.Type = *req.Type
	}
	if len(req.Config) > 0 {
		existing.Config = req.Config
	}
	if len(req.InputSchema) > 0 {
		existing.InputSchema = req.InputSchema
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Labels != nil {
		existing.Labels = req.Labels
	}
	if req.ReadOnly != nil {
		existing.ReadOnly = *req.ReadOnly
	}

	// P-C249-1: normalize SQL tool config on update as well.
	if isSQLToolType(existing.Type) {
		existing.Config = normalizeDataSourceID(existing.Config)
	}

	// P-C220-1 / P-C221-1 / P-C254-1: re-validate URL when type or config changed.
	// HTTP tools require a non-empty URL; reject updates that would leave it absent.
	if existing.Type == ToolTypeHTTP {
		u := extractURLFromConfig(existing.Config)
		if u == "" {
			return Response{}, fmt.Errorf("%w: url is required for HTTP tools", ErrValidation)
		}
		if err := ssrf.ValidateURL(u); err != nil {
			return Response{}, fmt.Errorf("%w: invalid URL (%v)", ErrValidation, err)
		}
	}

	t, err := s.repo.Update(ctx, id, existing)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(t), nil
}

// extractURLFromConfig extracts the "url" field from a JSON config blob.
// Returns empty string when the config is nil, unparseable, or has no url field.
func extractURLFromConfig(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var cfg struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return cfg.URL
}

// isSQLToolType returns true for SQL / DATABASE tool types.
func isSQLToolType(t ToolType) bool {
	return t == ToolTypeSQL || t == ToolTypeDatabase
}

// normalizeDataSourceID normalises camelCase `datasourceId` to snake_case `datasource_id`
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
	val, hasCamel := cfg["datasourceId"]
	if !hasCamel {
		return raw // nothing to do
	}
	// Move camelCase key to snake_case (keep both for compatibility).
	cfg["datasource_id"] = val
	delete(cfg, "datasourceId")
	normalized, err := json.Marshal(cfg)
	if err != nil {
		return raw
	}
	return normalized
}

// Delete deletes a tool.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
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

// TestTool executes a tool with the given inputs and returns the result as a string.
// Currently supports HTTP tool type; other types return a not-implemented error.
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
		return s.testHTTPTool(ctx, cfg, inputs)
	case ToolTypeSQL, ToolTypeDatabase:
		return "", fmt.Errorf("tool: SQL test requires a running datasource connection — coming soon")
	default:
		return "", fmt.Errorf("tool: test not supported for type %s", t.Type)
	}
}

func (s *Service) testHTTPTool(ctx context.Context, cfg, inputs map[string]any) (string, error) {
	rawURL, _ := cfg["url"].(string)
	if rawURL == "" {
		rawURL, _ = cfg["urlTemplate"].(string)
	}
	if rawURL == "" {
		return "", fmt.Errorf("tool: HTTP tool has no url configured")
	}
	method, _ := cfg["method"].(string)
	if method == "" {
		method = "GET"
	}

	// Render {key} and {{input.key}} placeholders in the URL from the test input.
	rawURL = renderPlaceholders(rawURL, inputs)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("tool: create HTTP request: %w", err)
	}

	if headers, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, renderPlaceholders(vs, inputs))
			}
		}
	}

	// Apply static auth from config (authType + authToken) so the test endpoint
	// mirrors what the skill-runtime executor does at runtime. P-C239-1:
	// authToken is never returned to the client, only sent on the outbound
	// request.
	authType, _ := cfg["authType"].(string)
	if authType == "" {
		authType, _ = cfg["auth_type"].(string)
	}
	authToken, _ := cfg["authToken"].(string)
	if authToken == "" {
		authToken, _ = cfg["auth_token"].(string)
	}
	switch strings.ToLower(strings.TrimSpace(authType)) {
	case "bearer":
		if authToken != "" {
			req.Header.Set("Authorization", "Bearer "+authToken)
		}
	case "basic":
		if authToken != "" {
			req.Header.Set("Authorization", "Basic "+authToken)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tool: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var buf strings.Builder
	fmt.Fprintf(&buf, "Status: %d %s\n\n", resp.StatusCode, resp.Status)
	respBody := make([]byte, 4096)
	n, _ := resp.Body.Read(respBody)
	buf.Write(respBody[:n])

	return buf.String(), nil
}

// renderPlaceholders substitutes {key} and {{input.key}} occurrences in s with
// the corresponding string value from inputs. Missing keys are left untouched
// so the caller can see the failure in the outbound URL/header.
func renderPlaceholders(s string, inputs map[string]any) string {
	if len(inputs) == 0 {
		return s
	}
	out := s
	for k, v := range inputs {
		var val string
		switch t := v.(type) {
		case string:
			val = t
		case fmt.Stringer:
			val = t.String()
		default:
			val = fmt.Sprintf("%v", t)
		}
		out = strings.ReplaceAll(out, "{"+k+"}", val)
		out = strings.ReplaceAll(out, "{{input."+k+"}}", val)
	}
	return out
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
		return DatabaseSchema{}, fmt.Errorf("tool: invalid dataSourceId: %w", err)
	}

	tenantID := s.tenantIDFn(ctx)
	creds, err := s.dsRdr.GetDatasourceCreds(ctx, tenantID, dsID)
	if err != nil {
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
