// Package agentic implements the agentic loop where the LLM decides which
// tools to invoke, replacing the previous pipeline/DAG execution model.
package agentic

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

// SkillLister returns skills linked to an agent.
type SkillLister interface {
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]skill.Skill, error)
}

// KBLister returns knowledge bases linked to an agent.
type KBLister interface {
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]knowledgebase.KnowledgeBase, error)
}

// CompactSummaryFinder returns the latest compact summary for a session.
type CompactSummaryFinder interface {
	GetLatestCompactSummary(ctx context.Context, sessionID uuid.UUID) (chat.ChatMessage, bool, error)
}

// PromptConfig holds tuneable parameters for the prompt builder.
type PromptConfig struct {
	// MaxEstimatedTokens is the soft limit for the system prompt (chars/4 heuristic).
	// Default: 4000 tokens ≈ 16000 chars.
	MaxEstimatedTokens int
}

// DefaultPromptConfig returns sensible defaults.
func DefaultPromptConfig() PromptConfig {
	return PromptConfig{MaxEstimatedTokens: 4000}
}

// PromptBuilder assembles the full system prompt for an agentic chat session.
type PromptBuilder struct {
	skills  SkillLister
	kbs     KBLister
	summFn  CompactSummaryFinder
	config  PromptConfig
}

// NewPromptBuilder creates a PromptBuilder.
func NewPromptBuilder(skills SkillLister, kbs KBLister, summFn CompactSummaryFinder, cfg PromptConfig) *PromptBuilder {
	if cfg.MaxEstimatedTokens == 0 {
		cfg = DefaultPromptConfig()
	}
	return &PromptBuilder{skills: skills, kbs: kbs, summFn: summFn, config: cfg}
}

// PromptInput carries everything the builder needs to compose the system prompt.
type PromptInput struct {
	// AgentID is used to look up skills and knowledge bases.
	AgentID uuid.UUID
	// SessionID is used to find the latest compact summary.
	SessionID uuid.UUID
	// SystemPrompt is the agent's custom identity/instructions (editable by user).
	SystemPrompt string
	// Memories is a pre-formatted block of recalled memories (injected by MemoryBridge).
	Memories string
	// CoordinatorMode enables coordinator instructions when sub-agent spawning is available.
	CoordinatorMode bool
}

// Build assembles the full system prompt from all dynamic sections.
func (b *PromptBuilder) Build(ctx context.Context, in PromptInput) (string, error) {
	var sections []string

	// 1. Agent Identity & Instructions
	if in.SystemPrompt != "" {
		sections = append(sections, in.SystemPrompt)
	}

	// 2. Available Tools
	if b.skills != nil {
		skills, err := b.skills.ListByAgentID(ctx, in.AgentID)
		if err != nil {
			return "", fmt.Errorf("prompt: list skills: %w", err)
		}
		if len(skills) > 0 {
			sections = append(sections, formatToolsSection(skills))
		}
	}

	// 3. Tool Usage Instructions (static rules)
	sections = append(sections, toolUsageInstructions)

	// 3b. Coordinator instructions (when sub-agent spawning is enabled).
	if in.CoordinatorMode {
		sections = append(sections, coordinatorInstructions)
	}

	// 4. Knowledge Base Context
	if b.kbs != nil {
		kbs, err := b.kbs.ListByAgentID(ctx, in.AgentID)
		if err != nil {
			return "", fmt.Errorf("prompt: list knowledge bases: %w", err)
		}
		if len(kbs) > 0 {
			sections = append(sections, formatKBSection(kbs))
		}
	}

	// 5. Memories
	if in.Memories != "" {
		sections = append(sections, "## Relevant Memories\n\n"+in.Memories)
	}

	// 6. Conversation Summary (compact_summary)
	if b.summFn != nil {
		msg, found, err := b.summFn.GetLatestCompactSummary(ctx, in.SessionID)
		if err != nil {
			return "", fmt.Errorf("prompt: get compact summary: %w", err)
		}
		if found && msg.Content != "" {
			sections = append(sections, "## Conversation Summary\n\n"+msg.Content)
		}
	}

	prompt := strings.Join(sections, "\n\n---\n\n")

	// Soft-truncate if the prompt exceeds the estimated token budget.
	maxChars := b.config.MaxEstimatedTokens * 4
	if len(prompt) > maxChars {
		prompt = prompt[:maxChars]
	}

	return prompt, nil
}

// formatToolsSection produces a markdown block listing the available skills.
func formatToolsSection(skills []skill.Skill) string {
	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	for _, s := range skills {
		fmt.Fprintf(&sb, "- **%s** (`%s`)", s.Name, s.Slug)
		if s.Description != "" {
			sb.WriteString(": " + s.Description)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatKBSection produces a markdown block listing the agent's knowledge bases.
func formatKBSection(kbs []knowledgebase.KnowledgeBase) string {
	var sb strings.Builder
	sb.WriteString("## Knowledge Bases\n\n")
	for _, kb := range kbs {
		fmt.Fprintf(&sb, "- **%s**", kb.Name)
		if kb.DocumentCount > 0 {
			fmt.Fprintf(&sb, " (%d documents)", kb.DocumentCount)
		}
		if kb.Description != "" {
			sb.WriteString(" — " + kb.Description)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// coordinatorInstructions is injected when the agent tool is available for sub-agent spawning.
const coordinatorInstructions = `## Coordinator Mode

You can spawn sub-agents to handle subtasks in parallel using the 'agent' tool.

### When to Delegate
- Tasks that are independent and can run concurrently
- Complex tasks that benefit from decomposition into focused subtasks
- Tasks that require different tool expertise

### When NOT to Delegate
- Simple tasks you can handle directly with a single tool call
- Tasks with strong sequential dependencies
- Trivial questions that don't require tool usage

### Guidelines
- Write clear, specific prompts — sub-agents start with zero conversation context
- Include all necessary context in the prompt
- Spawn multiple sub-agents simultaneously for independent subtasks
- Synthesize sub-agent results into a coherent final answer`

// toolUsageInstructions is the static section injected into every agentic prompt.
const toolUsageInstructions = `## Tool Usage Instructions

- Use available tools to answer questions that require data retrieval or actions.
- Use document_search when the user asks about topics covered by the knowledge bases.
- NEVER execute operations that modify data without confirming with the user first.
- When you receive tool results, synthesize them into a clear, concise answer.
- If a tool call fails, explain the error and suggest an alternative approach.
- Do not fabricate data — if you do not have the information, say so.
- Cite document sources when answering from knowledge base results.`
