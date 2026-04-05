package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
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

type stubMCPCatalog struct {
	items   []mcp.McpServerConfigResponse
	created mcp.CreateRequest
	updated mcp.UpdateRequest
	deleted uuid.UUID
}

func (s *stubMCPCatalog) List(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	return s.items, nil
}

func (s *stubMCPCatalog) Create(_ context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error) {
	s.created = req
	item := mcp.McpServerConfigResponse{ID: uuid.New(), Name: req.Name, TransportType: req.TransportType, Enabled: req.Enabled}
	s.items = append(s.items, item)
	return item, nil
}

func (s *stubMCPCatalog) GetByID(_ context.Context, id uuid.UUID) (mcp.McpServerConfigResponse, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
	return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
}

func (s *stubMCPCatalog) Update(_ context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error) {
	s.updated = req
	for i, item := range s.items {
		if item.ID != id {
			continue
		}
		if req.Name != nil {
			item.Name = *req.Name
		}
		s.items[i] = item
		return item, nil
	}
	return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
}

func (s *stubMCPCatalog) Delete(_ context.Context, id uuid.UUID) error {
	s.deleted = id
	return nil
}

type stubVPNCatalog struct{ items []vpnresource.VpnResource }

func (s *stubVPNCatalog) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]vpnresource.VpnResource, int, error) {
	return s.items, len(s.items), nil
}

type stubSkillCreator struct {
	created []skill.CreateRequest
	resp    skill.Response
}

func (s *stubSkillCreator) Create(_ context.Context, req skill.CreateRequest) (skill.Response, error) {
	s.created = append(s.created, req)
	if s.resp.ID == uuid.Nil {
		s.resp = skill.Response{ID: uuid.New(), Name: req.Name, Description: req.Description, Category: req.Category}
	}
	return s.resp, nil
}

type stubHTTPToolManager struct {
	created []tool.CreateRequest
	updated []tool.UpdateRequest
	deleted []uuid.UUID
	bound   []uuid.UUID
	unbound []uuid.UUID
	item    tool.Response
}

func (s *stubHTTPToolManager) Create(_ context.Context, req tool.CreateRequest) (tool.Response, error) {
	s.created = append(s.created, req)
	if s.item.ID == uuid.Nil {
		s.item = tool.Response{
			ID:          uuid.New(),
			Name:        req.Name,
			Type:        req.Type,
			Config:      map[string]any{"url": "https://api.example.com", "method": "POST"},
			Description: req.Description,
			ReadOnly:    req.ReadOnly,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
	}
	return s.item, nil
}

func (s *stubHTTPToolManager) GetByID(_ context.Context, _ uuid.UUID) (tool.Response, error) {
	if s.item.ID == uuid.Nil {
		return tool.Response{}, tool.ErrNotFound
	}
	return s.item, nil
}

func (s *stubHTTPToolManager) Update(_ context.Context, _ uuid.UUID, req tool.UpdateRequest) (tool.Response, error) {
	s.updated = append(s.updated, req)
	var cfg map[string]any
	_ = json.Unmarshal(req.Config, &cfg)
	s.item = tool.Response{
		ID:          s.item.ID,
		Name:        req.Name,
		Type:        req.Type,
		Config:      cfg,
		Description: req.Description,
		ReadOnly:    req.ReadOnly,
		CreatedAt:   s.item.CreatedAt,
		UpdatedAt:   time.Now().UTC(),
	}
	return s.item, nil
}

func (s *stubHTTPToolManager) Delete(_ context.Context, id uuid.UUID) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *stubHTTPToolManager) BindToSkill(_ context.Context, skillID uuid.UUID, req tool.BindRequest) (tool.SkillToolResponse, error) {
	s.bound = append(s.bound, skillID)
	return tool.SkillToolResponse{SkillID: skillID, Tool: s.item}, nil
}

func (s *stubHTTPToolManager) UnbindFromSkill(_ context.Context, skillID, toolID uuid.UUID) error {
	s.unbound = append(s.unbound, skillID)
	return nil
}

type stubManagementRepo struct {
	skills  []generatedSkillFixture
	deleted []uuid.UUID
	updated []uuid.UUID
	counts  map[uuid.UUID]int
}

type generatedSkillFixture struct {
	id          uuid.UUID
	name        string
	description string
	category    string
}

func (s *stubManagementRepo) ListSkillsByToolID(_ context.Context, _ uuid.UUID) ([]integrationGeneratedSkill, error) {
	out := make([]integrationGeneratedSkill, 0, len(s.skills))
	for _, item := range s.skills {
		out = append(out, integrationGeneratedSkill{ID: item.id, Name: item.name, Description: item.description, Category: item.category})
	}
	return out, nil
}

func (s *stubManagementRepo) UpdateSkillMetadata(_ context.Context, skillID uuid.UUID, _, _ string) error {
	s.updated = append(s.updated, skillID)
	return nil
}

func (s *stubManagementRepo) CountToolBindings(_ context.Context, skillID uuid.UUID) (int, error) {
	if s.counts == nil {
		return 0, nil
	}
	return s.counts[skillID], nil
}

func (s *stubManagementRepo) DeleteSkill(_ context.Context, skillID uuid.UUID) error {
	s.deleted = append(s.deleted, skillID)
	return nil
}

type integrationGeneratedSkill = integration.GeneratedSkill

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

func TestService_CreateHTTP_GeneratesSkillAndTool(t *testing.T) {
	skillCreator := &stubSkillCreator{resp: skill.Response{ID: uuid.New()}}
	toolManager := &stubHTTPToolManager{}
	repo := &stubManagementRepo{}
	svc := integration.NewService(&stubToolCatalog{}, &stubDatasourceCatalog{}, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(skillCreator, toolManager, repo)

	resp, err := svc.CreateHTTP(context.Background(), integration.HTTPCreateRequest{
		Name:        "ERP API",
		Description: "Sync customers",
		Method:      "post",
		URL:         "https://api.example.com/customers",
		ReadOnly:    false,
	})
	require.NoError(t, err)
	assert.Equal(t, "ERP API", resp.Name)
	assert.Len(t, skillCreator.created, 1)
	assert.Equal(t, "INTEGRATION_HTTP", skillCreator.created[0].Category)
	assert.Len(t, toolManager.created, 1)
	assert.Equal(t, tool.ToolTypeHTTP, toolManager.created[0].Type)
	assert.Len(t, toolManager.bound, 1)
}

func TestService_UpdateHTTP_UpdatesGeneratedSkillMetadata(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()
	toolManager := &stubHTTPToolManager{item: tool.Response{
		ID:          toolID,
		Name:        "Old ERP API",
		Type:        tool.ToolTypeHTTP,
		Config:      map[string]any{"url": "https://old.example.com", "method": "GET"},
		Description: "old",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}}
	repo := &stubManagementRepo{skills: []generatedSkillFixture{{id: skillID, category: "INTEGRATION_HTTP"}}}
	svc := integration.NewService(&stubToolCatalog{}, &stubDatasourceCatalog{}, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(&stubSkillCreator{}, toolManager, repo)

	resp, err := svc.UpdateHTTP(context.Background(), toolID, integration.HTTPCreateRequest{
		Name:        "ERP API Updated",
		Description: "new",
		Method:      "PATCH",
		URL:         "https://api.example.com/customers",
	})
	require.NoError(t, err)
	assert.Equal(t, "ERP API Updated", resp.Name)
	assert.Len(t, toolManager.updated, 1)
	assert.Equal(t, []uuid.UUID{skillID}, repo.updated)
}

func TestService_DeleteHTTP_RemovesGeneratedOrphanSkills(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()
	toolManager := &stubHTTPToolManager{item: tool.Response{ID: toolID, Type: tool.ToolTypeHTTP}}
	repo := &stubManagementRepo{
		skills: []generatedSkillFixture{{id: skillID, category: "INTEGRATION_HTTP"}},
		counts: map[uuid.UUID]int{skillID: 0},
	}
	svc := integration.NewService(&stubToolCatalog{}, &stubDatasourceCatalog{}, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(&stubSkillCreator{}, toolManager, repo)

	err := svc.DeleteHTTP(context.Background(), toolID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{skillID}, toolManager.unbound)
	assert.Equal(t, []uuid.UUID{toolID}, toolManager.deleted)
	assert.Equal(t, []uuid.UUID{skillID}, repo.deleted)
}

func TestService_CreateMCP_DelegatesToUnderlyingService(t *testing.T) {
	mcpCatalog := &stubMCPCatalog{}
	svc := integration.NewService(&stubToolCatalog{}, &stubDatasourceCatalog{}, mcpCatalog, &stubVPNCatalog{})

	resp, err := svc.CreateMCP(context.Background(), mcp.CreateRequest{Name: "filesystem", TransportType: "stdio", Enabled: true})
	require.NoError(t, err)
	assert.Equal(t, "filesystem", resp.Name)
	assert.Equal(t, "filesystem", mcpCatalog.created.Name)
}
