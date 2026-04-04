package agentic

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// CommandContext carries information available to slash command handlers.
type CommandContext struct {
	SessionID string
	AgentID   string
	TenantID  string
}

// CommandResult holds the output of a slash command execution.
type CommandResult struct {
	Output string
}

// SlashCommand defines a single slash command.
type SlashCommand struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args string, cc CommandContext) (CommandResult, error)
}

// CommandRegistry manages slash commands for the chat.
type CommandRegistry struct {
	commands map[string]SlashCommand
}

// NewCommandRegistry creates a registry with builtin commands pre-registered.
func NewCommandRegistry() *CommandRegistry {
	reg := &CommandRegistry{
		commands: make(map[string]SlashCommand),
	}
	reg.registerBuiltins()
	return reg
}

// Register adds a custom command. Returns error if name is already taken.
func (r *CommandRegistry) Register(cmd SlashCommand) error {
	name := strings.ToLower(cmd.Name)
	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("command '%s' already registered", name)
	}
	cmd.Name = name
	r.commands[name] = cmd
	return nil
}

// Parse checks if input starts with '/' and extracts the command and args.
// Returns nil if the input is not a slash command.
func (r *CommandRegistry) Parse(input string) (cmd *SlashCommand, args string, ok bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil, "", false
	}

	// Split into command name and args.
	parts := strings.SplitN(input[1:], " ", 2)
	name := strings.ToLower(parts[0])
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	if name == "" {
		return nil, "", false
	}

	c, exists := r.commands[name]
	if !exists {
		return nil, "", false
	}

	return &c, args, true
}

// Execute runs a parsed slash command.
func (r *CommandRegistry) Execute(ctx context.Context, cmd *SlashCommand, args string, cc CommandContext) (CommandResult, error) {
	if cmd == nil || cmd.Handler == nil {
		return CommandResult{}, fmt.Errorf("invalid command")
	}
	return cmd.Handler(ctx, args, cc)
}

// List returns all registered commands sorted by name.
func (r *CommandRegistry) List() []SlashCommand {
	cmds := make([]SlashCommand, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})
	return cmds
}

// registerBuiltins adds the default slash commands.
func (r *CommandRegistry) registerBuiltins() {
	r.commands["help"] = SlashCommand{
		Name:        "help",
		Description: "List available commands",
		Handler:     r.helpHandler,
	}
	r.commands["clear"] = SlashCommand{
		Name:        "clear",
		Description: "Clear conversation history",
		Handler:     clearHandler,
	}
	r.commands["compact"] = SlashCommand{
		Name:        "compact",
		Description: "Force context compaction",
		Handler:     compactHandler,
	}
	r.commands["status"] = SlashCommand{
		Name:        "status",
		Description: "Show agent status (tools, knowledge bases)",
		Handler:     statusHandler,
	}
	r.commands["cost"] = SlashCommand{
		Name:        "cost",
		Description: "Show token usage and cost for this session",
		Handler:     costHandler,
	}
}

func (r *CommandRegistry) helpHandler(_ context.Context, _ string, _ CommandContext) (CommandResult, error) {
	var sb strings.Builder
	sb.WriteString("**Available Commands:**\n\n")
	for _, cmd := range r.List() {
		sb.WriteString(fmt.Sprintf("- `/%s` — %s\n", cmd.Name, cmd.Description))
	}
	return CommandResult{Output: sb.String()}, nil
}

func clearHandler(_ context.Context, _ string, _ CommandContext) (CommandResult, error) {
	// The actual clearing is handled by the service layer.
	// This handler returns a signal that the service interprets.
	return CommandResult{Output: "[command:clear] Conversation history cleared."}, nil
}

func compactHandler(_ context.Context, _ string, _ CommandContext) (CommandResult, error) {
	return CommandResult{Output: "[command:compact] Context compaction requested."}, nil
}

func statusHandler(_ context.Context, _ string, cc CommandContext) (CommandResult, error) {
	return CommandResult{Output: fmt.Sprintf(
		"**Agent Status**\n- Agent: `%s`\n- Session: `%s`\n- Tenant: `%s`",
		cc.AgentID, cc.SessionID, cc.TenantID,
	)}, nil
}

func costHandler(_ context.Context, _ string, cc CommandContext) (CommandResult, error) {
	return CommandResult{Output: fmt.Sprintf(
		"**Session Cost**\n- Session: `%s`\n- (detailed cost tracking available via analytics)",
		cc.SessionID,
	)}, nil
}
