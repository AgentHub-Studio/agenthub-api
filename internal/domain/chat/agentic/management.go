package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ManagementExecutor defines the operations allowed by the agenthub_manage tool.
// It abstracts the underlying domain repositories to provide a unified CRUD interface.
type ManagementExecutor struct {
	agents     agent.Repository
	skills     skill.SkillRepository
	tools      tool.ToolRepository
	mcpServers mcp.Repository
}

// NewManagementExecutor creates a new ManagementExecutor.
func NewManagementExecutor(
	agents agent.Repository,
	skills skill.SkillRepository,
	tools tool.ToolRepository,
	mcpServers mcp.Repository,
) *ManagementExecutor {
	return &ManagementExecutor{
		agents:     agents,
		skills:     skills,
		tools:      tools,
		mcpServers: mcpServers,
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
	page := pagination.PageRequest{Page: 1, Size: 50}

	var data any
	var err error

	switch resource {
	case "agent":
		var items []agent.Agent
		var total int64
		items, total, err = e.agents.FindAll(ctx, "", page)
		data = map[string]any{"items": items, "total": total}
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
		data, err = e.agents.FindByID(ctx, id)
	case "skill":
		data, err = e.skills.GetByID(ctx, id)
	case "tool":
		data, err = e.tools.GetByID(ctx, id)
	case "mcp_server":
		data, err = e.mcpServers.GetByID(ctx, id)
	case "integration":
		// Integration get: try tool first, then mcp server.
		data, err = e.tools.GetByID(ctx, id)
		if err != nil {
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
		var a agent.Agent
		if err = json.Unmarshal(payload, &a); err == nil {
			created, err = e.agents.Create(ctx, a)
		}
	case "skill":
		var s skill.Skill
		if err = json.Unmarshal(payload, &s); err == nil {
			created, err = e.skills.Create(ctx, s)
		}
	case "tool":
		var t tool.Tool
		if err = json.Unmarshal(payload, &t); err == nil {
			created, err = e.tools.Create(ctx, t)
		}
	case "mcp_server":
		var m mcp.McpServerConfig
		if err = json.Unmarshal(payload, &m); err == nil {
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
		var a agent.Agent
		if err = json.Unmarshal(payload, &a); err == nil {
			a.ID = id
			updated, err = e.agents.Update(ctx, a)
		}
	case "skill":
		var s skill.UpdateRequest
		if err = json.Unmarshal(payload, &s); err == nil {
			updated, err = e.skills.Update(ctx, id, s)
		}
	case "tool":
		var t tool.UpdateRequest
		if err = json.Unmarshal(payload, &t); err == nil {
			updated, err = e.tools.Update(ctx, id, t)
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
		err = e.skills.Delete(ctx, id)
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

	return ToolExecResult{Output: []byte(`{"status": "deleted"}`)}
}
