package agentic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ManagementExecutor defines the operations allowed by the agenthub_manage tool.
// It coordinates administrative CRUD while agent mutations remain in the service layer.
type ManagementExecutor struct {
	agentManager agentManager
	agents       agent.Repository
	skills       skill.SkillRepository
	skillManager skillManager
	tools        tool.ToolRepository
	toolManager  toolManager
	mcpServers   mcp.Repository
	mcpManager   mcpManager
	audit        ManagementAuditRecorder
}

// agentManager is the narrow agent service surface used by agenthub_manage.
// Management operations must not bypass this service: it owns validation,
// lifecycle defaults, binding handling and audit records for agents.
type agentManager interface {
	Create(ctx context.Context, req agent.CreateAgentRequest) (agent.AgentResponse, error)
	Update(ctx context.Context, id uuid.UUID, req agent.UpdateAgentRequest) (agent.AgentResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Publish(ctx context.Context, id uuid.UUID) (agent.AgentResponse, error)
	Archive(ctx context.Context, id uuid.UUID) (agent.AgentResponse, error)
	Restore(ctx context.Context, id uuid.UUID) (agent.AgentResponse, error)
}

// skillManager is the narrow skill service surface used by agenthub_manage.
// It owns validation, slug generation and binding-protection checks.
type skillManager interface {
	Create(ctx context.Context, req skill.CreateRequest) (skill.Response, error)
	Update(ctx context.Context, id uuid.UUID, req skill.UpdateRequest) (skill.Response, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// toolManager is the narrow tool service surface used by agenthub_manage.
// It owns URL, schema, datasource and knowledge-base validation.
type toolManager interface {
	Create(ctx context.Context, req tool.CreateRequest) (tool.Response, error)
	Update(ctx context.Context, id uuid.UUID, req tool.UpdateRequest) (tool.Response, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// mcpManager is the narrow MCP service surface used by agenthub_manage.
// It owns transport validation and safe configuration normalization.
type mcpManager interface {
	Create(ctx context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error)
	Update(ctx context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ManagementAuditRecorder is the subset of audit.Service used by agenthub_manage.
type ManagementAuditRecorder interface {
	Record(ctx context.Context, tenantID string, req audit.RecordRequest) (audit.AuditLog, error)
}

// NewManagementExecutor creates a new ManagementExecutor.
// agentManager and skillManager must be service-layer implementations so that
// management operations retain validation, lifecycle and binding protections.
func NewManagementExecutor(
	agentManager agentManager,
	agents agent.Repository,
	skills skill.SkillRepository,
	skillManager skillManager,
	tools tool.ToolRepository,
	mcpServers mcp.Repository,
) *ManagementExecutor {
	return &ManagementExecutor{
		agentManager: agentManager,
		agents:       agents,
		skills:       skills,
		skillManager: skillManager,
		tools:        tools,
		mcpServers:   mcpServers,
	}
}

// WithAuditRecorder records destructive agenthub_manage operations.
func (e *ManagementExecutor) WithAuditRecorder(recorder ManagementAuditRecorder) *ManagementExecutor {
	e.audit = recorder
	return e
}

// WithToolManager routes tool mutations through the configured service.
func (e *ManagementExecutor) WithToolManager(manager toolManager) *ManagementExecutor {
	e.toolManager = manager
	return e
}

// WithMCPManager routes MCP mutations through the configured service.
func (e *ManagementExecutor) WithMCPManager(manager mcpManager) *ManagementExecutor {
	e.mcpManager = manager
	return e
}

// Execute performs an administrative operation on the platform resources.
func (e *ManagementExecutor) Execute(ctx context.Context, operation, resource string, id string, query string, payload json.RawMessage, sessionAgentID uuid.UUID) ToolExecResult {
	slog.Info("ManagementExecutor: executing operation", "operation", operation, "resource", resource, "id", id)

	switch operation {
	case "list":
		return e.list(ctx, resource, query)
	case "get":
		return e.get(ctx, resource, id, sessionAgentID)
	case "create":
		return e.create(ctx, resource, payload)
	case "update":
		return e.update(ctx, resource, id, payload, sessionAgentID)
	case "delete":
		return e.delete(ctx, resource, id, sessionAgentID)
	default:
		errMsg := fmt.Sprintf("Unsupported operation: %s", operation)
		return ToolExecResult{Error: &errMsg}
	}
}

func (e *ManagementExecutor) list(ctx context.Context, resource string, query string) ToolExecResult {
	page := pagination.PageRequest{Page: 0, Size: 50}

	var data any
	var err error

	switch resource {
	case "agent":
		var raw []agent.Agent
		var total int64
		raw, total, err = e.agents.FindAll(ctx, "", "", page)
		redacted := make([]agent.AgentManageResponse, len(raw))
		for i, a := range raw {
			redacted[i] = agent.ManageResponseFrom(a)
		}
		data = map[string]any{"items": redacted, "total": total}
	case "skill":
		var items []skill.Skill
		var total int64
		items, total, err = e.skills.List(ctx, nil, page)
		data = map[string]any{"items": items, "total": total}
	case "tool":
		var items []tool.Tool
		var total int64
		items, total, err = e.tools.List(ctx, page, "")
		data = map[string]any{"items": managementToolResponses(items), "total": total}
	case "integration":
		// Integration is an aggregation. Never marshal raw repository entities:
		// their configuration may contain credentials that the public DTO redacts.
		tItems, _, _ := e.tools.List(ctx, page, "")
		mItems, _ := e.mcpServers.List(ctx)
		data = map[string]any{"tools": managementToolResponses(tItems), "mcp_servers": managementMCPResponses(mItems)}
	case "mcp_server":
		var items []mcp.McpServerConfig
		items, err = e.mcpServers.List(ctx)
		data = map[string]any{"items": managementMCPResponses(items), "total": len(items)}
	default:
		errMsg := fmt.Sprintf("Unsupported resource: %s", resource)
		return ToolExecResult{Error: &errMsg}
	}

	if err != nil {
		errMsg := fmt.Sprintf("Error listing %s: %s", resource, err.Error())
		return ToolExecResult{Error: &errMsg}
	}

	output, _ := json.Marshal(data)
	return ToolExecResult{Output: output}
}

func (e *ManagementExecutor) get(ctx context.Context, resource string, idStr string, sessionAgentID uuid.UUID) ToolExecResult {
	id, err := uuid.Parse(idStr)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid ID format: %s", idStr)
		return ToolExecResult{Error: &errMsg}
	}
	if resource == "agent" {
		if err = e.assertSessionAgentScope(id, sessionAgentID); err != nil {
			errMsg := err.Error()
			return ToolExecResult{Error: &errMsg}
		}
	}

	var data any
	switch resource {
	case "agent":
		// Use redacted DTO — omits systemPrompt, permissionRules, and credential fields.
		// P-C208-1, P-C211-1
		var a agent.Agent
		a, err = e.agents.FindByID(ctx, id)
		if err == nil {
			data = agent.ManageResponseFrom(a)
		}
	case "skill":
		data, err = e.skills.GetByID(ctx, id)
	case "tool":
		// tool.ResponseFrom already sanitizes credential fields. P-C239-1
		var t tool.Tool
		t, err = e.tools.GetByID(ctx, id)
		if err == nil {
			data = tool.ResponseFrom(t)
		}
	case "mcp_server":
		var server mcp.McpServerConfig
		server, err = e.mcpServers.GetByID(ctx, id)
		if err == nil {
			data = mcp.ResponseFrom(server)
		}
	case "integration":
		// Integration get: try tool first, then mcp server.
		var t tool.Tool
		t, err = e.tools.GetByID(ctx, id)
		if err == nil {
			data = tool.ResponseFrom(t)
		} else {
			var server mcp.McpServerConfig
			server, err = e.mcpServers.GetByID(ctx, id)
			if err == nil {
				data = mcp.ResponseFrom(server)
			}
		}
	default:
		errMsg := fmt.Sprintf("Unsupported resource: %s", resource)
		return ToolExecResult{Error: &errMsg}
	}

	if err != nil {
		errMsg := fmt.Sprintf("%s not found: %s", resource, idStr)
		return ToolExecResult{Error: &errMsg}
	}

	output, _ := json.Marshal(data)
	return ToolExecResult{Output: output}
}

func (e *ManagementExecutor) create(ctx context.Context, resource string, payload json.RawMessage) ToolExecResult {
	var err error
	var created any

	switch resource {
	case "agent":
		if e.agentManager == nil {
			errMsg := "agent management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		// Accept the documented snake_case names from the LLM, then delegate to the
		// canonical service DTO. Do not unmarshal into agent.Agent or call the
		// repository directly: that bypasses validation and agent lifecycle rules.
		var req struct {
			Name            string          `json:"name"`
			Slug            string          `json:"slug"`
			Description     string          `json:"description"`
			SystemPrompt    *string         `json:"system_prompt"`
			ModelConfig     json.RawMessage `json:"model_config"`
			PermissionRules json.RawMessage `json:"permission_rules"`
			Config          json.RawMessage `json:"config"`
		}
		if err = json.Unmarshal(payload, &req); err == nil {
			createdAgent, createErr := e.agentManager.Create(ctx, agent.CreateAgentRequest{
				Name:            req.Name,
				Slug:            req.Slug,
				Description:     req.Description,
				SystemPrompt:    req.SystemPrompt,
				ModelConfig:     req.ModelConfig,
				PermissionRules: req.PermissionRules,
				Config:          req.Config,
				// Sub-agents created by management never inherit management access.
				EnableManagement: false,
			})
			if createErr != nil {
				err = createErr
			} else {
				created = manageResponseFromAgentResponse(createdAgent)
			}
		}
	case "skill":
		if e.skillManager == nil {
			errMsg := "skill management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		var req skill.CreateRequest
		if err = json.Unmarshal(payload, &req); err == nil {
			created, err = e.skillManager.Create(ctx, req)
		}
	case "tool":
		if e.toolManager == nil {
			errMsg := "tool management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		var req tool.CreateRequest
		if err = json.Unmarshal(payload, &req); err == nil {
			created, err = e.toolManager.Create(ctx, req)
		}
	case "mcp_server":
		if e.mcpManager == nil {
			errMsg := "mcp management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		var req mcp.CreateRequest
		if err = json.Unmarshal(payload, &req); err == nil {
			created, err = e.mcpManager.Create(ctx, req)
		}
	default:
		errMsg := fmt.Sprintf("Unsupported resource for creation: %s", resource)
		return ToolExecResult{Error: &errMsg}
	}

	if err != nil {
		errMsg := fmt.Sprintf("Error creating %s: %s", resource, err.Error())
		return ToolExecResult{Error: &errMsg}
	}

	output, _ := json.Marshal(created)
	return ToolExecResult{Output: output}
}

func (e *ManagementExecutor) update(ctx context.Context, resource string, idStr string, payload json.RawMessage, sessionAgentID uuid.UUID) ToolExecResult {
	id, err := uuid.Parse(idStr)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid ID format: %s", idStr)
		return ToolExecResult{Error: &errMsg}
	}
	if resource == "agent" {
		if err = e.assertSessionAgentScope(id, sessionAgentID); err != nil {
			errMsg := err.Error()
			return ToolExecResult{Error: &errMsg}
		}
	}

	var updated any
	switch resource {
	case "agent":
		if e.agentManager == nil {
			errMsg := "agent management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		// Map the documented snake_case payload to the canonical service DTO. The
		// service merges partial updates and validates every updated field.
		var req struct {
			Name            *string            `json:"name"`
			Slug            *string            `json:"slug"`
			Description     *string            `json:"description"`
			Status          *agent.AgentStatus `json:"status"`
			SystemPrompt    *string            `json:"system_prompt"`
			ModelConfig     json.RawMessage    `json:"model_config"`
			PermissionRules json.RawMessage    `json:"permission_rules"`
			Config          json.RawMessage    `json:"config"`
		}
		if err = json.Unmarshal(payload, &req); err == nil {
			var updatedAgent agent.AgentResponse
			if req.Status != nil {
				if req.Name != nil || req.Slug != nil || req.Description != nil || req.SystemPrompt != nil || len(req.ModelConfig) != 0 || len(req.PermissionRules) != 0 || len(req.Config) != 0 {
					err = errors.New("agent status must be updated separately from agent fields")
				} else {
					updatedAgent, err = e.updateAgentStatus(ctx, id, *req.Status)
				}
			} else {
				updatedAgent, err = e.agentManager.Update(ctx, id, agent.UpdateAgentRequest{
					Name:            req.Name,
					Slug:            req.Slug,
					Description:     req.Description,
					SystemPrompt:    req.SystemPrompt,
					ModelConfig:     req.ModelConfig,
					PermissionRules: req.PermissionRules,
					Config:          req.Config,
				})
			}
			if err == nil {
				updated = manageResponseFromAgentResponse(updatedAgent)
			}
		}
	case "skill":
		if e.skillManager == nil {
			errMsg := "skill management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		var s skill.UpdateRequest
		if err = json.Unmarshal(payload, &s); err == nil {
			updated, err = e.skillManager.Update(ctx, id, s)
		}
	case "tool":
		if e.toolManager == nil {
			errMsg := "tool management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		var req tool.UpdateRequest
		if err = json.Unmarshal(payload, &req); err == nil {
			updated, err = e.toolManager.Update(ctx, id, req)
		}
	case "integration":
		// Manual integration update not supported via this path
		return ToolExecResult{Output: []byte(`{"status": "error", "message": "use specific integration path"}`)}
	case "mcp_server":
		if e.mcpManager == nil {
			errMsg := "mcp management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		var req mcp.UpdateRequest
		if err = json.Unmarshal(payload, &req); err == nil {
			updated, err = e.mcpManager.Update(ctx, id, req)
		}
	default:
		errMsg := fmt.Sprintf("Unsupported resource for update: %s", resource)
		return ToolExecResult{Error: &errMsg}
	}

	if err != nil {
		errMsg := fmt.Sprintf("Error updating %s: %s", resource, err.Error())
		return ToolExecResult{Error: &errMsg}
	}

	output, _ := json.Marshal(updated)
	return ToolExecResult{Output: output}
}

func (e *ManagementExecutor) delete(ctx context.Context, resource string, idStr string, sessionAgentID uuid.UUID) ToolExecResult {
	id, err := uuid.Parse(idStr)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid ID format: %s", idStr)
		return ToolExecResult{Error: &errMsg}
	}
	if resource == "agent" {
		if err = e.assertSessionAgentScope(id, sessionAgentID); err != nil {
			errMsg := err.Error()
			return ToolExecResult{Error: &errMsg}
		}
	}

	switch resource {
	case "agent":
		err = e.deleteAgent(ctx, id)
	case "skill":
		if e.skillManager == nil {
			errMsg := "skill management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		// Route through service so validation and binding checks are enforced.
		err = e.skillManager.Delete(ctx, id)
	case "tool":
		if e.toolManager == nil {
			errMsg := "tool management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		err = e.toolManager.Delete(ctx, id)
	case "integration":
		// Integration delete: try tool then mcp.
		if e.toolManager == nil || e.mcpManager == nil {
			errMsg := "integration management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		err = e.toolManager.Delete(ctx, id)
		if err != nil {
			err = e.mcpManager.Delete(ctx, id)
		}
	case "mcp_server":
		if e.mcpManager == nil {
			errMsg := "mcp management service unavailable"
			return ToolExecResult{Error: &errMsg}
		}
		err = e.mcpManager.Delete(ctx, id)
	default:
		errMsg := fmt.Sprintf("Unsupported resource for deletion: %s", resource)
		return ToolExecResult{Error: &errMsg}
	}

	if err != nil {
		errMsg := fmt.Sprintf("Error deleting %s: %s", resource, err.Error())
		return ToolExecResult{Error: &errMsg}
	}

	// Agent deletes are already audited by agent.Service. Other agenthub_manage
	// destructive paths delete through repositories/services that do not own audit.
	if resource != "agent" {
		e.recordDeleteAudit(ctx, resource, id)
	}

	return ToolExecResult{Output: []byte(`{"status":"deleted"}`)}
}

func (e *ManagementExecutor) recordDeleteAudit(ctx context.Context, resource string, id uuid.UUID) {
	if e.audit == nil {
		return
	}
	tenantID := tenantctx.FromContext(ctx)
	if tenantID == "" {
		return
	}
	metadata, _ := json.Marshal(map[string]string{
		"source":   "agenthub_manage",
		"resource": resource,
	})
	if _, err := e.audit.Record(ctx, tenantID, audit.RecordRequest{
		EntityType: resource,
		EntityID:   id.String(),
		Action:     audit.AuditActionDelete,
		Metadata:   string(metadata),
	}); err != nil {
		slog.Warn("agenthub_manage: failed to record delete audit",
			"resource", resource, "id", id, "error", err)
	}
}

func (e *ManagementExecutor) assertSessionAgentScope(id, sessionAgentID uuid.UUID) error {
	if sessionAgentID == uuid.Nil {
		return errors.New("session agent ID is required")
	}
	if id != sessionAgentID {
		return fmt.Errorf("agent id %s does not match session agent id %s", id, sessionAgentID)
	}
	return nil
}

func (e *ManagementExecutor) deleteAgent(ctx context.Context, id uuid.UUID) error {
	if e.agentManager != nil {
		return e.agentManager.Delete(ctx, id)
	}
	return errors.New("agent delete backend unavailable")
}

// updateAgentStatus preserves the service-owned lifecycle transitions. Status
// changes may not be merged with field updates because the service exposes
// lifecycle actions as independent, audited operations.
func (e *ManagementExecutor) updateAgentStatus(ctx context.Context, id uuid.UUID, status agent.AgentStatus) (agent.AgentResponse, error) {
	switch status {
	case agent.StatusPublished:
		return e.agentManager.Publish(ctx, id)
	case agent.StatusArchived:
		return e.agentManager.Archive(ctx, id)
	case agent.StatusDraft:
		return e.agentManager.Restore(ctx, id)
	default:
		return agent.AgentResponse{}, fmt.Errorf("unsupported agent status: %s", status)
	}
}

// manageResponseFromAgentResponse preserves the redaction contract after an
// agent service mutation. AgentResponse intentionally contains fields that the
// LLM must never see through agenthub_manage, so never marshal it directly.
func manageResponseFromAgentResponse(response agent.AgentResponse) agent.AgentManageResponse {
	return agent.AgentManageResponse{
		ID:               response.ID,
		Name:             response.Name,
		Slug:             response.Slug,
		Description:      response.Description,
		Status:           agent.AgentStatus(response.Status),
		CurrentVersion:   response.CurrentVersion,
		ModelConfig:      agent.SanitizeModelConfig(response.ModelConfig),
		EnableManagement: response.EnableManagement,
		CreatedAt:        response.CreatedAt,
		UpdatedAt:        response.UpdatedAt,
	}
}

func managementToolResponses(items []tool.Tool) []tool.Response {
	responses := make([]tool.Response, len(items))
	for i, item := range items {
		responses[i] = tool.ResponseFrom(item)
	}
	return responses
}

func managementMCPResponses(items []mcp.McpServerConfig) []mcp.McpServerConfigResponse {
	responses := make([]mcp.McpServerConfigResponse, len(items))
	for i, item := range items {
		responses[i] = mcp.ResponseFrom(item)
	}
	return responses
}
