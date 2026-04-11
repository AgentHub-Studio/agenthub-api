package agentic

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PermissionAuditDecision is the final outcome stored in the audit log.
type PermissionAuditDecision string

const (
	// AuditDecisionAllow records an auto-allowed tool call.
	AuditDecisionAllow PermissionAuditDecision = "allow"
	// AuditDecisionDeny records an auto-denied tool call (deny rule matched).
	AuditDecisionDeny PermissionAuditDecision = "deny"
	// AuditDecisionConfirmApproved records a confirm rule where the user said yes.
	AuditDecisionConfirmApproved PermissionAuditDecision = "confirm_approved"
	// AuditDecisionConfirmDenied records a confirm rule where the user said no (or timed out).
	AuditDecisionConfirmDenied PermissionAuditDecision = "confirm_denied"
	// AuditDecisionConfirmEscalated records a confirm rule in automated mode (no elicitation).
	AuditDecisionConfirmEscalated PermissionAuditDecision = "confirm_escalated"
)

// permissionAuditInputMaxLen is the maximum number of characters stored from tool input.
const permissionAuditInputMaxLen = 300

// PermissionAuditEntry is one permission decision event.
type PermissionAuditEntry struct {
	SessionID    uuid.UUID
	RunID        *uuid.UUID
	ToolName     string
	Decision     PermissionAuditDecision
	MatchedRule  string // nullable — the pattern that triggered the decision
	InputSnippet string // truncated tool input
	CreatedAt    time.Time
}

// PermissionAuditLogger records permission decisions to persistent storage.
// Implementations must be safe for concurrent use.
type PermissionAuditLogger interface {
	LogDecision(ctx context.Context, entry PermissionAuditEntry) error
}

// NoopPermissionAuditLogger satisfies PermissionAuditLogger without persisting anything.
// Used when the audit repository is not wired (e.g. unit tests, sub-runners).
type NoopPermissionAuditLogger struct{}

func (NoopPermissionAuditLogger) LogDecision(_ context.Context, _ PermissionAuditEntry) error {
	return nil
}

// truncateInput returns the first n characters of s, or s if it is shorter.
func truncateInput(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
