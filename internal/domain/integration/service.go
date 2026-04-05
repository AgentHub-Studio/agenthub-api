package integration

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

const catalogScanSize = 1000

type toolCatalog interface {
	List(ctx context.Context, req pagination.PageRequest, toolType string) (pagination.Page[tool.Response], error)
}

type datasourceCatalog interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]datasource.DataSource, int, error)
}

type mcpCatalog interface {
	List(ctx context.Context) ([]mcp.McpServerConfigResponse, error)
}

type vpnCatalog interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]vpnresource.VpnResource, int, error)
}

// Service aggregates legacy admin entities into a simplified integration catalog.
type Service struct {
	tools       toolCatalog
	datasources datasourceCatalog
	mcps        mcpCatalog
	vpns        vpnCatalog
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
		items = append(items, buildDatabaseIntegration(item, dbToolsByDataSource[item.ID], vpnByID))
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

func buildDatabaseIntegration(item datasource.DataSource, linkedTools []tool.Response, vpnByID map[uuid.UUID]vpnresource.VpnResource) Integration {
	advanced := len(linkedTools) != 1
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
