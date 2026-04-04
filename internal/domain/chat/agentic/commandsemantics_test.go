package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- InterpretCommandResult: grep ---

func TestCommandSemantics_GrepNoMatch(t *testing.T) {
	r := agentic.InterpretCommandResult("grep pattern file.txt", 1, "", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "No matches found", r.Message)
}

func TestCommandSemantics_GrepMatch(t *testing.T) {
	r := agentic.InterpretCommandResult("grep pattern file.txt", 0, "match", "")
	assert.False(t, r.IsError)
}

func TestCommandSemantics_GrepError(t *testing.T) {
	r := agentic.InterpretCommandResult("grep pattern", 2, "", "error")
	assert.True(t, r.IsError)
}

// --- InterpretCommandResult: rg ---

func TestCommandSemantics_RgNoMatch(t *testing.T) {
	r := agentic.InterpretCommandResult("rg pattern", 1, "", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "No matches found", r.Message)
}

// --- InterpretCommandResult: diff ---

func TestCommandSemantics_DiffFilesDiffer(t *testing.T) {
	r := agentic.InterpretCommandResult("diff a.txt b.txt", 1, "< line", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "Files differ", r.Message)
}

func TestCommandSemantics_DiffSame(t *testing.T) {
	r := agentic.InterpretCommandResult("diff a.txt b.txt", 0, "", "")
	assert.False(t, r.IsError)
}

func TestCommandSemantics_DiffError(t *testing.T) {
	r := agentic.InterpretCommandResult("diff a.txt b.txt", 2, "", "not found")
	assert.True(t, r.IsError)
}

// --- InterpretCommandResult: find ---

func TestCommandSemantics_FindPartialSuccess(t *testing.T) {
	r := agentic.InterpretCommandResult("find / -name foo", 1, "/some/path", "permission denied")
	assert.False(t, r.IsError)
	assert.Equal(t, "Some directories were inaccessible", r.Message)
}

func TestCommandSemantics_FindError(t *testing.T) {
	r := agentic.InterpretCommandResult("find / -name foo", 2, "", "error")
	assert.True(t, r.IsError)
}

// --- InterpretCommandResult: test/[ ---

func TestCommandSemantics_TestFalse(t *testing.T) {
	r := agentic.InterpretCommandResult("test -f /nonexistent", 1, "", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "Condition is false", r.Message)
}

func TestCommandSemantics_BracketFalse(t *testing.T) {
	r := agentic.InterpretCommandResult("[ -f /nonexistent ]", 1, "", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "Condition is false", r.Message)
}

func TestCommandSemantics_TestTrue(t *testing.T) {
	r := agentic.InterpretCommandResult("test -f /etc/passwd", 0, "", "")
	assert.False(t, r.IsError)
}

// --- InterpretCommandResult: default ---

func TestCommandSemantics_DefaultSuccess(t *testing.T) {
	r := agentic.InterpretCommandResult("ls -la", 0, "file.txt", "")
	assert.False(t, r.IsError)
	assert.Empty(t, r.Message)
}

func TestCommandSemantics_DefaultFailure(t *testing.T) {
	r := agentic.InterpretCommandResult("ls -la", 1, "", "not found")
	assert.True(t, r.IsError)
	assert.Contains(t, r.Message, "exit code 1")
}

// --- Piped commands ---

func TestCommandSemantics_PipedGrep(t *testing.T) {
	// In "cat file | grep pattern", grep determines exit code
	r := agentic.InterpretCommandResult("cat file | grep pattern", 1, "", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "No matches found", r.Message)
}

func TestCommandSemantics_ChainedCommand(t *testing.T) {
	r := agentic.InterpretCommandResult("cd /tmp && diff a b", 1, "", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "Files differ", r.Message)
}

func TestCommandSemantics_SemicolonChained(t *testing.T) {
	r := agentic.InterpretCommandResult("echo hello; test -f /x", 1, "", "")
	assert.False(t, r.IsError)
	assert.Equal(t, "Condition is false", r.Message)
}

// --- Empty command ---

func TestCommandSemantics_Empty(t *testing.T) {
	r := agentic.InterpretCommandResult("", 0, "", "")
	assert.False(t, r.IsError)
}
