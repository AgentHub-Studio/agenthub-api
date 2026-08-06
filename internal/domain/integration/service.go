package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

const catalogScanSize = 1000
const generatedHTTPSkillCategory = "INTEGRATION_HTTP"
const generatedDatabaseSkillCategory = "INTEGRATION_DATABASE"

type toolCatalog interface {
	List(ctx context.Context, req pagination.PageRequest, toolType string) (pagination.Page[tool.Response], error)
}

type datasourceCatalog interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]datasource.DataSource, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (datasource.DataSource, error)
	Create(ctx context.Context, tenantID string, req datasource.CreateRequest) (datasource.DataSource, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req datasource.CreateRequest) (datasource.DataSource, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
}

type datasourceVPNReplacingUpdater interface {
	UpdateReplacingVPNResource(ctx context.Context, tenantID string, id uuid.UUID, req datasource.CreateRequest) (datasource.DataSource, error)
}

type mcpCatalog interface {
	List(ctx context.Context) ([]mcp.McpServerConfigResponse, error)
	Create(ctx context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (mcp.McpServerConfigResponse, error)
	Update(ctx context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type vpnCatalog interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]vpnresource.VpnResource, int, error)
}

type skillCreator interface {
	Create(ctx context.Context, req skill.CreateRequest) (skill.Response, error)
}

type httpToolManager interface {
	Create(ctx context.Context, req tool.CreateRequest) (tool.Response, error)
	GetByID(ctx context.Context, id uuid.UUID) (tool.Response, error)
	Update(ctx context.Context, id uuid.UUID, req tool.UpdateRequest) (tool.Response, error)
	Delete(ctx context.Context, id uuid.UUID) error
	BindToSkill(ctx context.Context, skillID uuid.UUID, req tool.BindRequest) (tool.SkillToolResponse, error)
	UnbindFromSkill(ctx context.Context, skillID, toolID uuid.UUID) error
}

// Service aggregates legacy admin entities into a simplified integration catalog.
type Service struct {
	tools       toolCatalog
	datasources datasourceCatalog
	mcps        mcpCatalog
	vpns        vpnCatalog
	skillMgmt   skillCreator
	httpTools   httpToolManager
	repo        managementRepository
}

// NewService creates a new integration catalog service.
func NewService(tools toolCatalog, datasources datasourceCatalog, mcps mcpCatalog, vpns vpnCatalog) *Service {
	return &Service{
		tools:       tools,
		datasources: datasources,
		mcps:        mcps,
		vpns:        vpns,
	}
}

// WithHTTPManagement wires the dependencies required by the simplified HTTP integration CRUD.
func (s *Service) WithHTTPManagement(skills skillCreator, tools httpToolManager, repo managementRepository) *Service {
	s.skillMgmt = skills
	s.httpTools = tools
	s.repo = repo
	return s
}

// List returns a paginated, filtered view of the unified integration catalog.
func (s *Service) List(ctx context.Context, req pagination.PageRequest, filters ListFilters) (pagination.Page[Response], error) {
	maxReq := pagination.PageRequest{Page: 0, Size: catalogScanSize}
	tenantID := tenant.FromContext(ctx)

	httpTools, err := s.tools.List(ctx, maxReq, string(tool.ToolTypeHTTP))
	if err != nil {
		return pagination.Page[Response]{}, fmt.Errorf("integration service: list HTTP tools: %w", err)
	}
	sqlTools, err := s.tools.List(ctx, maxReq, string(tool.ToolTypeSQL))
	if err != nil {
		return pagination.Page[Response]{}, fmt.Errorf("integration service: list SQL tools: %w", err)
	}
	databaseTools, err := s.tools.List(ctx, maxReq, string(tool.ToolTypeDatabase))
	if err != nil {
		return pagination.Page[Response]{}, fmt.Errorf("integration service: list DATABASE tools: %w", err)
	}
	datasources, _, err := s.datasources.ListAll(ctx, tenantID, maxReq)
	if err != nil {
		return pagination.Page[Response]{}, fmt.Errorf("integration service: list datasources: %w", err)
	}
	mcpConfigs, err := s.mcps.List(ctx)
	if err != nil {
		return pagination.Page[Response]{}, fmt.Errorf("integration service: list MCP configs: %w", err)
	}
	vpns, _, err := s.vpns.ListAll(ctx, tenantID, maxReq)
	if err != nil {
		return pagination.Page[Response]{}, fmt.Errorf("integration service: list VPN resources: %w", err)
	}

	vpnByID := make(map[uuid.UUID]vpnresource.VpnResource, len(vpns))
	for _, item := range vpns {
		vpnByID[item.ID] = item
	}

	items := make([]Integration, 0, len(httpTools.Content)+len(datasources)+len(mcpConfigs)+len(vpns))
	for _, item := range httpTools.Content {
		items = append(items, buildHTTPIntegration(item))
	}

	dbToolsByDataSource := make(map[uuid.UUID][]tool.Response)
	orphanDBTools := make([]tool.Response, 0)
	for _, item := range append(sqlTools.Content, databaseTools.Content...) {
		dataSourceID, ok := extractDataSourceID(item.Config)
		if !ok {
			orphanDBTools = append(orphanDBTools, item)
			continue
		}
		dbToolsByDataSource[dataSourceID] = append(dbToolsByDataSource[dataSourceID], item)
	}

	linkedVPNs := make(map[uuid.UUID]struct{})
	for _, item := range datasources {
		if item.VpnResourceID != nil {
			linkedVPNs[*item.VpnResourceID] = struct{}{}
		}
		linkedTools := dbToolsByDataSource[item.ID]
		advanced := len(linkedTools) != 1
		if len(linkedTools) == 1 && s.repo != nil {
			_, simple, err := s.simpleGeneratedDatabaseSkill(ctx, linkedTools[0].ID)
			if err != nil {
				return pagination.Page[Response]{}, err
			}
			advanced = !simple
		}
		items = append(items, buildDatabaseIntegration(item, linkedTools, vpnByID, advanced))
	}

	for _, item := range orphanDBTools {
		items = append(items, buildOrphanDatabaseToolIntegration(item))
	}

	for _, item := range vpns {
		if _, linked := linkedVPNs[item.ID]; linked {
			continue
		}
		items = append(items, buildVPNOnlyIntegration(item))
	}

	for _, item := range mcpConfigs {
		items = append(items, buildMCPIntegration(item))
	}

	filtered := filterItems(items, filters)
	sort.Slice(filtered, func(i, j int) bool {
		left := strings.ToLower(filtered[i].Name)
		right := strings.ToLower(filtered[j].Name)
		if left == right {
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		}
		return left < right
	})

	start := req.Offset()
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + req.Size
	if end > len(filtered) {
		end = len(filtered)
	}

	content := make([]Response, 0, end-start)
	for _, item := range filtered[start:end] {
		content = append(content, ResponseFrom(item))
	}

	return pagination.NewPage(content, int64(len(filtered)), req), nil
}

func filterItems(items []Integration, filters ListFilters) []Integration {
	filtered := make([]Integration, 0, len(items))
	for _, item := range items {
		if filters.Type != nil && item.Type != *filters.Type {
			continue
		}
		if filters.Enabled != nil && item.Enabled != *filters.Enabled {
			continue
		}
		if filters.Origin != nil && item.Origin != *filters.Origin {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (s *Service) GetDatabase(ctx context.Context, id uuid.UUID) (DatabaseResponse, error) {
	tenantID := tenant.FromContext(ctx)
	ds, err := s.datasources.GetByID(ctx, tenantID, id)
	if err != nil {
		return DatabaseResponse{}, err
	}
	linkedTool, description, err := s.findDatabaseToolForDatasource(ctx, id)
	if err != nil {
		return DatabaseResponse{}, err
	}
	if _, err := s.requireSimpleGeneratedDatabaseSkill(ctx, linkedTool.ID); err != nil {
		return DatabaseResponse{}, err
	}
	return databaseResponseFromDatasource(ds, linkedTool, description), nil
}

func (s *Service) GetMCP(ctx context.Context, id uuid.UUID) (mcp.McpServerConfigResponse, error) {
	return s.mcps.GetByID(ctx, id)
}

// CreateHTTP creates a simplified HTTP integration backed by a generated skill + tool pair.
func (s *Service) CreateHTTP(ctx context.Context, req HTTPCreateRequest) (HTTPResponse, error) {
	if err := validateHTTPRequest(req); err != nil {
		return HTTPResponse{}, err
	}
	if s.skillMgmt == nil || s.httpTools == nil || s.repo == nil {
		return HTTPResponse{}, errors.New("integration service: HTTP management not configured")
	}

	skillResp, err := s.skillMgmt.Create(ctx, skill.CreateRequest{
		Name:         req.Name,
		Description:  req.Description,
		Instructions: httpSkillInstructions(req),
		Category:     generatedHTTPSkillCategory,
	})
	if err != nil {
		return HTTPResponse{}, fmt.Errorf("integration service: create HTTP skill: %w", err)
	}

	toolResp, err := s.httpTools.Create(ctx, tool.CreateRequest{
		Name:        req.Name,
		Description: req.Description,
		Type:        tool.ToolTypeHTTP,
		Config:      buildHTTPConfig(req),
		ReadOnly:    req.ReadOnly,
	})
	if err != nil {
		_ = s.repo.DeleteSkill(ctx, skillResp.ID)
		return HTTPResponse{}, fmt.Errorf("integration service: create HTTP tool: %w", err)
	}

	_, err = s.httpTools.BindToSkill(ctx, skillResp.ID, tool.BindRequest{ToolID: toolResp.ID, Priority: 0})
	if err != nil {
		_ = s.httpTools.Delete(ctx, toolResp.ID)
		_ = s.repo.DeleteSkill(ctx, skillResp.ID)
		return HTTPResponse{}, fmt.Errorf("integration service: bind HTTP tool to skill: %w", err)
	}

	return httpResponseFromTool(toolResp)
}

// GetHTTP returns a simplified HTTP integration by its backing tool ID.
func (s *Service) GetHTTP(ctx context.Context, id uuid.UUID) (HTTPResponse, error) {
	if s.httpTools == nil {
		return HTTPResponse{}, errors.New("integration service: HTTP management not configured")
	}
	item, err := s.httpTools.GetByID(ctx, id)
	if err != nil {
		return HTTPResponse{}, err
	}
	if item.Type != tool.ToolTypeHTTP {
		return HTTPResponse{}, fmt.Errorf("integration service: tool %s is not an HTTP integration", id)
	}
	return httpResponseFromTool(item)
}

// UpdateHTTP updates the generated HTTP integration and its companion
// generated skill metadata when present. PATCH-friendly: GetByID
// primeiro, preserva campos vazios (true partial — backlog #216).
func (s *Service) UpdateHTTP(ctx context.Context, id uuid.UUID, req HTTPCreateRequest) (HTTPResponse, error) {
	if s.httpTools == nil || s.repo == nil {
		return HTTPResponse{}, errors.New("integration service: HTTP management not configured")
	}

	current, err := s.httpTools.GetByID(ctx, id)
	if err != nil {
		return HTTPResponse{}, err
	}
	if current.Type != tool.ToolTypeHTTP {
		return HTTPResponse{}, fmt.Errorf("integration service: tool %s is not an HTTP integration", id)
	}

	// Merge: campos vazios preservam estado atual.
	if strings.TrimSpace(req.Name) == "" {
		req.Name = current.Name
	}
	if strings.TrimSpace(req.URL) == "" {
		// Extrai URL atual da config — se já existia, mantém.
		if cfg, ok := current.Config.(map[string]any); ok {
			if url, ok := cfg["url"].(string); ok {
				req.URL = url
			}
		}
	}
	if err := validateHTTPRequest(req); err != nil {
		return HTTPResponse{}, err
	}

	httpType := tool.ToolType(tool.ToolTypeHTTP)
	updated, err := s.httpTools.Update(ctx, id, tool.UpdateRequest{
		Name:        &req.Name,
		Description: &req.Description,
		Type:        &httpType,
		Config:      buildHTTPConfig(req),
		ReadOnly:    &req.ReadOnly,
	})
	if err != nil {
		return HTTPResponse{}, fmt.Errorf("integration service: update HTTP tool: %w", err)
	}

	linkedSkills, err := s.repo.ListSkillsByToolID(ctx, id)
	if err != nil {
		return HTTPResponse{}, fmt.Errorf("integration service: load linked skills: %w", err)
	}
	for _, sk := range linkedSkills {
		if sk.Category != generatedHTTPSkillCategory {
			continue
		}
		if err := s.repo.UpdateSkillMetadata(ctx, sk.ID, req.Name, req.Description, httpSkillInstructions(req)); err != nil {
			return HTTPResponse{}, fmt.Errorf("integration service: update generated skill: %w", err)
		}
	}

	return httpResponseFromTool(updated)
}

// DeleteHTTP removes the HTTP tool and deletes generated skills that become orphaned after the unbind.
func (s *Service) DeleteHTTP(ctx context.Context, id uuid.UUID) error {
	if s.httpTools == nil || s.repo == nil {
		return errors.New("integration service: HTTP management not configured")
	}

	linkedSkills, err := s.repo.ListSkillsByToolID(ctx, id)
	if err != nil {
		return fmt.Errorf("integration service: load linked skills: %w", err)
	}

	for _, sk := range linkedSkills {
		if err := s.httpTools.UnbindFromSkill(ctx, sk.ID, id); err != nil && !errors.Is(err, tool.ErrNotFound) {
			return fmt.Errorf("integration service: unbind HTTP tool from skill: %w", err)
		}
	}
	if err := s.httpTools.Delete(ctx, id); err != nil {
		return fmt.Errorf("integration service: delete HTTP tool: %w", err)
	}

	for _, sk := range linkedSkills {
		if sk.Category != generatedHTTPSkillCategory {
			continue
		}
		bindings, err := s.repo.CountToolBindings(ctx, sk.ID)
		if err != nil {
			return fmt.Errorf("integration service: count generated skill bindings: %w", err)
		}
		if bindings == 0 {
			if err := s.repo.DeleteSkill(ctx, sk.ID); err != nil {
				return fmt.Errorf("integration service: delete generated skill: %w", err)
			}
		}
	}

	return nil
}

func validateHTTPRequest(req HTTPCreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("integration service: name is required")
	}
	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("integration service: url is required")
	}
	// Bug 164: cap URL em 2048 chars (RFC standard).
	if len(req.URL) > 2048 {
		return fmt.Errorf("integration service: url exceeds maximum length of 2048 chars (got %d)", len(req.URL))
	}
	// Bug 161: cap bodyTemplate em 32KB. Templates 200KB+ explodem o
	// HTTP request body cada vez que o tool executa.
	if len(req.BodyTemplate) > 32000 {
		return fmt.Errorf("integration service: bodyTemplate exceeds maximum length of 32000 chars (got %d)", len(req.BodyTemplate))
	}
	// Bug 167: cap headers count em 50. Integrations reais usam <10
	// headers; 1000+ é payload malformado e perf hit em request loop.
	if len(req.Headers) > 0 {
		var hm map[string]any
		if err := json.Unmarshal(req.Headers, &hm); err == nil {
			if len(hm) > 50 {
				return fmt.Errorf("integration service: headers exceeds maximum of 50 entries (got %d)", len(hm))
			}
			// Bug 250: validar header names (RFC 7230 token). Sem este gate,
			// keys com HTML (<script>) eram persistidas → XSS na UI Settings;
			// keys vazias ou com espaços viravam HTTP requests inválidos.
			for k := range hm {
				if !httpHeaderNamePattern.MatchString(k) {
					return fmt.Errorf("integration service: header name %q invalid — must match RFC 7230 token (alphanumeric + -_)", k)
				}
			}
		}
	}
	return nil
}

func httpSkillInstructions(req HTTPCreateRequest) string {
	return fmt.Sprintf("Use this skill to interact with the %s API. Describe the endpoint purpose and parameters here.", req.Name)
}

// httpHeaderNamePattern é um subset prático do RFC 7230 token:
// alphanumeric + hyphen + underscore. Cobre 99% dos headers reais
// (Authorization, X-Custom-Header, etc) sem permitir XSS chars.
var httpHeaderNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func buildHTTPConfig(req HTTPCreateRequest) json.RawMessage {
	payload := map[string]any{
		"url":          strings.TrimSpace(req.URL),
		"method":       normaliseHTTPMethod(req.Method),
		"bodyTemplate": req.BodyTemplate,
		"inputSchema":  decodeJSONOrNil(req.InputSchema),
	}
	if headers := decodeJSONOrNil(req.Headers); headers != nil {
		payload["headers"] = headers
	}
	if req.CredentialID != nil && strings.TrimSpace(*req.CredentialID) != "" {
		payload["credentialId"] = strings.TrimSpace(*req.CredentialID)
	}
	if responseMapping := decodeJSONOrNil(req.ResponseMapping); responseMapping != nil {
		payload["responseMapping"] = responseMapping
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func httpResponseFromTool(item tool.Response) (HTTPResponse, error) {
	config := asMap(item.Config)
	var inputSchema any
	if raw := config["inputSchema"]; raw != nil {
		inputSchema = raw
	}
	return HTTPResponse{
		ID:              item.ID,
		Name:            item.Name,
		Description:     item.Description,
		Method:          normaliseHTTPMethod(asString(config["method"])),
		URL:             asString(config["url"]),
		Headers:         config["headers"],
		BodyTemplate:    asString(config["bodyTemplate"]),
		CredentialID:    stringPtr(asString(config["credentialId"])),
		ResponseMapping: config["responseMapping"],
		InputSchema:     inputSchema,
		ReadOnly:        item.ReadOnly,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		LegacyPath:      legacyEditPath(SourceKindTool, item.ID),
	}, nil
}

func decodeJSONOrNil(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func normaliseHTTPMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return httpMethodDefault
	}
	return method
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func buildHTTPIntegration(item tool.Response) Integration {
	config := asMap(item.Config)
	method := strings.ToUpper(asString(config["method"]))
	if method == "" {
		method = httpMethodDefault
	}
	url := asString(config["url"])
	summary := strings.TrimSpace(method + " " + url)
	advanced := url == ""
	if summary == "" {
		summary = "HTTP tool"
	}

	return Integration{
		ID:          item.ID,
		Name:        item.Name,
		Slug:        buildSlug(item.Name, item.ID),
		Type:        IntegrationTypeHTTPAPI,
		Description: item.Description,
		Summary:     summary,
		Enabled:     true,
		Advanced:    advanced,
		Origin:      IntegrationOriginLegacy,
		SourceKind:  SourceKindTool,
		LegacyPath:  legacyEditPath(SourceKindTool, item.ID),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func buildDatabaseIntegration(item datasource.DataSource, linkedTools []tool.Response, vpnByID map[uuid.UUID]vpnresource.VpnResource, advanced bool) Integration {
	vpnSummary := "direct"
	enabled := true
	if item.VpnResourceID != nil {
		if vpn, ok := vpnByID[*item.VpnResourceID]; ok {
			vpnSummary = "vpn:" + vpn.Name
			enabled = vpn.Enabled
		} else {
			vpnSummary = "vpn:unknown"
		}
	}

	summary := fmt.Sprintf("%s %s:%d/%s • %s", item.Type, item.Host, item.Port, item.Database, vpnSummary)
	if len(linkedTools) == 1 {
		summary += " • tool:" + linkedTools[0].Name
	}
	if len(linkedTools) > 1 {
		summary += " • linkedTools:" + strconv.Itoa(len(linkedTools))
	}
	if len(linkedTools) == 0 {
		summary += " • no-linked-tool"
	}

	description := ""
	if len(linkedTools) == 1 {
		description = linkedTools[0].Description
	}

	return Integration{
		ID:          item.ID,
		Name:        item.Name,
		Slug:        buildSlug(item.Name, item.ID),
		Type:        IntegrationTypeDatabaseQuery,
		Description: description,
		Summary:     summary,
		Enabled:     enabled,
		Advanced:    advanced,
		Origin:      IntegrationOriginLegacy,
		SourceKind:  SourceKindDatasource,
		LegacyPath:  legacyEditPath(SourceKindDatasource, item.ID),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func buildOrphanDatabaseToolIntegration(item tool.Response) Integration {
	return Integration{
		ID:          item.ID,
		Name:        item.Name,
		Slug:        buildSlug(item.Name, item.ID),
		Type:        IntegrationTypeDatabaseQuery,
		Description: item.Description,
		Summary:     "Database tool without mapped datasource",
		Enabled:     true,
		Advanced:    true,
		Origin:      IntegrationOriginLegacy,
		SourceKind:  SourceKindTool,
		LegacyPath:  legacyEditPath(SourceKindTool, item.ID),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func buildVPNOnlyIntegration(item vpnresource.VpnResource) Integration {
	return Integration{
		ID:          item.ID,
		Name:        item.Name,
		Slug:        buildSlug(item.Name, item.ID),
		Type:        IntegrationTypeDatabaseQuery,
		Description: item.Description,
		Summary:     "VPN resource without linked datasource",
		Enabled:     item.Enabled,
		Advanced:    true,
		Origin:      IntegrationOriginLegacy,
		SourceKind:  SourceKindVPN,
		LegacyPath:  legacyEditPath(SourceKindVPN, item.ID),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func buildMCPIntegration(item mcp.McpServerConfigResponse) Integration {
	summary := item.TransportType
	advanced := false
	if item.TransportType == "http" || item.TransportType == "HTTP" {
		if item.HTTPBaseURL != nil && *item.HTTPBaseURL != "" {
			summary += " • " + *item.HTTPBaseURL
		} else {
			advanced = true
		}
	} else if item.Command != nil && *item.Command != "" {
		summary += " • " + *item.Command
	}

	return Integration{
		ID:          item.ID,
		Name:        item.Name,
		Slug:        buildSlug(item.Name, item.ID),
		Type:        IntegrationTypeMCP,
		Description: "",
		Summary:     summary,
		Enabled:     item.Enabled,
		Advanced:    advanced,
		Origin:      IntegrationOriginLegacy,
		SourceKind:  SourceKindMCP,
		LegacyPath:  legacyEditPath(SourceKindMCP, item.ID),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func extractDataSourceID(config any) (uuid.UUID, bool) {
	configMap := asMap(config)
	raw := asString(configMap["dataSourceId"])
	if raw == "" {
		raw = asString(configMap["datasource_id"])
	}
	if raw == "" {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}

func asMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func asString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func buildDatabaseConfig(datasourceID uuid.UUID, req DatabaseCreateRequest) json.RawMessage {
	operation := "SELECT"
	if req.AllowWrite {
		operation = "EXEC"
	}
	payload := map[string]any{
		"dataSourceId":  datasourceID.String(),
		"datasource_id": datasourceID.String(),
		"sql":           req.Query,
		"query":         req.Query,
		"operation":     operation,
		"max_rows":      100,
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func validateDatabaseRequest(req DatabaseCreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("integration service: name is required")
	}
	switch req.Type {
	case datasource.DataSourceTypePostgreSQL, datasource.DataSourceTypeMySQL, datasource.DataSourceTypeSQLServer:
	default:
		return fmt.Errorf("integration service: unsupported type: %s", req.Type)
	}
	if strings.TrimSpace(req.Host) == "" {
		return fmt.Errorf("integration service: host is required")
	}
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("integration service: port must be between 1 and 65535 (got %d)", req.Port)
	}
	if strings.TrimSpace(req.Database) == "" {
		return fmt.Errorf("integration service: database is required")
	}
	if strings.TrimSpace(req.DBUser) == "" {
		return fmt.Errorf("integration service: dbUser is required")
	}
	if strings.TrimSpace(req.Query) == "" {
		return fmt.Errorf("integration service: query is required")
	}
	return nil
}

func databaseSkillInstructions(req DatabaseCreateRequest) string {
	operation := "read"
	if req.AllowWrite {
		operation = "execute"
	}
	return fmt.Sprintf("Use the generated SQL tool to %s the configured %s database. Default query: %s",
		operation,
		strings.ToLower(string(req.Type)),
		strings.TrimSpace(req.Query),
	)
}

func databaseResponseFromDatasource(ds datasource.DataSource, sqlTool tool.Response, description string) DatabaseResponse {
	config := asMap(sqlTool.Config)
	query := asString(config["query"])
	if query == "" {
		query = asString(config["sql"])
	}
	operation := strings.ToUpper(asString(config["operation"]))
	allowWrite := operation != "" && operation != "SELECT"
	return DatabaseResponse{
		ID:            ds.ID,
		Name:          ds.Name,
		Description:   description,
		Type:          ds.Type,
		Host:          ds.Host,
		Port:          ds.Port,
		Database:      ds.Database,
		DBUser:        ds.DBUser,
		VpnResourceID: ds.VpnResourceID,
		Query:         query,
		AllowWrite:    allowWrite,
		CreatedAt:     ds.CreatedAt,
		UpdatedAt:     ds.UpdatedAt,
		LegacyPath:    legacyEditPath(SourceKindDatasource, ds.ID),
	}
}

func (s *Service) findDatabaseToolForDatasource(ctx context.Context, datasourceID uuid.UUID) (tool.Response, string, error) {
	maxReq := pagination.PageRequest{Page: 0, Size: catalogScanSize}
	sqlTools, err := s.tools.List(ctx, maxReq, string(tool.ToolTypeSQL))
	if err != nil {
		return tool.Response{}, "", fmt.Errorf("integration service: list SQL tools: %w", err)
	}
	databaseTools, err := s.tools.List(ctx, maxReq, string(tool.ToolTypeDatabase))
	if err != nil {
		return tool.Response{}, "", fmt.Errorf("integration service: list DATABASE tools: %w", err)
	}
	matches := make([]tool.Response, 0, 2)
	for _, item := range append(sqlTools.Content, databaseTools.Content...) {
		id, ok := extractDataSourceID(item.Config)
		if !ok || id != datasourceID {
			continue
		}
		matches = append(matches, item)
	}
	switch len(matches) {
	case 0:
		return tool.Response{}, "", tool.ErrNotFound
	case 1:
		return matches[0], matches[0].Description, nil
	default:
		return tool.Response{}, "", fmt.Errorf("integration service: multiple database tools linked to datasource %s", datasourceID)
	}
}

func (s *Service) updateDatabaseDatasource(ctx context.Context, tenantID string, id uuid.UUID, req datasource.CreateRequest) (datasource.DataSource, error) {
	if updater, ok := s.datasources.(datasourceVPNReplacingUpdater); ok {
		return updater.UpdateReplacingVPNResource(ctx, tenantID, id, req)
	}
	return s.datasources.Update(ctx, tenantID, id, req)
}

func errMissingDatabaseTool(datasourceID uuid.UUID) error {
	return fmt.Errorf("integration service: no linked database tool for datasource %s", datasourceID)
}

func (s *Service) simpleGeneratedDatabaseSkill(ctx context.Context, toolID uuid.UUID) (GeneratedSkill, bool, error) {
	if s.repo == nil {
		return GeneratedSkill{}, true, nil
	}
	linkedSkills, err := s.repo.ListSkillsByToolID(ctx, toolID)
	if err != nil {
		return GeneratedSkill{}, false, fmt.Errorf("integration service: load linked skills: %w", err)
	}
	if len(linkedSkills) != 1 || linkedSkills[0].Category != generatedDatabaseSkillCategory {
		return GeneratedSkill{}, false, nil
	}
	return linkedSkills[0], true, nil
}

func (s *Service) requireSimpleGeneratedDatabaseSkill(ctx context.Context, toolID uuid.UUID) (GeneratedSkill, error) {
	generatedSkill, simple, err := s.simpleGeneratedDatabaseSkill(ctx, toolID)
	if err != nil {
		return GeneratedSkill{}, err
	}
	if !simple {
		return GeneratedSkill{}, fmt.Errorf("integration service: database tool %s is not linked to exactly one generated database skill", toolID)
	}
	return generatedSkill, nil
}

func buildSlug(name string, id uuid.UUID) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug != "" {
		return slug
	}
	return id.String()[:8]
}

func legacyEditPath(kind SourceKind, id uuid.UUID) string {
	switch kind {
	case SourceKindDatasource:
		return "datasources/edit/" + id.String()
	case SourceKindMCP:
		return "mcp-server-configs/edit/" + id.String()
	case SourceKindVPN:
		return "vpns/edit/" + id.String()
	default:
		return "tools/edit/" + id.String()
	}
}

const httpMethodDefault = "GET"

// CreateMCP delegates to the underlying MCP catalog.
func (s *Service) CreateMCP(ctx context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error) {
	return s.mcps.Create(ctx, req)
}

// UpdateMCP delegates to the underlying MCP catalog.
func (s *Service) UpdateMCP(ctx context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error) {
	return s.mcps.Update(ctx, id, req)
}

// DeleteMCP delegates to the underlying MCP catalog.
func (s *Service) DeleteMCP(ctx context.Context, id uuid.UUID) error {
	return s.mcps.Delete(ctx, id)
}

// CreateDatabase creates a datasource and generates a companion skill + SQL tool.
func (s *Service) CreateDatabase(ctx context.Context, req DatabaseCreateRequest) (DatabaseResponse, error) {
	if err := validateDatabaseRequest(req); err != nil {
		return DatabaseResponse{}, err
	}
	if s.skillMgmt == nil || s.httpTools == nil || s.repo == nil {
		return DatabaseResponse{}, errors.New("integration service: HTTP management not configured")
	}

	tenantID := tenant.FromContext(ctx)
	ds, err := s.datasources.Create(ctx, tenantID, datasource.CreateRequest{
		Name:          req.Name,
		Type:          req.Type,
		Host:          req.Host,
		Port:          req.Port,
		Database:      req.Database,
		DBUser:        req.DBUser,
		DBPassword:    req.DBPassword,
		VpnResourceID: req.VpnResourceID,
	})
	if err != nil {
		return DatabaseResponse{}, fmt.Errorf("integration service: create datasource: %w", err)
	}

	skillResp, err := s.skillMgmt.Create(ctx, skill.CreateRequest{
		Name:         req.Name,
		Description:  req.Description,
		Instructions: databaseSkillInstructions(req),
		Category:     generatedDatabaseSkillCategory,
	})
	if err != nil {
		_ = s.datasources.Delete(ctx, tenantID, ds.ID)
		return DatabaseResponse{}, fmt.Errorf("integration service: create database skill: %w", err)
	}

	toolResp, err := s.httpTools.Create(ctx, tool.CreateRequest{
		Name:        req.Name,
		Description: req.Description,
		Type:        tool.ToolTypeSQL,
		Config:      buildDatabaseConfig(ds.ID, req),
	})
	if err != nil {
		_ = s.repo.DeleteSkill(ctx, skillResp.ID)
		_ = s.datasources.Delete(ctx, tenantID, ds.ID)
		return DatabaseResponse{}, fmt.Errorf("integration service: create SQL tool: %w", err)
	}

	_, err = s.httpTools.BindToSkill(ctx, skillResp.ID, tool.BindRequest{ToolID: toolResp.ID, Priority: 0})
	if err != nil {
		_ = s.httpTools.Delete(ctx, toolResp.ID)
		_ = s.repo.DeleteSkill(ctx, skillResp.ID)
		_ = s.datasources.Delete(ctx, tenantID, ds.ID)
		return DatabaseResponse{}, fmt.Errorf("integration service: bind SQL tool to skill: %w", err)
	}

	return databaseResponseFromDatasource(ds, toolResp, req.Description), nil
}

// UpdateDatabase updates the datasource and its generated SQL tool and skill metadata.
func (s *Service) UpdateDatabase(ctx context.Context, id uuid.UUID, req DatabaseCreateRequest) (DatabaseResponse, error) {
	if err := validateDatabaseRequest(req); err != nil {
		return DatabaseResponse{}, err
	}
	if s.httpTools == nil || s.repo == nil {
		return DatabaseResponse{}, errors.New("integration service: HTTP management not configured")
	}

	tenantID := tenant.FromContext(ctx)

	if _, err := s.datasources.GetByID(ctx, tenantID, id); err != nil {
		return DatabaseResponse{}, fmt.Errorf("integration service: load datasource: %w", err)
	}

	linkedTool, description, findErr := s.findDatabaseToolForDatasource(ctx, id)
	if errors.Is(findErr, tool.ErrNotFound) {
		return DatabaseResponse{}, errMissingDatabaseTool(id)
	}
	if findErr != nil && !errors.Is(findErr, tool.ErrNotFound) {
		return DatabaseResponse{}, fmt.Errorf("integration service: find database tool: %w", findErr)
	}
	generatedSkill, err := s.requireSimpleGeneratedDatabaseSkill(ctx, linkedTool.ID)
	if err != nil {
		return DatabaseResponse{}, err
	}

	_, err = s.updateDatabaseDatasource(ctx, tenantID, id, datasource.CreateRequest{
		Name:          req.Name,
		Type:          req.Type,
		Host:          req.Host,
		Port:          req.Port,
		Database:      req.Database,
		DBUser:        req.DBUser,
		DBPassword:    req.DBPassword,
		VpnResourceID: req.VpnResourceID,
	})
	if err != nil {
		return DatabaseResponse{}, fmt.Errorf("integration service: update datasource: %w", err)
	}

	if findErr == nil {
		sqlType := tool.ToolType(tool.ToolTypeSQL)
		updated, err := s.httpTools.Update(ctx, linkedTool.ID, tool.UpdateRequest{
			Name:        &req.Name,
			Description: &req.Description,
			Type:        &sqlType,
			Config:      buildDatabaseConfig(id, req),
		})
		if err != nil {
			return DatabaseResponse{}, fmt.Errorf("integration service: update SQL tool: %w", err)
		}
		linkedTool = updated
		description = req.Description

		if err := s.repo.UpdateSkillMetadata(ctx, generatedSkill.ID, req.Name, req.Description, databaseSkillInstructions(req)); err != nil {
			return DatabaseResponse{}, fmt.Errorf("integration service: update generated skill: %w", err)
		}
	}

	ds, err := s.datasources.GetByID(ctx, tenantID, id)
	if err != nil {
		return DatabaseResponse{}, fmt.Errorf("integration service: reload datasource: %w", err)
	}

	return databaseResponseFromDatasource(ds, linkedTool, description), nil
}

// DeleteDatabase removes the datasource and cleans up generated SQL tool and skill.
func (s *Service) DeleteDatabase(ctx context.Context, id uuid.UUID) error {
	if s.httpTools == nil || s.repo == nil {
		return errors.New("integration service: HTTP management not configured")
	}

	tenantID := tenant.FromContext(ctx)

	if _, err := s.datasources.GetByID(ctx, tenantID, id); err != nil {
		return fmt.Errorf("integration service: load datasource: %w", err)
	}

	linkedTool, _, err := s.findDatabaseToolForDatasource(ctx, id)
	if errors.Is(err, tool.ErrNotFound) {
		return errMissingDatabaseTool(id)
	}
	if err != nil && !errors.Is(err, tool.ErrNotFound) {
		return fmt.Errorf("integration service: find database tool: %w", err)
	}
	generatedSkill, err := s.requireSimpleGeneratedDatabaseSkill(ctx, linkedTool.ID)
	if err != nil {
		return err
	}

	if err == nil {
		if err := s.httpTools.UnbindFromSkill(ctx, generatedSkill.ID, linkedTool.ID); err != nil && !errors.Is(err, tool.ErrNotFound) {
			return fmt.Errorf("integration service: unbind SQL tool from skill: %w", err)
		}
		if err := s.httpTools.Delete(ctx, linkedTool.ID); err != nil {
			return fmt.Errorf("integration service: delete SQL tool: %w", err)
		}
		bindings, err := s.repo.CountToolBindings(ctx, generatedSkill.ID)
		if err != nil {
			return fmt.Errorf("integration service: count generated skill bindings: %w", err)
		}
		if bindings == 0 {
			if err := s.repo.DeleteSkill(ctx, generatedSkill.ID); err != nil {
				return fmt.Errorf("integration service: delete generated skill: %w", err)
			}
		}
	}

	if err := s.datasources.Delete(ctx, tenantID, id); err != nil {
		return fmt.Errorf("integration service: delete datasource: %w", err)
	}

	return nil
}
