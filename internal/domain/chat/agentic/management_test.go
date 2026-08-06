package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// stubAgentRepository implements agent.Repository and records calls.
type stubAgentRepository struct {
	findByIDRet    agent.Agent
	findByIDCalled bool
	findByIDErr    error
	createRet      agent.Agent
	createInput    *agent.Agent
	createCalled   bool
	createErr      error
	deleteCalled   bool
	deleteErr      error
	deleteID       uuid.UUID
}

func (s *stubAgentRepository) FindAll(_ context.Context, _ agent.AgentStatus, _ string, _ pagination.PageRequest) ([]agent.Agent, int64, error) {
	return nil, 0, nil
}

func (s *stubAgentRepository) FindByID(_ context.Context, _ uuid.UUID) (agent.Agent, error) {
	s.findByIDCalled = true
	return s.findByIDRet, s.findByIDErr
}

func (s *stubAgentRepository) Create(_ context.Context, a agent.Agent) (agent.Agent, error) {
	s.createCalled = true
	s.createInput = &a
	if s.createErr != nil {
		return agent.Agent{}, s.createErr
	}
	if s.createRet.ID == uuid.Nil {
		s.createRet = a
	}
	return s.createRet, nil
}

func (s *stubAgentRepository) Update(_ context.Context, _ agent.Agent) (agent.Agent, error) {
	return agent.Agent{}, nil
}

func (s *stubAgentRepository) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.deleteID = id
	return s.deleteErr
}

func (s *stubAgentRepository) UpdateStatus(_ context.Context, _ uuid.UUID, _ agent.AgentStatus) (agent.Agent, error) {
	return agent.Agent{}, nil
}

func (s *stubAgentRepository) CountPublishedWithoutProvider(_ context.Context) (int64, error) {
	return 0, nil
}

// stubAgentService records the agent service calls that agenthub_manage must
// route through instead of bypassing validations in the repository layer.
type stubAgentService struct {
	deleteCalled  bool
	deleteID      uuid.UUID
	deleteErr     error
	createCalled  bool
	createReq     agent.CreateAgentRequest
	createResp    agent.AgentResponse
	createErr     error
	updateCalled  bool
	updateID      uuid.UUID
	updateReq     agent.UpdateAgentRequest
	updateResp    agent.AgentResponse
	updateErr     error
	publishCalled bool
	publishID     uuid.UUID
	publishResp   agent.AgentResponse
	publishErr    error
	archiveCalled bool
	archiveID     uuid.UUID
	archiveResp   agent.AgentResponse
	archiveErr    error
	restoreCalled bool
	restoreID     uuid.UUID
	restoreResp   agent.AgentResponse
	restoreErr    error
}

func (s *stubAgentService) Create(_ context.Context, req agent.CreateAgentRequest) (agent.AgentResponse, error) {
	s.createCalled = true
	s.createReq = req
	return s.createResp, s.createErr
}

func (s *stubAgentService) Update(_ context.Context, id uuid.UUID, req agent.UpdateAgentRequest) (agent.AgentResponse, error) {
	s.updateCalled = true
	s.updateID = id
	s.updateReq = req
	return s.updateResp, s.updateErr
}

func (s *stubAgentService) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.deleteID = id
	return s.deleteErr
}

func (s *stubAgentService) Publish(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	s.publishCalled = true
	s.publishID = id
	return s.publishResp, s.publishErr
}

func (s *stubAgentService) Archive(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	s.archiveCalled = true
	s.archiveID = id
	return s.archiveResp, s.archiveErr
}

func (s *stubAgentService) Restore(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	s.restoreCalled = true
	s.restoreID = id
	return s.restoreResp, s.restoreErr
}

// stubSkillDeleter is a test double for the skill manager surface.
type stubSkillDeleter struct {
	createErr    error
	createCalled bool
	createReq    skill.CreateRequest
	createResp   skill.Response
	updateErr    error
	updateCalled bool
	updateID     uuid.UUID
	updateReq    skill.UpdateRequest
	updateResp   skill.Response
	deleteErr    error
	deleteCalled bool
	lastDeleteID uuid.UUID
}

func (s *stubSkillDeleter) Create(_ context.Context, req skill.CreateRequest) (skill.Response, error) {
	s.createCalled = true
	s.createReq = req
	return s.createResp, s.createErr
}

func (s *stubSkillDeleter) Update(_ context.Context, id uuid.UUID, req skill.UpdateRequest) (skill.Response, error) {
	s.updateCalled = true
	s.updateID = id
	s.updateReq = req
	return s.updateResp, s.updateErr
}

func (s *stubSkillDeleter) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.lastDeleteID = id
	return s.deleteErr
}

type stubToolManager struct {
	createCalled bool
	createReq    tool.CreateRequest
	createResp   tool.Response
	createErr    error
	updateCalled bool
	updateID     uuid.UUID
	updateReq    tool.UpdateRequest
	updateResp   tool.Response
	updateErr    error
	deleteCalled bool
	deleteID     uuid.UUID
	deleteErr    error
}

func (s *stubToolManager) Create(_ context.Context, req tool.CreateRequest) (tool.Response, error) {
	s.createCalled = true
	s.createReq = req
	return s.createResp, s.createErr
}

func (s *stubToolManager) Update(_ context.Context, id uuid.UUID, req tool.UpdateRequest) (tool.Response, error) {
	s.updateCalled = true
	s.updateID = id
	s.updateReq = req
	return s.updateResp, s.updateErr
}

func (s *stubToolManager) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.deleteID = id
	return s.deleteErr
}

type stubMCPManager struct {
	createCalled bool
	createReq    mcp.CreateRequest
	createResp   mcp.McpServerConfigResponse
	createErr    error
	updateCalled bool
	updateID     uuid.UUID
	updateReq    mcp.UpdateRequest
	updateResp   mcp.McpServerConfigResponse
	updateErr    error
	deleteCalled bool
	deleteID     uuid.UUID
	deleteErr    error
}

func (s *stubMCPManager) Create(_ context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error) {
	s.createCalled = true
	s.createReq = req
	return s.createResp, s.createErr
}

func (s *stubMCPManager) Update(_ context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error) {
	s.updateCalled = true
	s.updateID = id
	s.updateReq = req
	return s.updateResp, s.updateErr
}

func (s *stubMCPManager) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.deleteID = id
	return s.deleteErr
}

type stubMCPRepository struct {
	items []mcp.McpServerConfig
}

func (s *stubMCPRepository) List(_ context.Context) ([]mcp.McpServerConfig, error) {
	return s.items, nil
}

func (s *stubMCPRepository) GetByID(_ context.Context, id uuid.UUID) (mcp.McpServerConfig, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
	return mcp.McpServerConfig{}, mcp.ErrNotFound
}

func (s *stubMCPRepository) GetByName(_ context.Context, name string) (mcp.McpServerConfig, error) {
	for _, item := range s.items {
		if item.Name == name {
			return item, nil
		}
	}
	return mcp.McpServerConfig{}, mcp.ErrNotFound
}

func (s *stubMCPRepository) Create(_ context.Context, item mcp.McpServerConfig) (mcp.McpServerConfig, error) {
	return item, nil
}

func (s *stubMCPRepository) Update(_ context.Context, item mcp.McpServerConfig) (mcp.McpServerConfig, error) {
	return item, nil
}

func (s *stubMCPRepository) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *stubMCPRepository) ListAutoStart(_ context.Context) ([]mcp.McpServerConfig, error) {
	return s.items, nil
}

func (s *stubMCPRepository) ListAllEnabled(_ context.Context) ([]mcp.McpServerConfig, error) {
	return s.items, nil
}

type stubManagementAuditRecorder struct {
	tenantID string
	requests []audit.RecordRequest
}

func (s *stubManagementAuditRecorder) Record(_ context.Context, tenantID string, req audit.RecordRequest) (audit.AuditLog, error) {
	s.tenantID = tenantID
	s.requests = append(s.requests, req)
	return audit.AuditLog{ID: uuid.New()}, nil
}

func TestManagementDelete_SkillBoundToAgent_ReturnsError(t *testing.T) {
	deleter := &stubSkillDeleter{deleteErr: skill.ErrSkillBoundToAgents}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)

	skillID := uuid.New()
	result := exec.Execute(context.Background(), "delete", "skill", skillID.String(), "", nil, uuid.New())

	require.True(t, deleter.deleteCalled)
	require.NotNil(t, result.Error)
	assert.Equal(t, skillID, deleter.lastDeleteID)
	assert.Contains(t, *result.Error, "Error deleting skill")
}

func TestManagementDelete_UnboundSkill_Succeeds(t *testing.T) {
	deleter := &stubSkillDeleter{}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)

	skillID := uuid.New()
	result := exec.Execute(context.Background(), "delete", "skill", skillID.String(), "", nil, uuid.New())

	require.True(t, deleter.deleteCalled)
	assert.Nil(t, result.Error)
	assert.Equal(t, skillID, deleter.lastDeleteID)
	assert.Equal(t, `{"status":"deleted"}`, string(result.Output))
}

func TestManagementDelete_CallsDeleterNotRepo(t *testing.T) {
	deleter := &stubSkillDeleter{}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)

	exec.Execute(context.Background(), "delete", "skill", uuid.New().String(), "", nil, uuid.New())

	assert.True(t, deleter.deleteCalled)
}

func TestManagementDelete_AgentID_Mismatch_IsRejected(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{}

	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)
	result := exec.Execute(context.Background(), "delete", "agent", agentID.String(), "", nil, sessionID)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "does not match session agent")
	assert.False(t, agentSvc.deleteCalled)
	assert.False(t, agentRepo.deleteCalled)
}

func TestManagementDelete_Agent_UsesServiceDelete(t *testing.T) {
	agentID := uuid.New()
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{}

	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)
	result := exec.Execute(context.Background(), "delete", "agent", agentID.String(), "", nil, agentID)

	assert.Nil(t, result.Error)
	assert.Equal(t, agentID, agentSvc.deleteID)
	assert.True(t, agentSvc.deleteCalled)
	assert.False(t, agentRepo.deleteCalled)
	assert.Equal(t, `{"status":"deleted"}`, string(result.Output))
}

func TestManagementDelete_Agent_ServiceErrorPropagates(t *testing.T) {
	agentID := uuid.New()
	agentRepo := &stubAgentRepository{}
	errSvc := &stubAgentService{deleteErr: errors.New("delete failed")}

	exec := agentic.NewManagementExecutor(errSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)
	result := exec.Execute(context.Background(), "delete", "agent", agentID.String(), "", nil, agentID)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "delete failed")
	assert.True(t, errSvc.deleteCalled)
	assert.False(t, agentRepo.deleteCalled)
}

func TestManagementCreate_Agent_ForceDisableManagement(t *testing.T) {
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{createResp: agent.AgentResponse{ID: uuid.New(), Name: "Sub-agent", Slug: "sub-agent", Status: string(agent.StatusDraft)}}
	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	payload := []byte(`{
		"name": "Sub-agent",
		"slug": "sub-agent",
		"enable_management": true
	}`)
	result := exec.Execute(context.Background(), "create", "agent", "", "", payload, uuid.New())

	assert.Nil(t, result.Error)
	assert.True(t, agentSvc.createCalled)
	assert.False(t, agentSvc.createReq.EnableManagement)
	assert.False(t, agentRepo.createCalled)
}

func TestManagementCreate_Agent_UsesServiceValidation(t *testing.T) {
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{createErr: errors.New("invalid modelConfig")}
	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "create", "agent", "", "", []byte(`{
		"name": "Sub-agent",
		"model_config": {"provider": "unsupported"}
	}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid modelConfig")
	assert.True(t, agentSvc.createCalled)
	assert.False(t, agentRepo.createCalled)
}

func TestManagementCreate_Agent_ResponseRemainsRedacted(t *testing.T) {
	agentRepo := &stubAgentRepository{}
	prompt := "internal prompt must not reach the LLM"
	agentSvc := &stubAgentService{createResp: agent.AgentResponse{
		ID:              uuid.New(),
		Name:            "Sub-agent",
		Slug:            "sub-agent",
		Status:          string(agent.StatusDraft),
		SystemPrompt:    &prompt,
		ModelConfig:     json.RawMessage(`{"model":"safe-model","apiKey":"must-not-leak"}`),
		PermissionRules: json.RawMessage(`{"secret":"must-not-leak"}`),
	}}
	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "create", "agent", "", "", []byte(`{"name":"Sub-agent"}`), uuid.New())

	require.Nil(t, result.Error)
	assert.NotContains(t, string(result.Output), "systemPrompt")
	assert.NotContains(t, string(result.Output), "permissionRules")
	assert.NotContains(t, string(result.Output), "must-not-leak")
	assert.Contains(t, string(result.Output), "safe-model")
	assert.False(t, agentRepo.createCalled)
}

func TestManagementCreate_Agent_RequiresService(t *testing.T) {
	agentRepo := &stubAgentRepository{}
	exec := agentic.NewManagementExecutor(nil, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "create", "agent", "", "", []byte(`{"name":"Sub-agent"}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "service unavailable")
	assert.False(t, agentRepo.createCalled)
}

func TestManagementCreate_Skill_UsesServiceValidation(t *testing.T) {
	skillSvc := &stubSkillDeleter{createErr: skill.ErrSkillInert}
	exec := agentic.NewManagementExecutor(nil, nil, nil, skillSvc, nil, nil)

	result := exec.Execute(context.Background(), "create", "skill", "", "", []byte(`{"name":"Empty"}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "will have no effect")
	assert.True(t, skillSvc.createCalled)
}

func TestManagementUpdate_Skill_UsesServiceValidation(t *testing.T) {
	skillID := uuid.New()
	skillSvc := &stubSkillDeleter{updateErr: errors.New("invalid context mode")}
	exec := agentic.NewManagementExecutor(nil, nil, nil, skillSvc, nil, nil)

	result := exec.Execute(context.Background(), "update", "skill", skillID.String(), "", []byte(`{"contextMode":"invalid"}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid context mode")
	assert.True(t, skillSvc.updateCalled)
	assert.Equal(t, skillID, skillSvc.updateID)
}

func TestManagementCreate_Tool_UsesServiceValidation(t *testing.T) {
	toolSvc := &stubToolManager{createErr: errors.New("invalid URL")}
	exec := agentic.NewManagementExecutor(nil, nil, nil, &stubSkillDeleter{}, nil, nil).
		WithToolManager(toolSvc)

	result := exec.Execute(context.Background(), "create", "tool", "", "", []byte(`{"name":"Unsafe","type":"HTTP","config":{"url":"http://127.0.0.1"}}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid URL")
	assert.True(t, toolSvc.createCalled)
}

func TestManagementUpdate_Tool_UsesServiceValidation(t *testing.T) {
	toolID := uuid.New()
	toolSvc := &stubToolManager{updateErr: errors.New("invalid URL")}
	exec := agentic.NewManagementExecutor(nil, nil, nil, &stubSkillDeleter{}, nil, nil).
		WithToolManager(toolSvc)

	result := exec.Execute(context.Background(), "update", "tool", toolID.String(), "", []byte(`{"config":{"url":"http://127.0.0.1"}}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid URL")
	assert.True(t, toolSvc.updateCalled)
	assert.Equal(t, toolID, toolSvc.updateID)
}

func TestManagementCreate_MCPServer_UsesServiceValidation(t *testing.T) {
	mcpSvc := &stubMCPManager{createErr: errors.New("invalid transport")}
	exec := agentic.NewManagementExecutor(nil, nil, nil, &stubSkillDeleter{}, nil, nil).
		WithMCPManager(mcpSvc)

	result := exec.Execute(context.Background(), "create", "mcp_server", "", "", []byte(`{"name":"Invalid","transportType":"invalid"}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid transport")
	assert.True(t, mcpSvc.createCalled)
}

func TestManagementUpdate_MCPServer_UsesServiceValidation(t *testing.T) {
	mcpID := uuid.New()
	mcpSvc := &stubMCPManager{updateErr: errors.New("invalid transport")}
	exec := agentic.NewManagementExecutor(nil, nil, nil, &stubSkillDeleter{}, nil, nil).
		WithMCPManager(mcpSvc)

	result := exec.Execute(context.Background(), "update", "mcp_server", mcpID.String(), "", []byte(`{"transportType":"invalid"}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid transport")
	assert.True(t, mcpSvc.updateCalled)
	assert.Equal(t, mcpID, mcpSvc.updateID)
}

func TestManagementDelete_MCPServer_UsesService(t *testing.T) {
	mcpID := uuid.New()
	mcpSvc := &stubMCPManager{}
	exec := agentic.NewManagementExecutor(nil, nil, nil, &stubSkillDeleter{}, nil, nil).
		WithMCPManager(mcpSvc)

	result := exec.Execute(context.Background(), "delete", "mcp_server", mcpID.String(), "", nil, uuid.New())

	require.Nil(t, result.Error)
	assert.True(t, mcpSvc.deleteCalled)
	assert.Equal(t, mcpID, mcpSvc.deleteID)
}

func TestManagementRead_MCPServer_UsesSanitizedResponse(t *testing.T) {
	server := mcp.McpServerConfig{
		ID:   uuid.New(),
		Name: "protected-server",
		Env: map[string]string{
			"API_TOKEN": "opaque-token-value-that-must-not-reach-the-llm",
		},
	}
	repo := &stubMCPRepository{items: []mcp.McpServerConfig{server}}
	exec := agentic.NewManagementExecutor(nil, nil, nil, &stubSkillDeleter{}, nil, repo)

	for _, operation := range []string{"list", "get"} {
		t.Run(operation, func(t *testing.T) {
			id := ""
			if operation == "get" {
				id = server.ID.String()
			}
			result := exec.Execute(context.Background(), operation, "mcp_server", id, "", nil, uuid.New())

			require.Nil(t, result.Error)
			assert.NotContains(t, string(result.Output), "opaque-token-value-that-must-not-reach-the-llm")
			assert.Contains(t, string(result.Output), "\"API_TOKEN\":\"***\"")
		})
	}
}

func TestManagementCreate_Skill_RequiresService(t *testing.T) {
	exec := agentic.NewManagementExecutor(nil, nil, nil, nil, nil, nil)

	result := exec.Execute(context.Background(), "create", "skill", "", "", []byte(`{"name":"Skill"}`), uuid.New())

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "skill management service unavailable")
}

func TestManagementUpdate_Agent_UsesServiceValidation(t *testing.T) {
	agentID := uuid.New()
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{updateErr: errors.New("invalid modelConfig")}
	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "update", "agent", agentID.String(), "", []byte(`{
		"model_config": {"provider": "unsupported"}
	}`), agentID)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid modelConfig")
	assert.True(t, agentSvc.updateCalled)
	assert.Equal(t, agentID, agentSvc.updateID)
	assert.False(t, agentRepo.findByIDCalled)
}

func TestManagementUpdate_Agent_StatusUsesLifecycleService(t *testing.T) {
	agentID := uuid.New()
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{publishResp: agent.AgentResponse{
		ID:     agentID,
		Status: string(agent.StatusPublished),
	}}
	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "update", "agent", agentID.String(), "", []byte(`{"status":"PUBLISHED"}`), agentID)

	require.Nil(t, result.Error)
	assert.True(t, agentSvc.publishCalled)
	assert.Equal(t, agentID, agentSvc.publishID)
	assert.False(t, agentSvc.updateCalled)
	assert.False(t, agentRepo.findByIDCalled)
	assert.Contains(t, string(result.Output), `"status":"PUBLISHED"`)
}

func TestManagementUpdate_Agent_StatusRejectsMixedFieldUpdate(t *testing.T) {
	agentID := uuid.New()
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{}
	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "update", "agent", agentID.String(), "", []byte(`{"status":"PUBLISHED","name":"renamed"}`), agentID)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "status must be updated separately")
	assert.False(t, agentSvc.publishCalled)
	assert.False(t, agentSvc.updateCalled)
	assert.False(t, agentRepo.findByIDCalled)
}

func TestManagementUpdate_Agent_StatusRejectsUnsupportedValue(t *testing.T) {
	agentID := uuid.New()
	agentRepo := &stubAgentRepository{}
	agentSvc := &stubAgentService{}
	exec := agentic.NewManagementExecutor(agentSvc, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "update", "agent", agentID.String(), "", []byte(`{"status":"DELETED"}`), agentID)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "unsupported agent status")
	assert.False(t, agentSvc.publishCalled)
	assert.False(t, agentSvc.archiveCalled)
	assert.False(t, agentSvc.restoreCalled)
	assert.False(t, agentRepo.findByIDCalled)
}

func TestManagementDelete_Agent_RequiresServiceDelete(t *testing.T) {
	agentID := uuid.New()
	agentRepo := &stubAgentRepository{}
	exec := agentic.NewManagementExecutor(nil, agentRepo, nil, &stubSkillDeleter{}, nil, nil)

	result := exec.Execute(context.Background(), "delete", "agent", agentID.String(), "", nil, agentID)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "agent delete backend unavailable")
	assert.False(t, agentRepo.deleteCalled)
}

func TestManagementDelete_Tool_RecordsAudit(t *testing.T) {
	toolID := uuid.New()
	toolSvc := &stubToolManager{}
	auditRecorder := &stubManagementAuditRecorder{}
	exec := agentic.NewManagementExecutor(nil, nil, nil, &stubSkillDeleter{}, nil, nil).
		WithToolManager(toolSvc).
		WithAuditRecorder(auditRecorder)

	ctx := tenantctx.NewContext(context.Background(), "tenant-a")
	result := exec.Execute(ctx, "delete", "tool", toolID.String(), "", nil, uuid.New())

	assert.Nil(t, result.Error)
	assert.True(t, toolSvc.deleteCalled)
	assert.Equal(t, toolID, toolSvc.deleteID)
	require.Len(t, auditRecorder.requests, 1)
	assert.Equal(t, "tenant-a", auditRecorder.tenantID)
	assert.Equal(t, "tool", auditRecorder.requests[0].EntityType)
	assert.Equal(t, toolID.String(), auditRecorder.requests[0].EntityID)
	assert.Equal(t, audit.AuditActionDelete, auditRecorder.requests[0].Action)
	assert.Contains(t, auditRecorder.requests[0].Metadata, "agenthub_manage")
}
