package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewStreamTransformer ---

func TestNewStreamTransformer(t *testing.T) {
	tr := agentic.NewStreamTransformer(nil)
	assert.NotNil(t, tr)
	assert.Equal(t, 0, tr.Counts().Total())
}

// --- Transform text message ---

func TestStreamTransformer_TextMessage(t *testing.T) {
	tr := agentic.NewStreamTransformer(nil)
	out := tr.Transform(agentic.StreamTransformMessage{
		ID:   "m1",
		Type: "assistant",
		Text: "Hello world",
	})
	assert.Equal(t, agentic.StreamTransformText, out.Type)
	assert.Equal(t, "Hello world", out.Text)
}

// --- Transform tool-only message ---

func TestStreamTransformer_ToolOnlyMessage(t *testing.T) {
	cat := agentic.DefaultToolCategorizer(
		[]string{"grep", "glob"},
		[]string{"read"},
		[]string{"write", "edit"},
		[]string{"bash"},
	)
	tr := agentic.NewStreamTransformer(cat)

	out := tr.Transform(agentic.StreamTransformMessage{
		ID:        "m1",
		Type:      "assistant",
		ToolNames: []string{"grep", "read", "write"},
	})
	assert.Equal(t, agentic.StreamTransformToolSummary, out.Type)
	assert.Contains(t, out.ToolSummary, "Searched 1 pattern")
	assert.Contains(t, out.ToolSummary, "read 1 file")
	assert.Contains(t, out.ToolSummary, "wrote 1 file")
}

// --- Cumulative tool counts across messages ---

func TestStreamTransformer_CumulativeCounts(t *testing.T) {
	cat := agentic.DefaultToolCategorizer(
		[]string{"grep"}, nil, nil, nil,
	)
	tr := agentic.NewStreamTransformer(cat)

	tr.Transform(agentic.StreamTransformMessage{
		ID: "m1", Type: "assistant", ToolNames: []string{"grep"},
	})
	out := tr.Transform(agentic.StreamTransformMessage{
		ID: "m2", Type: "assistant", ToolNames: []string{"grep"},
	})

	assert.Contains(t, out.ToolSummary, "Searched 2 patterns")
}

// --- Text resets counts ---

func TestStreamTransformer_TextResetsCounts(t *testing.T) {
	cat := agentic.DefaultToolCategorizer(
		[]string{"grep"}, nil, nil, nil,
	)
	tr := agentic.NewStreamTransformer(cat)

	tr.Transform(agentic.StreamTransformMessage{
		ID: "m1", Type: "assistant", ToolNames: []string{"grep", "grep"},
	})
	assert.Equal(t, 2, tr.Counts().Searches)

	// Text resets
	tr.Transform(agentic.StreamTransformMessage{
		ID: "m2", Type: "assistant", Text: "Here are results",
	})
	assert.Equal(t, 0, tr.Counts().Total())
}

// --- Skipped message types ---

func TestStreamTransformer_SkipsSystem(t *testing.T) {
	tr := agentic.NewStreamTransformer(nil)
	out := tr.Transform(agentic.StreamTransformMessage{
		ID: "m1", Type: "system", Text: "system msg",
	})
	assert.Equal(t, agentic.StreamTransformSkipped, out.Type)
}

func TestStreamTransformer_SkipsUser(t *testing.T) {
	tr := agentic.NewStreamTransformer(nil)
	out := tr.Transform(agentic.StreamTransformMessage{
		ID: "m1", Type: "user", Text: "user msg",
	})
	assert.Equal(t, agentic.StreamTransformSkipped, out.Type)
}

func TestStreamTransformer_Result(t *testing.T) {
	tr := agentic.NewStreamTransformer(nil)
	out := tr.Transform(agentic.StreamTransformMessage{
		ID: "m1", Type: "result",
	})
	assert.Equal(t, agentic.StreamTransformResult, out.Type)
}

// --- Empty tool-only message ---

func TestStreamTransformer_EmptyToolMessage(t *testing.T) {
	tr := agentic.NewStreamTransformer(nil)
	out := tr.Transform(agentic.StreamTransformMessage{
		ID: "m1", Type: "assistant",
	})
	assert.Equal(t, agentic.StreamTransformSkipped, out.Type)
}

// --- DefaultToolCategorizer ---

func TestDefaultToolCategorizer(t *testing.T) {
	cat := agentic.DefaultToolCategorizer(
		[]string{"grep", "glob"},
		[]string{"read_file"},
		[]string{"write_file", "edit_file"},
		[]string{"bash", "tmux"},
	)
	assert.Equal(t, agentic.StreamCatSearch, cat("grep"))
	assert.Equal(t, agentic.StreamCatSearch, cat("glob_pattern"))
	assert.Equal(t, agentic.StreamCatRead, cat("read_file"))
	assert.Equal(t, agentic.StreamCatWrite, cat("write_file"))
	assert.Equal(t, agentic.StreamCatWrite, cat("edit_file"))
	assert.Equal(t, agentic.StreamCatCommand, cat("bash"))
	assert.Equal(t, agentic.StreamCatCommand, cat("tmux"))
	assert.Equal(t, agentic.StreamCatOther, cat("unknown_tool"))
}

// --- FormatToolSummary ---

func TestFormatToolSummary_Empty(t *testing.T) {
	s := agentic.FormatToolSummary(agentic.StreamTransformCounts{})
	assert.Equal(t, "", s)
}

func TestFormatToolSummary_SingleCategory(t *testing.T) {
	s := agentic.FormatToolSummary(agentic.StreamTransformCounts{Reads: 3})
	assert.Equal(t, "Read 3 files", s)
}

func TestFormatToolSummary_Multiple(t *testing.T) {
	s := agentic.FormatToolSummary(agentic.StreamTransformCounts{
		Searches: 2,
		Writes:   1,
		Commands: 3,
	})
	assert.Contains(t, s, "Searched 2 patterns")
	assert.Contains(t, s, "wrote 1 file")
	assert.Contains(t, s, "ran 3 commands")
}

func TestFormatToolSummary_Singular(t *testing.T) {
	s := agentic.FormatToolSummary(agentic.StreamTransformCounts{
		Searches: 1, Reads: 1, Writes: 1, Commands: 1, Other: 1,
	})
	assert.Contains(t, s, "1 pattern")
	assert.Contains(t, s, "1 file")
	assert.Contains(t, s, "1 command")
	assert.Contains(t, s, "1 other tool")
}

// --- Reset ---

func TestStreamTransformer_Reset(t *testing.T) {
	tr := agentic.NewStreamTransformer(nil)
	tr.Transform(agentic.StreamTransformMessage{
		ID: "m1", Type: "assistant", ToolNames: []string{"a", "b"},
	})
	assert.Equal(t, 2, tr.Counts().Total())

	tr.Reset()
	assert.Equal(t, 0, tr.Counts().Total())
}

// --- StreamTransformCounts.Total ---

func TestStreamTransformCounts_Total(t *testing.T) {
	c := agentic.StreamTransformCounts{
		Searches: 1, Reads: 2, Writes: 3, Commands: 4, Other: 5,
	}
	assert.Equal(t, 15, c.Total())
}
