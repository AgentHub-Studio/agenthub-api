package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// slugify converts a name to a kebab-case slug for use when the LLM omits it.
func slugify(name string) string {
	s := strings.ToLower(name)
	var b strings.Builder
	prevHyphen := true
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	if result == "" {
		return "resource-" + uuid.New().String()[:8]
	}
	return result
}

// ManagementExecutor defines the operations allowed by the agenthub_manage tool.
// It abstracts the underlying domain repositories to provide a unified CRUD interface.
type ManagementExecutor struct {
	agents       agent.Repository
	skills       skill.SkillRepository
	// skillDeleter routes skill deletions through the service layer so that
	// binding-protection checks are enforced. P-C185-1.
	skillDeleter skill.Deleter
	tools        tool.ToolRepository
	mcpServers   mcp.Repository
}

// NewManagementExecutor creates a new ManagementExecutor.
// skillDeleter should be a *skill.Service (implements skill.Deleter) so that
// delete operations pass through the binding-protection check.
func NewManagementExecutor(
	agents agent.Repository,
	skills skill.SkillRepository,
	skillDeleter skill.Deleter,
	tools tool.ToolRepository,
	mcpServers mcp.Repository,
) *ManagementExecutor {
	return &ManagementExecutor{
		agents:       agents,
		skills:       skills,
		skillDeleter: skillDeleter,
		tools:        tools,
		mcpServers:   mcpServers,
	}
}

// Execute performs an administrative operation on the platform resources.
func (e *ManagementExecutor) Execute(ctx context.Context, operation, resource string, id string, query string, payload json.RawMessage) ToolExecResult {
	slog.Info("ManagementExecutor: executing operation", "operation", operation, "resource", resource, "id", id)

	switch operation {
	case "list":
		return e.list(ctx, resource, query)
	case "get":
		return e.get(ctx, resource, id)
	case "create":
		return e.create(ctx, resource, payload)
	case "update":
		return e.update(ctx, resource, id, payload)
	case "delete":
		return e.delete(ctx, resource, id)
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
		raw, total, err = e.agents.FindAll(ctx, "", page)
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
		data = map[string]any{"items": items, "total": total}
	case "integration":
		// Integration is an aggregation, we'll return tools and mcp servers for now.
		tItems, _, _ := e.tools.List(ctx, page, "")
		mItems, _ := e.mcpServers.List(ctx)
		data = map[string]any{"tools": tItems, "mcp_servers": mItems}
	case "mcp_server":
		var items []mcp.McpServerConfig
		items, err = e.mcpServers.List(ctx)
		data = map[string]any{"items": items, "total": len(items)}
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

func (e *ManagementExecutor) get(ctx context.Context, resource string, idStr string) ToolExecResult {
	id, err := uuid.Parse(idStr)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid ID format: %s", idStr)
		return ToolExecResult{Error: &errMsg}
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
		data, err = e.mcpServers.GetByID(ctx, id)
	case "integration":
		// Integration get: try tool first, then mcp server.
		var t tool.Tool
		t, err = e.tools.GetByID(ctx, id)
		if err == nil {
			data = tool.ResponseFrom(t)
		} else {
			data, err = e.mcpServers.GetByID(ctx, id)
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
		// agentCreatePayload accepts snake_case field names from the LLM (as documented
		// in skilldesc.go). agent.Agent has no JSON tags, so direct unmarshal would drop
		// system_prompt and model_config (Go JSON requires exact-case or tagged matches).
		var req struct {
			ID              uuid.UUID       `json:"id"`
			Name            string          `json:"name"`
			Slug            string          `json:"slug"`
			Description     string          `json:"description"`
			Status          agent.AgentStatus `json:"status"`
			SystemPrompt    *string         `json:"system_prompt"`
			ModelConfig     json.RawMessage `json:"model_config"`
			PermissionRules json.RawMessage `json:"permission_rules"`
			Config          json.RawMessage `json:"config"`
		}
		if err = json.Unmarshal(payload, &req); err == nil {
			a := agent.Agent{
				ID:              req.ID,
				Name:            req.Name,
				Slug:            req.Slug,
				Description:     req.Description,
				Status:          req.Status,
				SystemPrompt:    req.SystemPrompt,
				ModelConfig:     req.ModelConfig,
				PermissionRules: req.PermissionRules,
				Config:          req.Config,
			}
			// Ensure the LLM cannot produce a nil or zero-value primary key.
			// The repository does not generate IDs — the service layer does —
			// but management bypasses the service, so we must guard here.
			if a.ID == uuid.Nil {
				a.ID = uuid.New()
			}
			if a.Name == "" {
				errMsg := "agent name is required"
				return ToolExecResult{Error: &errMsg}
			}
			if a.Slug == "" {
				a.Slug = slugify(a.Name)
			}
			if a.Status == "" {
				a.Status = agent.StatusDraft
			}
			if a.CurrentVersion == 0 {
				a.CurrentVersion = 1
			}
			var createdAgent agent.Agent
			createdAgent, err = e.agents.Create(ctx, a)
			if err == nil {
				created = agent.ManageResponseFrom(createdAgent)
			}
		}
	case "skill":
		var s skill.Skill
		if err = json.Unmarshal(payload, &s); err == nil {
			if s.ID == uuid.Nil {
				s.ID = uuid.New()
			}
			if s.Name == "" {
				errMsg := "skill name is required"
				return ToolExecResult{Error: &errMsg}
			}
			if s.Slug == "" {
				s.Slug = slugify(s.Name)
			}
			created, err = e.skills.Create(ctx, s)
		}
	case "tool":
		var t tool.Tool
		if err = json.Unmarshal(payload, &t); err == nil {
			if t.ID == uuid.Nil {
				t.ID = uuid.New()
			}
			if t.Name == "" {
				errMsg := "tool name is required"
				return ToolExecResult{Error: &errMsg}
			}
			created, err = e.tools.Create(ctx, t)
		}
	case "mcp_server":
		var m mcp.McpServerConfig
		if err = json.Unmarshal(payload, &m); err == nil {
			if m.ID == uuid.Nil {
				m.ID = uuid.New()
			}
			if m.Name == "" {
				errMsg := "mcp_server name is required"
				return ToolExecResult{Error: &errMsg}
			}
			created, err = e.mcpServers.Create(ctx, m)
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

func (e *ManagementExecutor) update(ctx context.Context, resource string, idStr string, payload json.RawMessage) ToolExecResult {
	id, err := uuid.Parse(idStr)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid ID format: %s", idStr)
		return ToolExecResult{Error: &errMsg}
	}

	var updated any
	switch resource {
	case "agent":
		// Fetch existing agent first so that partial LLM payloads do not zero-out fields
		// that were not included in the payload. The repository.Update is a full overwrite.
		var existing agent.Agent
		existing, err = e.agents.FindByID(ctx, id)
		if err != nil {
			break
		}
		// Use snake_case DTO to match LLM-generated payloads (agent.Agent has no JSON tags).
		var req struct {
			Name            *string         `json:"name"`
			Slug            *string         `json:"slug"`
			Description     *string         `json:"description"`
			Status          agent.AgentStatus `json:"status"`
			SystemPrompt    *string         `json:"system_prompt"`
			ModelConfig     json.RawMessage `json:"model_config"`
			PermissionRules json.RawMessage `json:"permission_rules"`
			Config          json.RawMessage `json:"config"`
		}
		if err = json.Unmarshal(payload, &req); err == nil {
			// Merge: only overwrite fields that were explicitly provided.
			if req.Name != nil {
				existing.Name = *req.Name
			}
			if req.Slug != nil {
				existing.Slug = *req.Slug
			}
			if req.Description != nil {
				existing.Description = *req.Description
			}
			if req.Status != "" {
				existing.Status = req.Status
			}
			if req.SystemPrompt != nil {
				existing.SystemPrompt = req.SystemPrompt
			}
			if len(req.ModelConfig) > 0 {
				existing.ModelConfig = req.ModelConfig
			}
			if len(req.PermissionRules) > 0 {
				existing.PermissionRules = req.PermissionRules
			}
			if len(req.Config) > 0 {
				existing.Config = req.Config
			}
			var updatedAgent agent.Agent
			updatedAgent, err = e.agents.Update(ctx, existing)
			if err == nil {
				updated = agent.ManageResponseFrom(updatedAgent)
			}
		}
	case "skill":
		var s skill.UpdateRequest
		if err = json.Unmarshal(payload, &s); err == nil {
			updated, err = e.skills.Update(ctx, id, s)
		}
	case "tool":
		var req tool.UpdateRequest
		if err = json.Unmarshal(payload, &req); err == nil {
			var existing tool.Tool
			existing, err = e.tools.GetByID(ctx, id)
			if err == nil {
				// Merge: only overwrite fields that were explicitly provided.
				if req.Name != nil {
					existing.Name = *req.Name
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
				var updatedTool tool.Tool
				updatedTool, err = e.tools.Update(ctx, id, existing)
				if err == nil {
					updated = tool.ResponseFrom(updatedTool)
				}
			}
		}
	case "integration":
		// Manual integration update not supported via this path
		return ToolExecResult{Output: []byte(`{"status": "error", "message": "use specific integration path"}`)}
	case "mcp_server":
		var m mcp.McpServerConfig
		if err = json.Unmarshal(payload, &m); err == nil {
			m.ID = id
			updated, err = e.mcpServers.Update(ctx, m)
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

func (e *ManagementExecutor) delete(ctx context.Context, resource string, idStr string) ToolExecResult {
	id, err := uuid.Parse(idStr)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid ID format: %s", idStr)
		return ToolExecResult{Error: &errMsg}
	}

	switch resource {
	case "agent":
		err = e.agents.Delete(ctx, id)
	case "skill":
		// Route through service so CountAgentBindings check is enforced. P-C185-1.
		err = e.skillDeleter.Delete(ctx, id)
	case "tool":
		err = e.tools.Delete(ctx, id)
	case "integration":
		// Integration delete: try tool then mcp.
		err = e.tools.Delete(ctx, id)
		if err != nil {
			err = e.mcpServers.Delete(ctx, id)
		}
	case "mcp_server":
		err = e.mcpServers.Delete(ctx, id)
	default:
		errMsg := fmt.Sprintf("Unsupported resource for deletion: %s", resource)
		return ToolExecResult{Error: &errMsg}
	}

	if err != nil {
		errMsg := fmt.Sprintf("Error deleting %s: %s", resource, err.Error())
		return ToolExecResult{Error: &errMsg}
	}

	return ToolExecResult{Output: []byte(`{"status":"deleted"}`)}
}
