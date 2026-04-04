package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DiagnosticSeverity ---

func TestDiagnosticSeverity_String(t *testing.T) {
	assert.Equal(t, "error", agentic.SeverityError.String())
	assert.Equal(t, "warning", agentic.SeverityWarning.String())
	assert.Equal(t, "info", agentic.SeverityInfo.String())
	assert.Equal(t, "hint", agentic.SeverityHint.String())
}

func TestDiagnosticSeverity_Order(t *testing.T) {
	assert.Less(t, int(agentic.SeverityError), int(agentic.SeverityWarning))
	assert.Less(t, int(agentic.SeverityWarning), int(agentic.SeverityInfo))
	assert.Less(t, int(agentic.SeverityInfo), int(agentic.SeverityHint))
}

// --- NewDiagnosticTracker ---

func TestNewDiagnosticTracker(t *testing.T) {
	tracker := agentic.NewDiagnosticTracker(nil)
	assert.NotNil(t, tracker)
	assert.Empty(t, tracker.TrackedFiles())
}

// --- CaptureBaseline ---

func TestDiagnosticTracker_CaptureBaseline(t *testing.T) {
	fetcher := func(path string) []agentic.Diagnostic {
		return []agentic.Diagnostic{
			{Severity: agentic.SeverityError, Message: "existing error"},
		}
	}

	tracker := agentic.NewDiagnosticTracker(fetcher)
	tracker.CaptureBaseline("/main.go")
	assert.Len(t, tracker.TrackedFiles(), 1)
}

func TestDiagnosticTracker_CaptureBaselineManual(t *testing.T) {
	tracker := agentic.NewDiagnosticTracker(nil)
	tracker.CaptureBaselineManual("/main.go", []agentic.Diagnostic{
		{Severity: agentic.SeverityWarning, Message: "unused var"},
	})
	assert.Len(t, tracker.TrackedFiles(), 1)
}

// --- GetNewDiagnostics ---

func TestDiagnosticTracker_GetNewDiagnostics_NoBaseline(t *testing.T) {
	fetcher := func(path string) []agentic.Diagnostic {
		return []agentic.Diagnostic{
			{Severity: agentic.SeverityError, Message: "new error"},
		}
	}

	tracker := agentic.NewDiagnosticTracker(fetcher)
	// No baseline captured — all current diagnostics are "new".
	diags := tracker.GetNewDiagnostics("/main.go")
	require.Len(t, diags, 1)
	assert.Equal(t, "new error", diags[0].Message)
}

func TestDiagnosticTracker_GetNewDiagnostics_ExistingFiltered(t *testing.T) {
	baseline := []agentic.Diagnostic{
		{Severity: agentic.SeverityError, Message: "existing error", Source: "go"},
	}
	current := []agentic.Diagnostic{
		{Severity: agentic.SeverityError, Message: "existing error", Source: "go"},
		{Severity: agentic.SeverityError, Message: "new error", Source: "go"},
	}

	tracker := agentic.NewDiagnosticTracker(func(_ string) []agentic.Diagnostic {
		return current
	})
	tracker.CaptureBaselineManual("/main.go", baseline)

	newDiags := tracker.GetNewDiagnostics("/main.go")
	require.Len(t, newDiags, 1)
	assert.Equal(t, "new error", newDiags[0].Message)
}

func TestDiagnosticTracker_GetNewDiagnostics_AllExisting(t *testing.T) {
	diags := []agentic.Diagnostic{
		{Severity: agentic.SeverityWarning, Message: "unused var"},
	}

	tracker := agentic.NewDiagnosticTracker(func(_ string) []agentic.Diagnostic {
		return diags
	})
	tracker.CaptureBaselineManual("/main.go", diags)

	assert.Empty(t, tracker.GetNewDiagnostics("/main.go"))
}

func TestDiagnosticTracker_GetNewDiagnostics_DuplicateHandling(t *testing.T) {
	// Baseline has 1 "unused var", current has 2 "unused var".
	baseline := []agentic.Diagnostic{
		{Severity: agentic.SeverityWarning, Message: "unused var"},
	}
	current := []agentic.Diagnostic{
		{Severity: agentic.SeverityWarning, Message: "unused var"},
		{Severity: agentic.SeverityWarning, Message: "unused var"},
	}

	tracker := agentic.NewDiagnosticTracker(func(_ string) []agentic.Diagnostic {
		return current
	})
	tracker.CaptureBaselineManual("/main.go", baseline)

	newDiags := tracker.GetNewDiagnostics("/main.go")
	require.Len(t, newDiags, 1, "should detect one new duplicate")
}

func TestDiagnosticTracker_GetNewDiagnostics_NilFetcher(t *testing.T) {
	tracker := agentic.NewDiagnosticTracker(nil)
	tracker.CaptureBaselineManual("/main.go", nil)
	assert.Nil(t, tracker.GetNewDiagnostics("/main.go"))
}

// --- GetAllNewDiagnostics ---

func TestDiagnosticTracker_GetAllNewDiagnostics(t *testing.T) {
	callNum := 0
	tracker := agentic.NewDiagnosticTracker(func(path string) []agentic.Diagnostic {
		callNum++
		if path == "/a.go" {
			return []agentic.Diagnostic{
				{Severity: agentic.SeverityError, Message: "error in a"},
			}
		}
		return nil
	})

	tracker.CaptureBaselineManual("/a.go", nil)
	tracker.CaptureBaselineManual("/b.go", nil)

	files := tracker.GetAllNewDiagnostics()
	require.Len(t, files, 1)
	assert.Equal(t, "/a.go", files[0].Path)
}

// --- HasNewErrors ---

func TestDiagnosticTracker_HasNewErrors_True(t *testing.T) {
	tracker := agentic.NewDiagnosticTracker(func(_ string) []agentic.Diagnostic {
		return []agentic.Diagnostic{
			{Severity: agentic.SeverityError, Message: "compile error"},
		}
	})
	tracker.CaptureBaselineManual("/main.go", nil)
	assert.True(t, tracker.HasNewErrors())
}

func TestDiagnosticTracker_HasNewErrors_OnlyWarnings(t *testing.T) {
	tracker := agentic.NewDiagnosticTracker(func(_ string) []agentic.Diagnostic {
		return []agentic.Diagnostic{
			{Severity: agentic.SeverityWarning, Message: "warning"},
		}
	})
	tracker.CaptureBaselineManual("/main.go", nil)
	assert.False(t, tracker.HasNewErrors())
}

func TestDiagnosticTracker_HasNewErrors_NoFiles(t *testing.T) {
	tracker := agentic.NewDiagnosticTracker(nil)
	assert.False(t, tracker.HasNewErrors())
}

// --- Reset ---

func TestDiagnosticTracker_Reset(t *testing.T) {
	tracker := agentic.NewDiagnosticTracker(nil)
	tracker.CaptureBaselineManual("/a.go", nil)
	tracker.CaptureBaselineManual("/b.go", nil)

	tracker.Reset()
	assert.Empty(t, tracker.TrackedFiles())
}

// --- FormatDiagnosticSummary ---

func TestFormatDiagnosticSummary_Empty(t *testing.T) {
	summary := agentic.FormatDiagnosticSummary(nil)
	assert.Equal(t, "No new diagnostics.", summary)
}

func TestFormatDiagnosticSummary_WithDiagnostics(t *testing.T) {
	files := []agentic.DiagnosticFile{
		{
			Path: "/main.go",
			Diagnostics: []agentic.Diagnostic{
				{Severity: agentic.SeverityError, Message: "undefined: foo", Line: 42, Column: 5, Code: "E001"},
				{Severity: agentic.SeverityWarning, Message: "unused import", Line: 3},
			},
		},
	}

	summary := agentic.FormatDiagnosticSummary(files)
	assert.Contains(t, summary, "/main.go")
	assert.Contains(t, summary, "undefined: foo")
	assert.Contains(t, summary, ":42:5")
	assert.Contains(t, summary, "(E001)")
	assert.Contains(t, summary, "unused import")
	assert.Contains(t, summary, "1 error(s), 1 warning(s)")
}

func TestFormatDiagnosticSummary_MultipleFiles(t *testing.T) {
	files := []agentic.DiagnosticFile{
		{Path: "/a.go", Diagnostics: []agentic.Diagnostic{
			{Severity: agentic.SeverityError, Message: "err1"},
		}},
		{Path: "/b.go", Diagnostics: []agentic.Diagnostic{
			{Severity: agentic.SeverityError, Message: "err2"},
		}},
	}

	summary := agentic.FormatDiagnosticSummary(files)
	assert.Contains(t, summary, "/a.go")
	assert.Contains(t, summary, "/b.go")
	assert.Contains(t, summary, "2 error(s)")
}

// --- Diagnostic fields ---

func TestDiagnostic_Fields(t *testing.T) {
	d := agentic.Diagnostic{
		Severity: agentic.SeverityError,
		Message:  "test error",
		Source:   "go vet",
		Code:     "E100",
		Line:     10,
		Column:   5,
	}
	assert.Equal(t, agentic.SeverityError, d.Severity)
	assert.Equal(t, "go vet", d.Source)
	assert.Equal(t, 10, d.Line)
}
