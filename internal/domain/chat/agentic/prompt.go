// Package agentic implements the agentic loop where the LLM decides which
// tools to invoke, replacing the previous pipeline/DAG execution model.
package agentic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// SkillLister returns skills.
type SkillLister interface {
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]skill.Skill, error)
	// ListByIDs returns the skills with the given IDs. Used to load a session's
	// snapshotted skill bindings (P-C115-1).
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]skill.Skill, error)
	List(ctx context.Context, category *string, req pagination.PageRequest) ([]skill.Skill, int64, error)
}

// KBLister returns knowledge bases.
type KBLister interface {
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]knowledgebase.KnowledgeBase, error)
	List(ctx context.Context, req pagination.PageRequest) ([]knowledgebase.KnowledgeBase, int64, error)
}

// CompactSummaryFinder returns the latest compact summary for a session.
type CompactSummaryFinder interface {
	GetLatestCompactSummary(ctx context.Context, sessionID uuid.UUID) (chat.ChatMessage, bool, error)
}

// PromptTemplateResolver resolves configurable prompt sections by slug.
// Agent-specific templates should override global templates when both exist.
type PromptTemplateResolver interface {
	ResolvePromptTemplate(ctx context.Context, agentID uuid.UUID, slug string) (content string, found bool, err error)
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
// Sections are cached after first computation and only recomputed on ClearCache().
// This mirrors Claude Code's systemPromptSection memoization pattern that
// computes sections once and caches until /clear or /compact.
type PromptBuilder struct {
	skills SkillLister
	kbs    KBLister
	summFn CompactSummaryFinder
	tpl    PromptTemplateResolver
	config PromptConfig
	// sectionCache stores computed prompt sections by name.
	// Cache is cleared on context compaction or explicit reset.
	// Inspired by Claude Code's systemPromptSectionCache in state.ts.
	sectionCache map[string]string
}

// NewPromptBuilder creates a PromptBuilder.
func NewPromptBuilder(skills SkillLister, kbs KBLister, summFn CompactSummaryFinder, cfg PromptConfig) *PromptBuilder {
	if cfg.MaxEstimatedTokens == 0 {
		cfg = DefaultPromptConfig()
	}
	return &PromptBuilder{
		skills:       skills,
		kbs:          kbs,
		summFn:       summFn,
		config:       cfg,
		sectionCache: make(map[string]string),
	}
}

// WithPromptTemplateResolver attaches an optional resolver used to load prompt
// sections from the prompt_template table. Missing templates fall back to the
// built-in constants.
func (b *PromptBuilder) WithPromptTemplateResolver(resolver PromptTemplateResolver) *PromptBuilder {
	b.tpl = resolver
	return b
}

// ClearCache resets all cached prompt sections.
// Should be called on context compaction or /clear to allow section recomputation.
// Inspired by Claude Code's clearSystemPromptSections() in systemPromptSections.ts.
func (b *PromptBuilder) ClearCache() {
	b.sectionCache = make(map[string]string)
}

// ClearCacheForAgent removes all cached sections for the given agent.
// Should be called at the start of each new run so edits to skills, tools,
// or KBs are reflected without requiring a server restart.
// P-C343-1: per-agent cache invalidation on run start.
func (b *PromptBuilder) ClearCacheForAgent(agentID uuid.UUID) {
	suffix := ":" + agentID.String()
	for k := range b.sectionCache {
		if strings.HasSuffix(k, suffix) {
			delete(b.sectionCache, k)
		}
	}
}

// getCachedOrCompute returns a cached section or computes and caches it.
func (b *PromptBuilder) getCachedOrCompute(name string, compute func() (string, error)) (string, error) {
	if cached, ok := b.sectionCache[name]; ok {
		return cached, nil
	}
	result, err := compute()
	if err != nil {
		return "", err
	}
	if result != "" {
		b.sectionCache[name] = result
	}
	return result, nil
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
	// DeferredToolNames lists tools whose schemas are not in the initial tools[] array.
	// The LLM can load them on demand via the tool_search builtin.
	// Inspired by Claude Code's deferred tool announcement in system prompt.
	DeferredToolNames []string
	// UserOnlySkills lists skills with disableModelInvocation=true. These are
	// announced in the prompt so the LLM can suggest them to the user, but the LLM
	// cannot invoke them directly. Inspired by Claude Code's user-invocable skills
	// that appear in /help but not in the tools[] array.
	UserOnlySkills []skill.Skill
	// ActiveSkillSlugs is the set of skill slugs that have at least one active tool
	// binding, as determined by ToolSchemaBuilder (P-C62-1). When non-empty, only
	// skills present in this set are listed in "## Available Tools". Skills absent
	// from the set are excluded so the LLM cannot hallucinate calls to a slug that
	// has no callable implementation (BUG-SKILL-EMPTY).
	ActiveSkillSlugs map[string]bool
	// RequestContext supplies per-request identity (user, tenant) used to resolve
	// {{user.email}}, {{tenant.id}}, etc. placeholders in SystemPrompt. MA-09.
	RequestContext RequestContext
}

// Build assembles the full system prompt from all dynamic sections.
// Stable sections (tools, KBs, static instructions) are cached across turns
// to preserve prompt cache prefix stability. Volatile sections (memories,
// summaries, deferred tools) recompute every turn.
// Inspired by Claude Code's systemPromptSection (cached) vs
// DANGEROUS_uncachedSystemPromptSection (volatile) pattern.
func (b *PromptBuilder) Build(ctx context.Context, in PromptInput) (string, error) {
	var sections []string

	// 1. Agent Identity & Instructions (cached — stable across turns)
	if in.SystemPrompt != "" {
		sections = append(sections, ResolveSystemPromptPlaceholders(in.SystemPrompt, in.RequestContext))
	}

	// 1b. Anti-hallucination guard (P-C127-3, P-C130-1, P-C142-2).
	// Always present — guarantees safe behaviour regardless of agent prompt.
	// Not configurable from prompt_template: injected unconditionally.
	sections = append(sections, antiHallucinationGuard)

	// 1c. User interaction policy — injected at the top of every prompt so
	// the LLM sees it before the tool list. This ensures the model always
	// uses ask_user for structured input instead of asking in plain text.
	userInteractionSection, err := b.resolvePromptSection(
		ctx,
		in.AgentID,
		"prompt-section:user_interaction_policy:"+in.AgentID.String(),
		promptTemplateSlugUserInteractionPolicy,
		userInteractionPolicy,
	)
	if err != nil {
		return "", err
	}
	sections = append(sections, userInteractionSection)

	// 2. Available Tools (cached — only changes on skill config changes)
	if b.skills != nil {
		// BUG-SKILL-EMPTY: snapshot active slugs so the closure uses the value
		// from this Build() call. The active slug fingerprint is part of the cache
		// key because Available Tools and tool-referencing instructions depend on it.
		activeSlugSnapshot := in.ActiveSkillSlugs
		activeSlugCacheKey := activeSkillSlugsCacheKey(activeSlugSnapshot)
		toolsSection, err := b.getCachedOrCompute("tools:"+activeSlugCacheKey+":"+in.AgentID.String(), func() (string, error) {
			skills, err := b.skills.ListByAgentID(ctx, in.AgentID)
			if err != nil {
				return "", fmt.Errorf("prompt: list skills: %w", err)
			}
			if len(skills) > 0 {
				return formatToolsSection(skills, activeSlugSnapshot), nil
			}
			return "", nil
		})
		if err != nil {
			return "", err
		}
		if toolsSection != "" {
			sections = append(sections, toolsSection)
		}

		// 2b. Skill Instructions (cached — stable across turns).
		// P-C152-2: behavioral instructions (no tool references) are always included.
		// Tool-referencing instructions require active tool bindings to avoid hallucination;
		// they are handled by FormatSkillInstructionsSection when tool info is available.
		// Without tool info here, we safely include only behavioral (non-tool-referencing)
		// instructions so that formatting, tone, and workflow rules always reach the LLM.
		instrSection, instrErr := b.getCachedOrCompute("skill-instructions:"+activeSlugCacheKey+":"+in.AgentID.String(), func() (string, error) {
			skills, err := b.skills.ListByAgentID(ctx, in.AgentID)
			if err != nil {
				return "", fmt.Errorf("prompt: list skills for instructions: %w", err)
			}
			var sb strings.Builder
			for _, s := range skills {
				if s.Instructions == "" || s.DisableModelInvocation {
					continue
				}
				referencesTool := referencesToolByName(s.Instructions)
				if referencesTool {
					// RT-02: tool-referencing instructions are safe only when the caller
					// proved this skill has at least one active callable tool. When the
					// caller has no active-tool snapshot, keep the conservative legacy
					// behavior and omit them.
					if len(activeSlugSnapshot) == 0 || !activeSlugSnapshot[s.Slug] {
						continue
					}
				}
				sb.WriteString(s.Instructions)
				sb.WriteString("\n")
			}
			return sb.String(), nil
		})
		if instrErr != nil {
			return "", instrErr
		}
		if instrSection != "" {
			sections = append(sections, "## Skill Instructions\n\n"+instrSection)
		}
	}

	// 3. Tool Usage Instructions (static — always cached)
	toolUsageSection, err := b.resolvePromptSection(
		ctx,
		in.AgentID,
		"prompt-section:tool_usage_instructions:"+in.AgentID.String(),
		promptTemplateSlugToolUsageInstructions,
		toolUsageInstructions,
	)
	if err != nil {
		return "", err
	}
	sections = append(sections, toolUsageSection)

	// 3b. Deferred tools announcement (volatile — changes per BuildWithDeferred result).
	if len(in.DeferredToolNames) > 0 {
		sections = append(sections, formatDeferredToolsSection(in.DeferredToolNames))
	}

	// 3c. User-only skills announcement (cached — stable within a run).
	// These skills have disableModelInvocation=true but the LLM should know
	// they exist so it can suggest them to the user when relevant.
	if len(in.UserOnlySkills) > 0 {
		sections = append(sections, formatUserOnlySkillsSection(in.UserOnlySkills))
	}

	// 3d. Coordinator instructions (cached — stable within a run).
	if in.CoordinatorMode {
		sections = append(sections, coordinatorInstructions)
	}

	// 4. Knowledge Base Context (cached — only changes on KB config changes)
	if b.kbs != nil {
		kbSection, err := b.getCachedOrCompute("kbs:"+in.AgentID.String(), func() (string, error) {
			kbs, err := b.kbs.ListByAgentID(ctx, in.AgentID)
			if err != nil {
				return "", fmt.Errorf("prompt: list knowledge bases: %w", err)
			}
			if len(kbs) > 0 {
				return formatKBSection(kbs), nil
			}
			return "", nil
		})
		if err != nil {
			return "", err
		}
		if kbSection != "" {
			sections = append(sections, kbSection)
		}
	}

	// 5. Memories (volatile — changes per recall)
	if in.Memories != "" {
		sections = append(sections, "## Relevant Memories\n\n"+in.Memories)
	}

	// 6. Conversation Summary (volatile — changes after compaction)
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

func (b *PromptBuilder) resolvePromptSection(
	ctx context.Context,
	agentID uuid.UUID,
	cacheKey, slug, fallback string,
) (string, error) {
	return b.getCachedOrCompute(cacheKey, func() (string, error) {
		if b.tpl == nil {
			return fallback, nil
		}
		content, found, err := b.tpl.ResolvePromptTemplate(ctx, agentID, slug)
		if err != nil {
			return "", fmt.Errorf("prompt: resolve template %s: %w", slug, err)
		}
		if found && strings.TrimSpace(content) != "" {
			return content, nil
		}
		return fallback, nil
	})
}

func activeSkillSlugsCacheKey(activeSkillSlugs map[string]bool) string {
	if len(activeSkillSlugs) == 0 {
		return "active-slugs=all"
	}
	slugs := make([]string, 0, len(activeSkillSlugs))
	for slug, enabled := range activeSkillSlugs {
		if enabled {
			slugs = append(slugs, slug)
		}
	}
	if len(slugs) == 0 {
		return "active-slugs=none"
	}
	sort.Strings(slugs)
	return "active-slugs=" + strings.Join(slugs, ",")
}

// formatToolsSection produces a markdown block listing the available skills.
// Skills with DisableModelInvocation=true are excluded (they appear in the
// User Commands section instead).
// activeSkillSlugs, when non-empty, restricts the list to skills that have at
// least one active tool binding. Skills absent from the set are omitted so the
// LLM cannot hallucinate calls to a slug with no callable implementation.
// BUG-SKILL-EMPTY: this prevents empty-tool skills from appearing here even
// though P-C62-1 already excludes them from the JSON tools[] array — the
// system prompt listing was the remaining vector for LLM hallucination.
// For skills with WhenToUse set, it injects a "When to use:" sub-bullet so the
// LLM can make more precise selection decisions. This keeps the tool description
// focused on WHAT the skill does while WhenToUse explains WHEN to invoke it.
// Inspired by Claude Code's BundledSkillDefinition.whenToUse injection pattern.
func formatToolsSection(skills []skill.Skill, activeSkillSlugs map[string]bool) string {
	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	for _, s := range skills {
		if s.DisableModelInvocation {
			continue
		}
		// BUG-SKILL-EMPTY: skip skills that have no active tool bindings.
		if len(activeSkillSlugs) > 0 && !activeSkillSlugs[s.Slug] {
			continue
		}
		fmt.Fprintf(&sb, "- **%s** (`%s`)", s.Name, s.Slug)
		if s.Description != "" {
			sb.WriteString(": " + s.Description)
		}
		sb.WriteString("\n")
		if s.WhenToUse != nil && *s.WhenToUse != "" {
			fmt.Fprintf(&sb, "  - *When to use:* %s\n", *s.WhenToUse)
		}
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

// formatUserOnlySkillsSection produces a markdown block listing user-only slash commands.
// The LLM cannot invoke these but should suggest them when relevant.
// Shows argument_hint inline (e.g. "/debug-agent <agent_id>") and when_to_use as a sub-bullet
// so the LLM can suggest proper usage without trial-and-error.
// Inspired by Claude Code's userInvocable skills in /help with argumentHint and whenToUse.
func formatUserOnlySkillsSection(skills []skill.Skill) string {
	var sb strings.Builder
	sb.WriteString("## User Commands\n\n")
	sb.WriteString("The following commands are available to the user (not callable by you directly). ")
	sb.WriteString("Suggest them when relevant to the user's needs.\n\n")
	for _, s := range skills {
		// Format: "- **/slug** `<arg_hint>`" or "- **/slug**" if no hint
		if s.ArgumentHint != nil && *s.ArgumentHint != "" {
			fmt.Fprintf(&sb, "- **/%s** `%s`", s.Slug, *s.ArgumentHint)
		} else {
			fmt.Fprintf(&sb, "- **/%s**", s.Slug)
		}
		if s.Description != "" {
			sb.WriteString(": " + s.Description)
		}
		sb.WriteString("\n")
		if s.WhenToUse != nil && *s.WhenToUse != "" {
			fmt.Fprintf(&sb, "  - *When to suggest:* %s\n", *s.WhenToUse)
		}
	}
	return sb.String()
}

// formatDeferredToolsSection produces a system-reminder block announcing deferred tools.
// The LLM sees these names but must call tool_search to load their full schemas.
// Inspired by Claude Code's deferred tool announcement in ToolSearchTool/prompt.ts.
func formatDeferredToolsSection(names []string) string {
	var sb strings.Builder
	sb.WriteString("## Deferred Tools\n\n")
	sb.WriteString("The following tools are available but their schemas are not loaded yet. ")
	sb.WriteString("Use the `tool_search` tool to load a tool's full schema before calling it.\n\n")
	for _, name := range names {
		fmt.Fprintf(&sb, "- `%s`\n", name)
	}
	return sb.String()
}

// coordinatorInstructions is injected when the agent tool is available for sub-agent spawning.
// Inspired by Claude Code's coordinatorMode.ts — detailed guidance on delegation,
// synthesis, verification, and prompt quality.
const coordinatorInstructions = `## Coordinator Mode

You are a **coordinator**. Your job is to:
- Help the user achieve their goal
- Direct sub-agents to research, implement, and verify
- Synthesize results and communicate with the user
- Answer questions directly when possible — don't delegate work you can handle without tools

Sub-agent results are internal signals, not conversation partners — never thank or acknowledge them. Summarize new information for the user as it arrives.

### Your Tools

- **agent** — Spawn a new sub-agent
- To launch sub-agents in parallel, make multiple tool calls in a single message

When calling the agent tool:
- Do not use one sub-agent to check on another
- Do not use sub-agents for trivial operations — give them higher-level tasks
- After launching agents, briefly tell the user what you launched and end your response
- Never fabricate or predict agent results — results arrive as separate messages

### Task Workflow

Most tasks follow these phases:

| Phase | Who | Purpose |
|-------|-----|---------|
| Research | Sub-agents (parallel) | Investigate codebase, find files, understand problem |
| Synthesis | **You** (coordinator) | Read findings, craft implementation specs |
| Implementation | Sub-agents | Make targeted changes per spec |
| Verification | Sub-agents | Test changes work |

### Concurrency

**Parallelism is your superpower.** Launch independent sub-agents concurrently whenever possible. When doing research, cover multiple angles.

- **Read-only tasks** (research) — run in parallel freely
- **Write-heavy tasks** (implementation) — one at a time per set of files
- **Verification** can sometimes run alongside implementation on different file areas

### Writing Sub-Agent Prompts

**Sub-agents cannot see your conversation.** Every prompt must be self-contained with everything the sub-agent needs.

#### Always synthesize — your most important job

When sub-agents report research findings, **you must understand them before directing follow-up work**. Read the findings. Identify the approach. Then write a prompt that proves you understood by including specific file paths, line numbers, and exactly what to change.

Never write "based on your findings" — these phrases delegate understanding instead of doing it yourself.

Bad: "Based on the research, fix the auth bug"
Good: "Fix the null pointer in src/auth/validate.go:42. The user field is undefined when sessions expire but the token remains cached. Add a nil check before user.ID access — if nil, return 401. Run tests and report results."

#### Add a purpose statement

Include a brief purpose so sub-agents can calibrate depth:
- "This research will inform implementation — report file paths, line numbers, and type signatures."
- "This is a quick check — just verify the happy path."

#### Prompt tips

- Include file paths, line numbers, error messages — sub-agents start fresh
- State what "done" looks like
- For implementation: "Run relevant tests, then report results"
- For research: "Report findings — do not modify files"
- For verification: "Prove the code works, don't just confirm it exists"
- For verification: "Try edge cases and error paths — don't just re-run happy paths"

### What Real Verification Looks Like

Verification means **proving the code works**, not confirming it exists:
- Run tests **with the feature enabled** — not just "tests pass"
- Run builds and **investigate errors** — don't dismiss as "unrelated"
- Be skeptical — if something looks off, dig in
- **Test independently** — prove the change works, don't rubber-stamp

### Handling Failures

When a sub-agent reports failure (tests failed, build errors, file not found):
- Spawn a new sub-agent with the error context and a corrected approach
- If a correction attempt fails, try a different approach or report to the user

### When NOT to Delegate

- Simple tasks you can handle directly with a single tool call
- Tasks with strong sequential dependencies (sub-agent B needs sub-agent A's result)
- Trivial questions that don't require tool usage`

// antiHallucinationGuard is injected into every system prompt immediately after
// the agent's persona instructions. It establishes hard behavioural guardrails
// that prevent the LLM from fabricating tool calls, referencing non-existent
// tools, or inventing data when it lacks a tool to retrieve real information.
// P-C127-3, P-C127-4, P-C130-1, P-C142-2.
// This constant is not configurable from the prompt_template table — it is
// always present and cannot be overridden by skill instructions.
const antiHallucinationGuard = `## Tool Usage Rules (enforced by system)
- You MUST NOT claim to have called a tool unless it appears in your tool_use block in this turn.
- You MUST NOT reference tools by name unless they are listed in your available tools for this turn.
- If a user asks you to use a tool you do not have access to, respond clearly: "I don't have access to that capability."
- If you don't know something and have no tool to look it up, say so honestly — do not fabricate data, statistics, or API results.
- If a tool call fails, report the failure to the user — do not invent a successful result.`

// userInteractionPolicy is a high-priority section injected near the top of the
// prompt so the LLM sees it BEFORE the tool list. It establishes the hard rule
// that all data collection must happen via the ask_user tool, not plain text.
const userInteractionPolicy = `## CRITICAL — User Input Policy

When you need ANY information from the user, you MUST call the **ask_user** tool. NEVER ask for information in plain text.

**WRONG:**
> "Please tell me: 1) the skill name, 2) the description, 3) the category"

**CORRECT:**
> Call ask_user with message and questions array.

Rules for building questions:
- When a field has known valid values (categories, statuses, types, environments), use type "select" with options — NEVER let the user type a free-text value for enum fields.
- For each select option include a description so the user understands the choice.
- Group all related fields in a single ask_user call (1-6 questions).
- Use "text" only for genuinely free-form input (names, descriptions, custom values).
- Use "confirm" for yes/no decisions.
- Make fields required unless truly optional.
- If an operation needs confirmation, include the confirm question in the SAME ask_user call that collects the other required fields.
- After the user submits a form that already included a confirm question, DO NOT open another ask_user form just to confirm the same action again.
- Treat an accepted confirm field in the submitted form as the final authorization to proceed with the write operation.

Platform enum values you MUST use (do not invent new values):
- **Skill categories:** rag, data, integration, compute, platform, system, productivity, storage, analysis, diagnostic, wizard, orchestration, memory
- **Agent states:** DRAFT, PUBLISHED, ARCHIVED
- **Tool types:** HTTP, SQL, DOCUMENT_SEARCH, CUSTOM
- **Knowledge base states:** ACTIVE, PAUSED`

// toolUsageInstructions is the static section injected into every agentic prompt.
const toolUsageInstructions = `## Tool Usage Instructions

- Use available tools to answer questions that require data retrieval or actions.
- Use document_search when the user asks about topics covered by the knowledge bases.
- NEVER execute operations that modify data without confirming with the user first via ask_user with a confirm question.
- When you receive tool results, synthesize them into a clear, concise answer.
- If a tool call fails, explain the error and suggest an alternative approach.
- Do not fabricate data — if you do not have the information, say so.
- Cite document sources when answering from knowledge base results.
- ALWAYS call ask_user to collect information — never ask via plain text.
- **After ask_user returns**, the result is a JSON object containing the user's answers (e.g. {"city":"Tokyo","units":"celsius"}). Extract those values and IMMEDIATELY proceed to call the intended tool or perform the pending action — do NOT call ask_user again for the same information.

**Memory recall rule (BUG-MEM7 fix):** When the user asks about something previously stored, told you, or saved in memory, ALWAYS call agenthub_search_memories (or agenthub_list_memories) FIRST to look it up — do NOT call ask_user to request information the user already provided. Only call ask_user if the search returns nothing useful AND you genuinely need clarification.`

const (
	// Prompt template slugs for agentic global sections. When present in the
	// prompt_template table, they override the built-in fallback constants.
	promptTemplateSlugUserInteractionPolicy = "agentic-user-interaction-policy"
	promptTemplateSlugToolUsageInstructions = "agentic-tool-usage-instructions"
)

// SkillWithTools pairs a skill with its active tool implementations.
// Used by FormatSkillInstructionsSection to filter orphaned skill instructions.
type SkillWithTools struct {
	Skill       skill.Skill
	ActiveTools []tool.Tool
}

// FormatSkillInstructionsSection produces the skill instructions block for the system
// prompt. Applies two P-C filters:
//   - P-C152-2 / P-C159-1 / P-C168-1: skills with no active tools whose instructions
//     reference a tool by name are omitted entirely to prevent LLM hallucination.
//   - Behavioral instructions (no tool reference) are included even for orphaned skills.
func FormatSkillInstructionsSection(skills []SkillWithTools) string {
	var sb strings.Builder
	for _, sw := range skills {
		if sw.Skill.Instructions == "" {
			continue
		}
		// Keep purely behavioral instructions even when a skill has no active
		// implementation, but suppress instructions that mention concrete tool use.
		if len(sw.ActiveTools) == 0 && referencesToolByName(sw.Skill.Instructions) {
			continue
		}
		sb.WriteString(sw.Skill.Instructions)
		sb.WriteString("\n")
	}
	return sb.String()
}

// referencesToolByName returns true when instructions appear to reference a specific
// tool by name. Used to decide whether to omit orphaned skill instructions.
// Heuristic: looks for common invocation phrases ("call the X", "use the X", etc.).
func referencesToolByName(instructions string) bool {
	lower := strings.ToLower(instructions)
	// BUG-SKILL-EMPTY: added "this skill", "use this" to catch instructions like
	// "use this skill to perform X" that cause LLM to hallucinate tool calls.
	for _, p := range []string{"call the ", "call ", "use the ", "use this", "invoke ", "using tool", " tool", "this skill"} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
