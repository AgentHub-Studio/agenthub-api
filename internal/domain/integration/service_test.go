package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type stubToolCatalog struct {
	pages map[string]pagination.Page[tool.Response]
}

func (s *stubToolCatalog) List(_ context.Context, _ pagination.PageRequest, toolType string) (pagination.Page[tool.Response], error) {
	if page, ok := s.pages[toolType]; ok {
		return page, nil
	}
	return pagination.NewPage([]tool.Response{}, 0, pagination.PageRequest{Page: 0, Size: 1000}), nil
}

type stubDatasourceCatalog struct{ items []datasource.DataSource }

func (s *stubDatasourceCatalog) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]datasource.DataSource, int, error) {
	return s.items, len(s.items), nil
}

type stubMCPCatalog struct{ items []mcp.McpServerConfigResponse }

func (s *stubMCPCatalog) List(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	return s.items, nil
}

type stubVPNCatalog struct{ items []vpnresource.VpnResource }

func (s *stubVPNCatalog) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]vpnresource.VpnResource, int, error) {
	return s.items, len(s.items), nil
}

func TestService_List_AggregatesLegacySources(t *testing.T) {
	now := time.Now().UTC()
	dsID := uuid.New()
	vpnID := uuid.New()
	httpID := uuid.New()
	mcpID := uuid.New()

	svc := integration.NewService(
		&stubToolCatalog{pages: map[string]pagination.Page[tool.Response]{
			string(tool.ToolTypeHTTP): pagination.NewPage([]tool.Response{{
				ID:          httpID,
				Name:        "ERP API",
				Type:        tool.ToolTypeHTTP,
				Description: "Sync customer records",
				Config:      map[string]any{"method": "POST", "url": "https://erp.example.com/customers"},
				CreatedAt:   now,
				UpdatedAt:   now,
			}}, 1, pagination.PageRequest{Page: 0, Size: 1000}),
			string(tool.ToolTypeSQL): pagination.NewPage([]tool.Response{{
				ID:          uuid.New(),
				Name:        "Orders Query",
				Type:        tool.ToolTypeSQL,
				Description: "Read order data",
				Config:      map[string]any{"dataSourceId": dsID.String()},
				CreatedAt:   now,
				UpdatedAt:   now,
			}}, 1, pagination.PageRequest{Page: 0, Size: 1000}),
		}},
		&stubDatasourceCatalog{items: []datasource.DataSource{{
			ID:            dsID,
			Name:          "Orders DB",
			Type:          datasource.DataSourceTypePostgreSQL,
			Host:          "pg.internal",
			Port:          5432,
			Database:      "orders",
			VpnResourceID: &vpnID,
			CreatedAt:     now,
			UpdatedAt:     now,
		}}},
		&stubMCPCatalog{items: []mcp.McpServerConfigResponse{{
			ID:            mcpID,
			Name:          "filesystem",
			TransportType: "stdio",
			Enabled:       true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}}},
		&stubVPNCatalog{items: []vpnresource.VpnResource{{
			ID:        vpnID,
			Name:      "Corp VPN",
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		}}},
	)

	ctx := tenantctx.NewContext(context.Background(), "test-tenant")
	page, err := svc.List(ctx, pagination.PageRequest{Page: 0, Size: 20}, integration.ListFilters{})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)

	itemsByType := make(map[integration.IntegrationType][]integration.Response)
	for _, item := range page.Content {
		itemsByType[item.Type] = append(itemsByType[item.Type], item)
	}

	require.Len(t, itemsByType[integration.IntegrationTypeHTTPAPI], 1)
	assert.Equal(t, "POST https://erp.example.com/customers", itemsByType[integration.IntegrationTypeHTTPAPI][0].Summary)
	assert.False(t, itemsByType[integration.IntegrationTypeHTTPAPI][0].Advanced)

	require.Len(t, itemsByType[integration.IntegrationTypeDatabaseQuery], 1)
	assert.Contains(t, itemsByType[integration.IntegrationTypeDatabaseQuery][0].Summary, "vpn:Corp VPN")
	assert.False(t, itemsByType[integration.IntegrationTypeDatabaseQuery][0].Advanced)
	assert.Equal(t, integration.SourceKindDatasource, itemsByType[integration.IntegrationTypeDatabaseQuery][0].SourceKind)

	require.Len(t, itemsByType[integration.IntegrationTypeMCP], 1)
	assert.Equal(t, integration.SourceKindMCP, itemsByType[integration.IntegrationTypeMCP][0].SourceKind)
	assert.Equal(t, integration.IntegrationOriginLegacy, itemsByType[integration.IntegrationTypeMCP][0].Origin)
}

func TestService_List_MarksStandaloneVPNAndOrphanDatabaseToolAsAdvanced(t *testing.T) {
	now := time.Now().UTC()
	orphanToolID := uuid.New()
	vpnID := uuid.New()

	svc := integration.NewService(
		&stubToolCatalog{pages: map[string]pagination.Page[tool.Response]{
			string(tool.ToolTypeDatabase): pagination.NewPage([]tool.Response{{
				ID:          orphanToolID,
				Name:        "Legacy SQL Tool",
				Type:        tool.ToolTypeDatabase,
				Description: "Legacy DB config",
				Config:      map[string]any{"sql": "select 1"},
				CreatedAt:   now,
				UpdatedAt:   now,
			}}, 1, pagination.PageRequest{Page: 0, Size: 1000}),
		}},
		&stubDatasourceCatalog{},
		&stubMCPCatalog{},
		&stubVPNCatalog{items: []vpnresource.VpnResource{{
			ID:          vpnID,
			Name:        "Standalone VPN",
			Description: "VPN without datasource",
			Enabled:     false,
			CreatedAt:   now,
			UpdatedAt:   now,
		}}},
	)

	ctx := tenantctx.NewContext(context.Background(), "test-tenant")
	page, err := svc.List(ctx, pagination.PageRequest{Page: 0, Size: 20}, integration.ListFilters{})
	require.NoError(t, err)
	assert.Len(t, page.Content, 2)

	for _, item := range page.Content {
		assert.True(t, item.Advanced)
		assert.Equal(t, integration.IntegrationTypeDatabaseQuery, item.Type)
	}
}

func TestService_List_AppliesFilters(t *testing.T) {
	now := time.Now().UTC()
	falseValue := false
	origin := integration.IntegrationOriginLegacy
	typeFilter := integration.IntegrationTypeMCP

	httpPage := pagination.NewPage([]tool.Response{{
		ID:        uuid.New(),
		Name:      "HTTP One",
		Type:      tool.ToolTypeHTTP,
		Config:    map[string]any{"url": "https://example.com"},
		CreatedAt: now,
		UpdatedAt: now,
	}}, 1, pagination.PageRequest{Page: 0, Size: 1000})
	httpURL := "https://mcp.example.com"
	mcpItems := []mcp.McpServerConfigResponse{{
		ID:            uuid.New(),
		Name:          "Remote MCP",
		TransportType: "http",
		HTTPBaseURL:   &httpURL,
		Enabled:       false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}}

	svc := integration.NewService(
		&stubToolCatalog{pages: map[string]pagination.Page[tool.Response]{string(tool.ToolTypeHTTP): httpPage}},
		&stubDatasourceCatalog{},
		&stubMCPCatalog{items: mcpItems},
		&stubVPNCatalog{},
	)

	ctx := tenantctx.NewContext(context.Background(), "test-tenant")
	page, err := svc.List(ctx, pagination.PageRequest{Page: 0, Size: 20}, integration.ListFilters{
		Type:    &typeFilter,
		Enabled: &falseValue,
		Origin:  &origin,
	})
	require.NoError(t, err)
	require.Len(t, page.Content, 1)
	assert.Equal(t, integration.IntegrationTypeMCP, page.Content[0].Type)
	assert.False(t, page.Content[0].Enabled)
	assert.Equal(t, integration.IntegrationOriginLegacy, page.Content[0].Origin)
}
