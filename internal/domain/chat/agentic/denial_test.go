package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestNewDenialTracker_DefaultThreshold(t *testing.T) {
	dt := agentic.NewDenialTracker(0)
	require.NotNil(t, dt)
	// Default threshold is 3 — should escalate on 3rd denial.
	dt.RecordDenial("tool-a", "input")
	dt.RecordDenial("tool-a", "input")
	escalated := dt.RecordDenial("tool-a", "input")
	assert.True(t, escalated, "should escalate after 3 denials with default threshold")
}

func TestDenialTracker_EscalatesAtThreshold(t *testing.T) {
	dt := agentic.NewDenialTracker(2)

	esc1 := dt.RecordDenial("execute-sql", `{"query":"DROP TABLE"}`)
	assert.False(t, esc1, "first denial should not escalate")

	esc2 := dt.RecordDenial("execute-sql", `{"query":"DROP TABLE"}`)
	assert.True(t, esc2, "second denial should escalate at threshold=2")
}

func TestDenialTracker_NoDoubleEscalation(t *testing.T) {
	dt := agentic.NewDenialTracker(2)

	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-a", "") // escalates

	esc := dt.RecordDenial("tool-a", "")
	assert.False(t, esc, "should not escalate again after already escalated")
}

func TestDenialTracker_IndependentTools(t *testing.T) {
	dt := agentic.NewDenialTracker(2)

	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-b", "")

	// Neither should be escalated yet.
	assert.Empty(t, dt.EscalationHints())

	// Escalate tool-a.
	esc := dt.RecordDenial("tool-a", "")
	assert.True(t, esc)

	hints := dt.EscalationHints()
	assert.Len(t, hints, 1)
	assert.Contains(t, hints[0], "tool-a")
}

func TestDenialTracker_RecordAllowResetsCounter(t *testing.T) {
	dt := agentic.NewDenialTracker(3)

	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-a", "")
	// 2 denials — one more would escalate.

	dt.RecordAllow("tool-a")

	// Counter reset — need 3 more denials to escalate.
	esc := dt.RecordDenial("tool-a", "")
	assert.False(t, esc)
	assert.False(t, dt.GetRecord("tool-a").Escalated) // not escalated
}

func TestDenialTracker_GetRecord(t *testing.T) {
	dt := agentic.NewDenialTracker(5)

	assert.Nil(t, dt.GetRecord("nonexistent"))

	dt.RecordDenial("tool-a", "some input")
	rec := dt.GetRecord("tool-a")
	require.NotNil(t, rec)
	assert.Equal(t, "tool-a", rec.ToolName)
	assert.Equal(t, 1, rec.Count)
	assert.Equal(t, "some input", rec.LastInput)
	assert.False(t, rec.Escalated)
}

func TestDenialTracker_GetRecord_Escalated(t *testing.T) {
	dt := agentic.NewDenialTracker(1)

	dt.RecordDenial("tool-a", "input")
	rec := dt.GetRecord("tool-a")
	require.NotNil(t, rec)
	assert.True(t, rec.Escalated)
	assert.NotEmpty(t, rec.EscalationHint)
}

func TestDenialTracker_EscalationHints_Empty(t *testing.T) {
	dt := agentic.NewDenialTracker(5)
	assert.Empty(t, dt.EscalationHints())
}

func TestDenialTracker_EscalationHints_Multiple(t *testing.T) {
	dt := agentic.NewDenialTracker(1)

	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-b", "")

	hints := dt.EscalationHints()
	assert.Len(t, hints, 2)
}

func TestDenialTracker_DeniedTools(t *testing.T) {
	dt := agentic.NewDenialTracker(5)

	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-b", "")

	denied := dt.DeniedTools()
	assert.Equal(t, 2, denied["tool-a"])
	assert.Equal(t, 1, denied["tool-b"])
}

func TestDenialTracker_TotalDenials(t *testing.T) {
	dt := agentic.NewDenialTracker(5)

	assert.Equal(t, 0, dt.TotalDenials())

	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-a", "")
	dt.RecordDenial("tool-b", "")

	assert.Equal(t, 3, dt.TotalDenials())
}

func TestDenialTracker_RecordAllowOnNonexistent(t *testing.T) {
	dt := agentic.NewDenialTracker(3)
	// Should not panic.
	dt.RecordAllow("nonexistent")
	assert.Equal(t, 0, dt.TotalDenials())
}

func TestDenialTracker_ConcurrentAccess(t *testing.T) {
	dt := agentic.NewDenialTracker(100)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				dt.RecordDenial("tool-a", "input")
				dt.GetRecord("tool-a")
				dt.EscalationHints()
				dt.DeniedTools()
				dt.TotalDenials()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	assert.Equal(t, 500, dt.TotalDenials())
}

func TestDefaultRunConfig_DenialThreshold(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	assert.Equal(t, 3, cfg.DenialEscalationThreshold)
}
