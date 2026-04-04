package agentic_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestCommandRegistry_Parse_SlashCommand(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmd, args, ok := reg.Parse("/help")
	require.True(t, ok)
	assert.Equal(t, "help", cmd.Name)
	assert.Empty(t, args)
}

func TestCommandRegistry_Parse_WithArgs(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmd, args, ok := reg.Parse("/status some args here")
	require.True(t, ok)
	assert.Equal(t, "status", cmd.Name)
	assert.Equal(t, "some args here", args)
}

func TestCommandRegistry_Parse_CaseInsensitive(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmd, _, ok := reg.Parse("/HELP")
	require.True(t, ok)
	assert.Equal(t, "help", cmd.Name)
}

func TestCommandRegistry_Parse_NotACommand(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	_, _, ok := reg.Parse("Hello world")
	assert.False(t, ok)
}

func TestCommandRegistry_Parse_EmptySlash(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	_, _, ok := reg.Parse("/")
	assert.False(t, ok)
}

func TestCommandRegistry_Parse_UnknownCommand(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	_, _, ok := reg.Parse("/nonexistent")
	assert.False(t, ok)
}

func TestCommandRegistry_Parse_WhitespaceHandling(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmd, _, ok := reg.Parse("  /help  ")
	require.True(t, ok)
	assert.Equal(t, "help", cmd.Name)
}

func TestCommandRegistry_Execute_Help(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmd, _, _ := reg.Parse("/help")
	result, err := reg.Execute(context.Background(), cmd, "", agentic.CommandContext{})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "/help")
	assert.Contains(t, result.Output, "/clear")
	assert.Contains(t, result.Output, "/compact")
	assert.Contains(t, result.Output, "/status")
	assert.Contains(t, result.Output, "/cost")
}

func TestCommandRegistry_Execute_Clear(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmd, _, _ := reg.Parse("/clear")
	result, err := reg.Execute(context.Background(), cmd, "", agentic.CommandContext{})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "clear")
}

func TestCommandRegistry_Execute_Compact(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmd, _, _ := reg.Parse("/compact")
	result, err := reg.Execute(context.Background(), cmd, "", agentic.CommandContext{})
	require.NoError(t, err)
	assert.Contains(t, result.Output, "compact")
}

func TestCommandRegistry_Execute_Status(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cc := agentic.CommandContext{
		SessionID: "sess-123",
		AgentID:   "agent-456",
		TenantID:  "my-tenant",
	}

	cmd, _, _ := reg.Parse("/status")
	result, err := reg.Execute(context.Background(), cmd, "", cc)
	require.NoError(t, err)
	assert.Contains(t, result.Output, "agent-456")
	assert.Contains(t, result.Output, "sess-123")
	assert.Contains(t, result.Output, "my-tenant")
}

func TestCommandRegistry_Execute_Cost(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cc := agentic.CommandContext{SessionID: "sess-789"}
	cmd, _, _ := reg.Parse("/cost")
	result, err := reg.Execute(context.Background(), cmd, "", cc)
	require.NoError(t, err)
	assert.Contains(t, result.Output, "sess-789")
}

func TestCommandRegistry_Execute_NilCommand(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	_, err := reg.Execute(context.Background(), nil, "", agentic.CommandContext{})
	assert.Error(t, err)
}

func TestCommandRegistry_Register_Custom(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	err := reg.Register(agentic.SlashCommand{
		Name:        "debug",
		Description: "Debug info",
		Handler: func(ctx context.Context, args string, cc agentic.CommandContext) (agentic.CommandResult, error) {
			return agentic.CommandResult{Output: "debug: " + args}, nil
		},
	})
	require.NoError(t, err)

	cmd, args, ok := reg.Parse("/debug test-arg")
	require.True(t, ok)
	result, err := reg.Execute(context.Background(), cmd, args, agentic.CommandContext{})
	require.NoError(t, err)
	assert.Equal(t, "debug: test-arg", result.Output)
}

func TestCommandRegistry_Register_Duplicate(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	err := reg.Register(agentic.SlashCommand{Name: "help", Description: "dup"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestCommandRegistry_List(t *testing.T) {
	reg := agentic.NewCommandRegistry()

	cmds := reg.List()
	assert.GreaterOrEqual(t, len(cmds), 5) // help, clear, compact, status, cost

	// Should be sorted.
	for i := 1; i < len(cmds); i++ {
		assert.True(t, cmds[i-1].Name < cmds[i].Name, "commands should be sorted")
	}
}
