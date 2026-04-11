package agentic

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// captureAuditLogger is an in-memory PermissionAuditLogger used in tests.
type captureAuditLogger struct {
	entries []PermissionAuditEntry
}

func (c *captureAuditLogger) LogDecision(_ context.Context, entry PermissionAuditEntry) error {
	c.entries = append(c.entries, entry)
	return nil
}

func TestNoopPermissionAuditLogger_NeverErrors(t *testing.T) {
	var logger NoopPermissionAuditLogger
	entry := PermissionAuditEntry{
		SessionID: uuid.New(),
		ToolName:  "execute-sql",
		Decision:  AuditDecisionDeny,
		CreatedAt: time.Now(),
	}
	if err := logger.LogDecision(context.Background(), entry); err != nil {
		t.Errorf("NoopPermissionAuditLogger should never return an error, got: %v", err)
	}
}

func TestTruncateInput_ShortInput(t *testing.T) {
	s := "hello"
	got := truncateInput(s, 300)
	if got != s {
		t.Errorf("truncateInput for short string: expected %q, got %q", s, got)
	}
}

func TestTruncateInput_LongInput(t *testing.T) {
	s := ""
	for i := 0; i < 400; i++ {
		s += "a"
	}
	got := truncateInput(s, 300)
	if len([]rune(got)) != 300 {
		t.Errorf("truncateInput: expected 300 chars, got %d", len(got))
	}
}

func TestPermissionAuditEntry_AllDecisionTypes(t *testing.T) {
	logger := &captureAuditLogger{}
	sessionID := uuid.New()
	runID := uuid.New()

	decisions := []PermissionAuditDecision{
		AuditDecisionAllow,
		AuditDecisionDeny,
		AuditDecisionConfirmApproved,
		AuditDecisionConfirmDenied,
		AuditDecisionConfirmEscalated,
	}

	for _, d := range decisions {
		entry := PermissionAuditEntry{
			SessionID: sessionID,
			RunID:     &runID,
			ToolName:  "tool-x",
			Decision:  d,
			CreatedAt: time.Now(),
		}
		if err := logger.LogDecision(context.Background(), entry); err != nil {
			t.Errorf("unexpected error logging decision %s: %v", d, err)
		}
	}

	if len(logger.entries) != len(decisions) {
		t.Errorf("expected %d entries, got %d", len(decisions), len(logger.entries))
	}
}
