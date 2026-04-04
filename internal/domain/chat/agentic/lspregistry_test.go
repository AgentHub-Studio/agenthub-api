package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- LSPDiagnosticSeverity ---

func TestLSPSeverity_String(t *testing.T) {
	assert.Equal(t, "error", agentic.LSPSeverityError.String())
	assert.Equal(t, "warning", agentic.LSPSeverityWarning.String())
	assert.Equal(t, "info", agentic.LSPSeverityInfo.String())
	assert.Equal(t, "hint", agentic.LSPSeverityHint.String())
	assert.Equal(t, "unknown", agentic.LSPDiagnosticSeverity(99).String())
}

// --- NewLSPDiagnosticRegistry ---

func TestNewLSPDiagnosticRegistry(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.PendingCount())
}

// --- Register ---

func TestLSPDiagnosticRegistry_Register(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("typescript", "/main.ts", []agentic.LSPDiagnosticItem{
		{Severity: agentic.LSPSeverityError, Message: "type error"},
	})
	assert.Equal(t, 1, r.PendingCount())
}

func TestLSPDiagnosticRegistry_Register_ReplacesSameFile(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/main.ts", []agentic.LSPDiagnosticItem{
		{Message: "error 1"},
	})
	r.Register("ts", "/main.ts", []agentic.LSPDiagnosticItem{
		{Message: "error 2"},
		{Message: "error 3"},
	})

	pending := r.GetPending()
	assert.Len(t, pending, 1)
	assert.Len(t, pending[0].Diagnostics, 2)
}

func TestLSPDiagnosticRegistry_Register_TrimsPerFileLimit(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()

	diags := make([]agentic.LSPDiagnosticItem, 20)
	for i := range diags {
		diags[i] = agentic.LSPDiagnosticItem{Message: "error"}
	}

	r.Register("ts", "/main.ts", diags)
	pending := r.GetPending()
	assert.Len(t, pending[0].Diagnostics, agentic.LSPMaxDiagnosticsPerFile)
}

// --- GetPending ---

func TestLSPDiagnosticRegistry_GetPending(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{{Message: "err1"}})
	r.Register("ts", "/b.ts", []agentic.LSPDiagnosticItem{{Message: "err2"}})

	pending := r.GetPending()
	assert.Len(t, pending, 2)
}

func TestLSPDiagnosticRegistry_GetPending_ExcludesSent(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{{Message: "err"}})
	r.MarkSent("/a.ts")

	pending := r.GetPending()
	assert.Len(t, pending, 0)
}

// --- MarkSent ---

func TestLSPDiagnosticRegistry_MarkSent(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{{Message: "err"}})
	r.MarkSent("/a.ts")

	assert.Equal(t, 0, r.PendingCount())
}

func TestLSPDiagnosticRegistry_MarkSent_NonExistent(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.MarkSent("/nonexistent.ts") // should not panic
}

// --- Clear ---

func TestLSPDiagnosticRegistry_Clear(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{{Message: "err"}})
	r.Clear()
	assert.Equal(t, 0, r.PendingCount())
}

func TestLSPDiagnosticRegistry_ClearFile(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{{Message: "err1"}})
	r.Register("ts", "/b.ts", []agentic.LSPDiagnosticItem{{Message: "err2"}})

	r.ClearFile("/a.ts")
	assert.Equal(t, 1, r.PendingCount())
}

// --- HasErrors ---

func TestLSPDiagnosticRegistry_HasErrors_True(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{
		{Severity: agentic.LSPSeverityError, Message: "err"},
	})
	assert.True(t, r.HasErrors())
}

func TestLSPDiagnosticRegistry_HasErrors_False(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{
		{Severity: agentic.LSPSeverityWarning, Message: "warn"},
	})
	assert.False(t, r.HasErrors())
}

func TestLSPDiagnosticRegistry_HasErrors_Empty(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	assert.False(t, r.HasErrors())
}

// --- FormatSummary ---

func TestLSPDiagnosticRegistry_FormatSummary(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	r.Register("ts", "/a.ts", []agentic.LSPDiagnosticItem{
		{Severity: agentic.LSPSeverityError, Message: "e1"},
		{Severity: agentic.LSPSeverityError, Message: "e2"},
		{Severity: agentic.LSPSeverityWarning, Message: "w1"},
		{Severity: agentic.LSPSeverityInfo, Message: "i1"},
	})

	s := r.FormatSummary()
	assert.Contains(t, s, "2 errors")
	assert.Contains(t, s, "1 warnings")
	assert.Contains(t, s, "1 info")
}

func TestLSPDiagnosticRegistry_FormatSummary_Empty(t *testing.T) {
	r := agentic.NewLSPDiagnosticRegistry()
	assert.Equal(t, "no diagnostics", r.FormatSummary())
}

// --- Constants ---

func TestLSP_Constants(t *testing.T) {
	assert.Equal(t, 10, agentic.LSPMaxDiagnosticsPerFile)
	assert.Equal(t, 30, agentic.LSPMaxTotalDiagnostics)
	assert.Equal(t, 500, agentic.LSPMaxTrackedFiles)
}
