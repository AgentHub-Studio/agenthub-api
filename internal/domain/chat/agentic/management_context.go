package agentic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ManagementContext carries session-scoped authorization for agenthub_manage.
type ManagementContext struct {
	SessionID      uuid.UUID
	SessionAgentID uuid.UUID
	RunAgentID     uuid.UUID
	Audit          PermissionAuditLogger
}

// ValidateDestructive ensures the running agent matches the session-bound agent
// before delete or sensitive update operations (SEC-01 / TR-01).
func (mc ManagementContext) ValidateDestructive() error {
	if mc.SessionAgentID == uuid.Nil || mc.RunAgentID == uuid.Nil {
		return fmt.Errorf("management: missing session agent context")
	}
	if mc.RunAgentID != mc.SessionAgentID {
		return fmt.Errorf("management: agent mismatch — operation denied")
	}
	return nil
}

func (mc ManagementContext) logAudit(operation, resource, id string) {
	if mc.Audit == nil {
		return
	}
	snippet := fmt.Sprintf("%s %s", operation, resource)
	if id != "" {
		snippet = fmt.Sprintf("%s %s id=%s", operation, resource, id)
	}
	_ = mc.Audit.LogDecision(context.Background(), PermissionAuditEntry{
		SessionID:    mc.SessionID,
		ToolName:     "agenthub_manage",
		Decision:     AuditDecisionAllow,
		InputSnippet: snippet,
	})
}
