package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ToolsBySkillLister returns the tools bound to a skill.
type ToolsBySkillLister interface {
	ListBySkill(ctx context.Context, skillID uuid.UUID) ([]tool.SkillTool, []tool.Tool, error)
}

// LLMTool represents a tool definition sent to the LLM in the tools[] array.
// The structure matches the Anthropic / OpenAI function-calling schema.
type LLMTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	// ReadOnly indicates whether this tool only reads data and has no side effects.
	// Read-only tools can be executed in parallel; write tools run serially.
	// Inspired by Claude Code's isConcurrencySafe / isReadOnly per-tool flags.
	ReadOnly bool `json:"readOnly,omitempty"`
	// MaxResultChars overrides the global MaxToolResultChars for this tool.
	// Zero means use the global default from RunConfig.
	// Inspired by Claude Code's per-tool maxResultSizeChars.
	MaxResultChars int `json:"-"` // not sent to LLM
	// Builtin marks tools that are built into the platform (document_search,
	// memory_store, agent) as opposed to user-configured skills/MCP tools.
	// Used for stable sort ordering: builtins form a contiguous prefix,
	// skill/MCP tools form a suffix — both sorted alphabetically within
	// their partition. This prevents MCP tools from interleaving with builtins
	// and invalidating the LLM's prompt cache.
	// Inspired by Claude Code's assembleToolPool in tools.ts.
	Builtin bool `json:"-"` // not sent to LLM
	// ShouldDefer indicates this tool's full schema should not be included in
	// the initial prompt. The LLM loads it on demand via tool_search builtin.
	// Reduces prompt size for agents with many tools.
	// Inspired by Claude Code's shouldDefer flag (Tool.ts).
	ShouldDefer bool `json:"-"` // not sent to LLM
	// IsDestructive flags tools that perform irreversible operations (delete,
	// drop, overwrite). Used to auto-require confirmation even in permissive modes.
	// Inspired by Claude Code's isDestructive per-tool flag (Tool.ts).
	IsDestructive bool `json:"-"` // not sent to LLM
	// SearchHint is a short keyword phrase for tool_search matching.
	// Inspired by Claude Code's searchHint per-tool string (Tool.ts).
	SearchHint string `json:"-"` // not sent to LLM
	// AllowedTools restricts which tools this skill can use when invoked
	// in coordinator/worker mode. Empty means all tools are allowed.
	// Propagated from skill.AllowedTools.
	// Inspired by Claude Code's BundledSkillDefinition.allowedTools.
	AllowedTools []string `json:"-"` // not sent to LLM
	// AlwaysLoad prevents this tool from being deferred even when ToolSearch is active.
	// Use for tools the LLM must see on turn 1 without a tool_search round-trip.
	// Builtins are implicitly always-load; this flag is for MCP/skill tools that
	// need the same treatment.
	// Inspired by Claude Code's alwaysLoad flag (Tool.ts, MCP _meta['anthropic/alwaysLoad']).
	AlwaysLoad bool `json:"-"` // not sent to LLM
	// DisableModelInvocation marks skills that should only be user-invocable
	// (via slash commands), not LLM-invocable. Their descriptions are excluded
	// from the system prompt to save context tokens.
	// Inspired by Claude Code's BundledSkillDefinition.disableModelInvocation.
	DisableModelInvocation bool `json:"-"` // not sent to LLM
	// ConcurrencySafe indicates this tool can run in parallel even if not read-only.
	// When false, the tool is only parallelized if ReadOnly is true.
	// Inspired by Claude Code's isConcurrencySafe independent of isReadOnly.
	ConcurrencySafe bool `json:"-"` // not sent to LLM
	// ContextMode determines how this skill executes: "inline" (default) runs in
	// the current conversation context; "fork" runs in a sub-agent with its own context.
	// Fork mode prevents diagnostic skills from polluting the main conversation.
	// Inspired by Claude Code's BundledSkillDefinition.context ('inline' | 'fork').
	ContextMode string `json:"-"` // not sent to LLM
	// WhenToUse provides WHEN guidance separate from description (WHAT).
	// Injected into the system prompt as a sub-bullet under the tool listing.
	// Inspired by Claude Code's BundledSkillDefinition.whenToUse.
	WhenToUse string `json:"-"` // not sent to LLM; injected into system prompt
	// InterruptBehavior is "block" or "" (meaning cancel).
	// "block" tools complete before the run stops; "" tools are aborted immediately.
	// Inspired by Claude Code's Tool.ts interruptBehavior(): 'cancel' | 'block'.
	InterruptBehavior string `json:"-"` // not sent to LLM
	// IsSearchOrRead marks tools whose results auto-collapse in the UI.
	// Inspired by Claude Code's Tool.ts isSearchOrReadCommand().
	IsSearchOrRead bool `json:"-"` // not sent to LLM
	// TokenBudget is an optional per-skill token budget (in output tokens) set
	// on the agent_skill binding. When non-nil, the runner tracks cumulative
	// output tokens for this tool's skill and refuses further calls once the
	// budget is exhausted.
	TokenBudget *int `json:"-"` // not sent to LLM
}

// SkillTokenBudgetProvider returns per-agent-skill token budget overrides.
// The returned map keys are skill IDs; nil value means no budget limit.
// Used to populate LLMTool.TokenBudget for enforcement in the runner.
type SkillTokenBudgetProvider interface {
	GetSkillTokenBudgets(ctx context.Context, agentID uuid.UUID) (map[uuid.UUID]*int, error)
}

// CoreToolProvider loads platform-managed tools from the global ah_core schema.
// Implementations are non-fatal: they must return (nil, nil) when the schema
// does not exist (e.g. fresh deployments, tests without the schema).
type CoreToolProvider interface {
	LoadCoreTools(ctx context.Context) ([]LLMTool, error)
}

// ToolSchemaBuilder converts the skills/tools linked to an agent into LLMTool
// definitions compatible with the LLM function-calling API.
type ToolSchemaBuilder struct {
	skills                 SkillLister
	tools                  ToolsBySkillLister
	kbs                    KBLister
	mcpBridge              *MCPToolBridge
	coreTools              CoreToolProvider
	tokenBudgets           SkillTokenBudgetProvider
	currentDepth           int
	maxDepth               int
	adminScope             bool        // P-C298-1: gate agenthub_manage to admin callers only
	enableManagement       bool        // P-C184-2: gate agenthub_manage to agents with enable_management=true
	disableAskUser         bool        // per-agent opt-out — removes ask_user from the builtin set
	disableAgentDelegation bool        // per-agent opt-out — removes the agent sub-agent spawner
	skillIDsSnapshot       []uuid.UUID // P-C115-1: when set, use these IDs instead of querying by agentID
	lastUserOnlySkills     []skill.Skill
	lastWarnings           []string
}

// NewToolSchemaBuilder creates a ToolSchemaBuilder.
func NewToolSchemaBuilder(skills SkillLister, tools ToolsBySkillLister, kbs KBLister) *ToolSchemaBuilder {
	return &ToolSchemaBuilder{skills: skills, tools: tools, kbs: kbs, maxDepth: 3}
}

// Clone returns a shallow copy so per-run state (depth, MCP bridge, cached skill
// subsets) can be mutated without racing other concurrent runs.
// coreTools is intentionally shared — it is stateless and safe for concurrent reads.
func (b *ToolSchemaBuilder) Clone() *ToolSchemaBuilder {
	if b == nil {
		return nil
	}
	cloned := *b
	if len(b.lastUserOnlySkills) > 0 {
		cloned.lastUserOnlySkills = append([]skill.Skill(nil), b.lastUserOnlySkills...)
	}
	cloned.lastWarnings = nil
	return &cloned
}

// WithMCPBridge attaches an MCP tool bridge so that MCP tools are included
// alongside internal skills in the tool definitions sent to the LLM.
func (b *ToolSchemaBuilder) WithMCPBridge(bridge *MCPToolBridge) *ToolSchemaBuilder {
	b.mcpBridge = bridge
	return b
}

// WithCoreToolProvider attaches a provider for platform-managed tools from the
// ah_core schema. When set, core tools are appended to the tool list after
// tenant skills and MCP tools.
func (b *ToolSchemaBuilder) WithCoreToolProvider(p CoreToolProvider) *ToolSchemaBuilder {
	b.coreTools = p
	return b
}

// WithTokenBudgetProvider attaches a provider for per-agent-skill token budgets.
// When set, Build() populates LLMTool.TokenBudget for enforcement by the runner.
func (b *ToolSchemaBuilder) WithTokenBudgetProvider(p SkillTokenBudgetProvider) *ToolSchemaBuilder {
	b.tokenBudgets = p
	return b
}

// WithDepthLimits sets the current and max depth for sub-agent tool availability.
func (b *ToolSchemaBuilder) WithDepthLimits(currentDepth, maxDepth int) *ToolSchemaBuilder {
	b.currentDepth = currentDepth
	b.maxDepth = maxDepth
	return b
}

// WithAdminScope controls whether the agenthub_manage builtin tool is included.
// P-C298-1: restrict agenthub_manage to sessions where the caller has the "admin" role.
// When false (default), the tool is omitted — preventing weaker models from calling
// it opportunistically during normal user sessions.
func (b *ToolSchemaBuilder) WithAdminScope(admin bool) *ToolSchemaBuilder {
	b.adminScope = admin
	return b
}

// WithDisableAskUser removes the ask_user builtin from the tool set when true.
// Useful for fully autonomous agents where asking the user is never desired.
func (b *ToolSchemaBuilder) WithDisableAskUser(disabled bool) *ToolSchemaBuilder {
	b.disableAskUser = disabled
	return b
}

// WithDisableAgentDelegation removes the agent builtin (sub-agent spawner) when true.
// Useful when the parent agent must handle every tool call itself rather than delegating.
func (b *ToolSchemaBuilder) WithDisableAgentDelegation(disabled bool) *ToolSchemaBuilder {
	b.disableAgentDelegation = disabled
	return b
}

// WithEnableManagement sets whether the agent has explicitly opted in to management tools.
// P-C184-2: agenthub_manage is only included when BOTH adminScope AND enableManagement are true,
// AND the run is at depth 0 (not a sub-agent). This two-layer gate prevents:
// - Non-admin callers from getting management tools even on opted-in agents
// - Sub-agents from inheriting management scope from their parent (P-C281-1)
func (b *ToolSchemaBuilder) WithEnableManagement(enabled bool) *ToolSchemaBuilder {
	b.enableManagement = enabled
	return b
}

// WithSkillIDsSnapshot sets explicit skill IDs to load instead of querying by agentID.
// P-C115-1: when set, Build() uses these IDs to ensure the tool set is fixed for
// the lifetime of the session even if the agent's bindings change mid-conversation.
func (b *ToolSchemaBuilder) WithSkillIDsSnapshot(ids []uuid.UUID) *ToolSchemaBuilder {
	b.skillIDsSnapshot = ids
	return b
}

// ToolBuildResult contains both the loaded and deferred tools from a Build call.
type ToolBuildResult struct {
	// Loaded contains tools whose full schema is sent to the LLM.
	Loaded []LLMTool
	// Deferred contains tools that require tool_search to load.
	// Only their names are announced to the LLM in a system message.
	Deferred []LLMTool
	// All contains every tool (loaded + deferred) for execution lookups.
	All []LLMTool
	// UserOnlySkills contains skills with DisableModelInvocation=true.
	// These are announced in the system prompt so the LLM can suggest them
	// to the user, but they are not included in the tools[] array.
	UserOnlySkills []skill.Skill
	// Warnings contains non-fatal issues encountered during tool loading
	// (e.g. MCP auth failures). These should be surfaced to the client.
	Warnings []string
}

// DeferredToolThreshold is the minimum number of total tools before deferred
// loading is activated. Below this count, all tools are loaded immediately.
// Inspired by Claude Code's isToolSearchEnabled threshold.
const DeferredToolThreshold = 15

// Build returns the tool definitions for the given agent.
func (b *ToolSchemaBuilder) Build(ctx context.Context, agentID uuid.UUID) ([]LLMTool, error) {
	// 1. Load explicitly linked skills.
	// P-C115-1: when a skill snapshot is present, use those IDs instead of the
	// agent's current bindings so the tool set stays fixed for the session.
	var skills []skill.Skill
	var err error
	if len(b.skillIDsSnapshot) > 0 {
		skills, err = b.skills.ListByIDs(ctx, b.skillIDsSnapshot)
	} else {
		skills, err = b.skills.ListByAgentID(ctx, agentID)
	}
	if err != nil {
		return nil, fmt.Errorf("toolschema: list skills: %w", err)
	}

	// 2. Load integration-based skills automatically (simplification)
	// We include all skills from 'INTEGRATION_HTTP' and 'INTEGRATION_DATABASE'
	// as they are managed via the Integrations tab and should be globally available.
	integrationCategories := []string{"INTEGRATION_HTTP", "INTEGRATION_DATABASE"}
	for _, cat := range integrationCategories {
		content, _, err := b.skills.List(ctx, &cat, pagination.PageRequest{Page: 0, Size: 100})
		if err == nil {
			for _, skItem := range content {
				// Avoid duplicates if already explicitly linked
				exists := false
				for _, sk := range skills {
					if sk.ID == skItem.ID {
						exists = true
						break
					}
				}
				if !exists {
					skills = append(skills, skItem)
				}
			}
		}
	}

	// Load per-skill token budgets for this agent (optional).
	var budgetMap map[uuid.UUID]*int
	if b.tokenBudgets != nil {
		budgetMap, _ = b.tokenBudgets.GetSkillTokenBudgets(ctx, agentID)
	}

	var tools []LLMTool
	var userOnlySkills []skill.Skill
	for _, sk := range skills {
		// Skills marked DisableModelInvocation are user-only (slash commands).
		// Exclude from LLM tool list to save context tokens.
		// Inspired by Claude Code's BundledSkillDefinition.disableModelInvocation.
		if sk.DisableModelInvocation {
			userOnlySkills = append(userOnlySkills, sk)
			continue
		}
		// Bug 199: skill com 2+ tools agora expõe cada tool individualmente
		// (slug do tool + schema do tool). Skill com 1 tool mantém aggregator
		// pattern (skill slug + first tool schema).
		llmTools, callable, err := b.skillToLLMTools(ctx, sk)
		if err != nil {
			return nil, err
		}
		// P-SK10: skills with no active bound tools must not appear as callable tools.
		// They contribute only to behavioral instructions (injected via prompt.go),
		// and listing them as tools causes the LLM to invoke them → "skill not found".
		if !callable {
			continue
		}
		for i := range llmTools {
			// Skill-level ShouldDefer (3rd progressive disclosure layer): if the skill
			// itself is marked should_defer, set the flag regardless of bound tool flags.
			if sk.ShouldDefer {
				llmTools[i].ShouldDefer = true
			}
			// Populate per-skill token budget from the agent_skill binding.
			if budgetMap != nil {
				if budget, ok := budgetMap[sk.ID]; ok {
					llmTools[i].TokenBudget = budget
				}
			}
		}
		tools = append(tools, llmTools...)
	}
	// Store user-only skills for prompt announcement (accessible via lastUserOnlySkills).
	b.lastUserOnlySkills = userOnlySkills

	// Builtin: document_search — added when the agent has linked knowledge bases
	// or if we decide to include all tenant KBs automatically.
	if b.kbs != nil {
		kbsResp, err := b.kbs.ListByAgentID(ctx, agentID)
		if err != nil {
			return nil, fmt.Errorf("toolschema: list knowledge bases: %w", err)
		}

		// Convert legacy knowledgebase.KnowledgeBase to the internal type if needed
		// or use the response format.
		var kbs = kbsResp

		// Auto-include tenant-wide KBs that are not explicitly linked
		allKbs, _, err := b.kbs.List(ctx, pagination.PageRequest{Page: 0, Size: 100})
		if err == nil {
			for _, kbItem := range allKbs {
				exists := false
				for _, existing := range kbs {
					if existing.ID == kbItem.ID {
						exists = true
						break
					}
				}
				if !exists {
					kbs = append(kbs, kbItem)
				}
			}
		}

		// P-C179-1: exclude PAUSED KBs — document_search on a paused KB would
		// silently return no results or an error, confusing the LLM.
		activeKBs := FilterActiveKBs(kbs)
		if len(activeKBs) > 0 {
			tools = append(tools, documentSearchTool(activeKBs))
		}
	}

	// Builtin: memory_store — always available.
	tools = append(tools, memoryStoreTool())

	// Builtin: memory_store_bulk — batch variant; always available.
	// BUG-MEM-STRESS1: models that loop on sequential memory_store calls can use
	// this to persist all facts in a single tool invocation.
	tools = append(tools, memoryStoreBulkTool())

	// Builtin: memory_recall — bug 287: explicit recall fallback when auto-recall
	// (via system prompt injection) does not surface the relevant memory. Useful
	// when user asks to retrieve a specific stored fact and embedding-based
	// similarity scores below threshold.
	tools = append(tools, memoryRecallTool())

	// Builtin: ask_user — available by default; agents can opt-out via config.disableAskUser.
	if !b.disableAskUser {
		tools = append(tools, askUserTool())
	}

	// Builtins: canvas_update, canvas_feedback, canvas_export_table — always available.
	// These tools render rich visual content in the canvas panel alongside the chat.
	tools = append(tools, canvasUpdateTool(), canvasFeedbackTool(), canvasExportTableTool())

	// Builtin: agent — available when depth < maxDepth and agent hasn't opted out.
	if b.currentDepth < b.maxDepth && !b.disableAgentDelegation {
		tools = append(tools, agentTool(b.maxDepth-b.currentDepth))
	}

	// Builtin: send_message — available to sub-agents (depth > 0) for inter-agent messaging.
	if b.currentDepth > 0 {
		tools = append(tools, sendMessageTool())
	}

	// Builtin: agenthub_manage — requires ALL three conditions (P-C184-2, P-C298-1, P-C281-1):
	// 1. The agent has enable_management=true (per-agent opt-in)
	// 2. The caller has the "admin" realm role (JWT-based)
	// 3. This is not a sub-agent run (depth == 0)
	// Sub-agents must never inherit management scope from their parent agent.
	managementScope := b.enableManagement && b.adminScope && b.currentDepth == 0
	if managementScope {
		tools = append(tools, agentHubManageTool())
	}

	// MCP tools — fetched from external MCP servers via the bridge.
	// ListTools may return both tools and a non-nil error (partial success): some servers
	// responded while others failed. Always add the tools we got; always emit warnings for
	// the failures so neither the tools nor the problem are silently dropped.
	b.lastWarnings = nil
	if b.mcpBridge != nil {
		mcpTools, mcpErr := b.mcpBridge.ListTools(ctx)
		if mcpErr != nil {
			slog.Warn("agentic: failed to load MCP tools", "error", mcpErr)
			// Parse one warning per server from the combined error message.
			b.lastWarnings = append(b.lastWarnings, summarizeMCPErrors(mcpErr.Error())...)
		}
		tools = appendNonDuplicateMCPTools(tools, mcpTools)
	}

	// Core tools — legacy platform-management tools from ah_core schema.
	// They have the same blast radius as agenthub_manage, so keep them behind
	// the same explicit admin + agent opt-in gate. Normal demo/business agents
	// should not send dozens of management tools to the LLM.
	// Non-fatal: provider returns nil when ah_core schema does not exist.
	if b.coreTools != nil && managementScope {
		coreList, coreErr := b.coreTools.LoadCoreTools(ctx)
		if coreErr != nil {
			slog.WarnContext(ctx, "agentic: failed to load core tools", "error", coreErr)
		} else {
			for _, ct := range coreList {
				// Skip if a tenant skill with the same slug was already loaded.
				duplicate := false
				for _, existing := range tools {
					if existing.Name == ct.Name {
						duplicate = true
						break
					}
				}
				if !duplicate {
					tools = append(tools, ct)
				}
			}
		}
	}

	if tools == nil {
		tools = []LLMTool{}
	}

	// Stable sort: builtins as contiguous prefix, skill/MCP tools as suffix.
	// Both partitions are sorted alphabetically within their group.
	// This prevents external tools from interleaving with builtins and
	// invalidating the LLM's prompt cache prefix on every MCP tool change.
	// Inspired by Claude Code's assembleToolPool (tools.ts).
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Builtin != tools[j].Builtin {
			return tools[i].Builtin // builtins first
		}
		return tools[i].Name < tools[j].Name
	})

	return tools, nil
}

func appendNonDuplicateMCPTools(tools []LLMTool, mcpTools []LLMTool) []LLMTool {
	if len(mcpTools) == 0 {
		return tools
	}
	existing := make(map[string]struct{}, len(tools)+len(mcpTools))
	for _, t := range tools {
		existing[t.Name] = struct{}{}
	}
	for _, mcpTool := range mcpTools {
		if _, toolName, ok := ParseMCPToolName(mcpTool.Name); ok {
			if _, duplicate := existing[toolName]; duplicate {
				continue
			}
		}
		if _, duplicate := existing[mcpTool.Name]; duplicate {
			continue
		}
		tools = append(tools, mcpTool)
		existing[mcpTool.Name] = struct{}{}
	}
	return tools
}

// BuildWithDeferred returns loaded and deferred tools separately.
// When the total tool count exceeds DeferredToolThreshold, tools marked
// ShouldDefer=true are separated and require tool_search to load.
// The tool_search builtin is automatically added when deferred tools exist.
//
// Inspired by Claude Code's isDeferredTool + ToolSearchTool pattern.
func (b *ToolSchemaBuilder) BuildWithDeferred(ctx context.Context, agentID uuid.UUID) (*ToolBuildResult, error) {
	all, err := b.Build(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Only activate deferred loading when there are enough tools.
	if len(all) < DeferredToolThreshold {
		return &ToolBuildResult{Loaded: all, All: all, UserOnlySkills: b.lastUserOnlySkills, Warnings: b.lastWarnings}, nil
	}

	var loaded, deferred []LLMTool
	for _, t := range all {
		// Builtins and always-load tools are never deferred — they must be available on turn 1.
		// Inspired by Claude Code's alwaysLoad flag (Tool.ts).
		if t.ShouldDefer && !t.Builtin && !t.AlwaysLoad {
			deferred = append(deferred, t)
		} else {
			loaded = append(loaded, t)
		}
	}

	// Add tool_search builtin when there are deferred tools.
	if len(deferred) > 0 {
		loaded = append(loaded, toolSearchTool(deferred))
	}

	return &ToolBuildResult{
		Loaded:         loaded,
		Deferred:       deferred,
		All:            all,
		UserOnlySkills: b.lastUserOnlySkills,
		Warnings:       b.lastWarnings,
	}, nil
}

// DeferredToolNames returns the names of deferred tools for system message injection.
func (r *ToolBuildResult) DeferredToolNames() []string {
	names := make([]string, len(r.Deferred))
	for i, t := range r.Deferred {
		names[i] = t.Name
	}
	return names
}

// skillToLLMTool converts a single skill (plus its first active tool's config) into an LLMTool.
// skillToLLMTool converts a skill to an LLM-callable tool definition.
// The second return value indicates whether the skill has active bound tools and
// should therefore be included in the callable tool list. Skills with no active
// tools (instructions-only skills) must not appear as callable — the LLM would
// attempt to invoke them and receive "skill not found" errors (P-SK10).
func (b *ToolSchemaBuilder) skillToLLMTool(ctx context.Context, sk skill.Skill) (LLMTool, bool, error) {
	// Prefer the description stored in the database so skill behaviour can be
	// adjusted without redeploying. Fall back to the legacy static catalog only
	// when the DB description is empty.
	description := strings.TrimSpace(sk.Description)
	if description == "" {
		description = EnrichDescription(sk.Slug, sk.Description)
	}
	if description == "" {
		description = sk.Name
	}

	// 4. Use instructions as part of the description if available.
	if sk.Instructions != "" {
		description = fmt.Sprintf("%s\n\nInstructions:\n%s", description, sk.Instructions)
	}

	inputSchema := json.RawMessage(`{"type":"object","properties":{}}`)
	readOnly := IsReadOnlyTool(sk.Slug)
	shouldDefer := false
	isDestructive := false
	searchHint := ""
	alwaysLoad := false
	concurrencySafe := false
	maxResultChars := 0
	interruptBehavior := ""
	isSearchOrRead := false

	// Load bound tools once — used for schema derivation, read-only, deferred, and destructive detection.
	hasActiveTool := false
	if b.tools != nil {
		bindings, boundTools, err := b.tools.ListBySkill(ctx, sk.ID)
		if err != nil {
			return LLMTool{}, false, fmt.Errorf("toolschema: list tools for skill %s: %w", sk.Slug, err)
		}
		for i, bt := range bindings {
			if bt.IsActive && i < len(boundTools) {
				hasActiveTool = true
				// P-C175-1: prefer explicit InputSchema; fall back to derived schema.
				if len(boundTools[i].InputSchema) > 2 { // non-nil, non-"{}"
					inputSchema = boundTools[i].InputSchema
				} else {
					derived := deriveSchemaFromToolConfig(boundTools[i])
					if len(derived) > 0 {
						inputSchema = derived
					}
				}
				// Use DB flags from bound tool.
				if boundTools[i].ReadOnly {
					readOnly = true
				}
				if boundTools[i].ShouldDefer {
					shouldDefer = true
				}
				if boundTools[i].IsDestructive {
					isDestructive = true
				}
				if boundTools[i].SearchHint != nil && *boundTools[i].SearchHint != "" {
					searchHint = *boundTools[i].SearchHint
				}
				if boundTools[i].AlwaysLoad {
					alwaysLoad = true
				}
				if boundTools[i].ConcurrencySafe != nil && *boundTools[i].ConcurrencySafe {
					concurrencySafe = true
				}
				if boundTools[i].MaxResultChars != nil && *boundTools[i].MaxResultChars > 0 {
					maxResultChars = *boundTools[i].MaxResultChars
				}
				if boundTools[i].InterruptBehavior != nil && *boundTools[i].InterruptBehavior != "" {
					interruptBehavior = *boundTools[i].InterruptBehavior
				}
				if boundTools[i].IsSearchOrRead {
					isSearchOrRead = true
				}
				// P-C175-2: no break — iterate all active tools so every tool's flags
				// are aggregated and only the first explicit InputSchema wins.
			}
		}
	}

	// P-C62-1: if a tool repository is wired but no active binding was found, this
	// skill has no executable implementation. Exclude it from the LLM schema entirely
	// so the model never announces a tool it cannot call.
	if b.tools != nil && !hasActiveTool {
		return LLMTool{}, false, nil
	}

	if len(inputSchema) == 0 {
		inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
	}

	return LLMTool{
		Name:                   sk.Slug,
		Description:            description,
		InputSchema:            inputSchema,
		ReadOnly:               readOnly,
		MaxResultChars:         maxResultChars,
		ShouldDefer:            shouldDefer,
		IsDestructive:          isDestructive,
		SearchHint:             searchHint,
		AllowedTools:           sk.AllowedTools,
		AlwaysLoad:             alwaysLoad,
		DisableModelInvocation: sk.DisableModelInvocation,
		ConcurrencySafe:        concurrencySafe,
		InterruptBehavior:      interruptBehavior,
		IsSearchOrRead:         isSearchOrRead,
	}, hasActiveTool, nil
}

// skillToLLMTools converts a skill to one or more LLM-callable tool definitions.
//
// Bug 199 fix: when a skill has 2+ active bound tools, this returns one LLMTool
// per tool (slug = tool.slug, schema = tool.inputSchema). When the skill has 1
// tool, it returns the legacy aggregator pattern (slug = skill.slug). Skills
// with no active tools return (nil, false, nil).
//
// The skill-runtime resolver was extended to fall back to tool.slug lookup
// when skill.slug doesn't match (see resolver.go ResolveSkill bug 199 path),
// so individual tool exposure routes correctly to the executor.
func (b *ToolSchemaBuilder) skillToLLMTools(ctx context.Context, sk skill.Skill) ([]LLMTool, bool, error) {
	if b.tools == nil {
		t, callable, err := b.skillToLLMTool(ctx, sk)
		if err != nil || !callable {
			return nil, callable, err
		}
		return []LLMTool{t}, true, nil
	}
	bindings, boundTools, err := b.tools.ListBySkill(ctx, sk.ID)
	if err != nil {
		return nil, false, fmt.Errorf("toolschema: list tools for skill %s: %w", sk.Slug, err)
	}
	// Filter active bindings.
	var activeIdx []int
	for i, bt := range bindings {
		if bt.IsActive && i < len(boundTools) {
			activeIdx = append(activeIdx, i)
		}
	}
	// Single-tool skill or no tools: keep legacy aggregator path (skill slug as
	// LLM tool name) — preserves backwards compatibility for existing agents.
	if len(activeIdx) <= 1 {
		t, callable, err := b.skillToLLMTool(ctx, sk)
		if err != nil || !callable {
			return nil, callable, err
		}
		return []LLMTool{t}, true, nil
	}
	// 2+ tools: expose each individually. Use tool.slug + tool.inputSchema.
	skillDesc := strings.TrimSpace(sk.Description)
	if skillDesc == "" {
		skillDesc = sk.Name
	}
	if sk.Instructions != "" {
		skillDesc = fmt.Sprintf("%s\n\nSkill instructions:\n%s", skillDesc, sk.Instructions)
	}
	out := make([]LLMTool, 0, len(activeIdx))
	for _, i := range activeIdx {
		bt := boundTools[i]
		toolDesc := strings.TrimSpace(bt.Description)
		desc := skillDesc
		if toolDesc != "" {
			desc = fmt.Sprintf("%s\n\nTool: %s", desc, toolDesc)
		}
		schema := bt.InputSchema
		if len(schema) <= 2 {
			schema = deriveSchemaFromToolConfig(bt)
		}
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		readOnly := IsReadOnlyTool(bt.Slug) || bt.ReadOnly
		concurrencySafe := bt.ConcurrencySafe != nil && *bt.ConcurrencySafe
		searchHint := ""
		if bt.SearchHint != nil {
			searchHint = *bt.SearchHint
		}
		maxResultChars := 0
		if bt.MaxResultChars != nil {
			maxResultChars = *bt.MaxResultChars
		}
		interruptBehavior := ""
		if bt.InterruptBehavior != nil {
			interruptBehavior = *bt.InterruptBehavior
		}
		out = append(out, LLMTool{
			Name:                   bt.Slug,
			Description:            desc,
			InputSchema:            schema,
			ReadOnly:               readOnly,
			MaxResultChars:         maxResultChars,
			ShouldDefer:            bt.ShouldDefer,
			IsDestructive:          bt.IsDestructive,
			SearchHint:             searchHint,
			AllowedTools:           sk.AllowedTools,
			AlwaysLoad:             bt.AlwaysLoad,
			DisableModelInvocation: sk.DisableModelInvocation,
			ConcurrencySafe:        concurrencySafe,
			InterruptBehavior:      interruptBehavior,
			IsSearchOrRead:         bt.IsSearchOrRead,
		})
	}
	return out, true, nil
}

// normaliseSchema ensures the raw bytes are a valid JSON Schema object, or returns nil.
func normaliseSchema(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	// Must have "type" to be a valid JSON Schema.
	if _, ok := obj["type"]; !ok {
		return nil
	}
	return json.RawMessage(raw)
}

// templateVarRe matches both {variable} and {{input.variable}} placeholders in URL
// and body templates. Group 1 captures the variable name in both cases.
// The {{input.key}} form is matched first to avoid partial matches against {input.key}.
var templateVarRe = regexp.MustCompile(`\{\{input\.(\w+)\}\}|\{(\w+)\}`)

// deriveSchemaFromToolConfig attempts to extract a usable input schema from a
// tool's config JSON. This covers cases where the skill has no explicit
// inputSchema but the underlying tool config defines parameter shapes.
//
// Priority:
//  1. Explicit "inputSchema" field in config — used as-is.
//  2. For HTTP tools: template variables extracted from "url" and "body_template"
//     fields (e.g. {city} in the URL becomes a required string parameter so the
//     LLM knows what inputs to provide).
func deriveSchemaFromToolConfig(t tool.Tool) json.RawMessage {
	if len(t.Config) == 0 {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(t.Config, &cfg); err != nil {
		return nil
	}
	// If the tool config already has an "inputSchema" field, use it (after validation).
	if raw, ok := cfg["inputSchema"]; ok {
		if data, err := json.Marshal(raw); err == nil {
			if normalised := normaliseSchema(data); normalised != nil {
				return normalised
			}
		}
	}

	// BUG-TOOL-PARAM-SCHEMA: For HTTP tools with a "parameters" array in config,
	// convert that array to a JSON Schema. This is the primary mechanism for
	// exposing required/optional parameters to the LLM when the tool was created
	// via POST /api/tools with a "parameters" array in config (without inputSchema).
	if params, ok := cfg["parameters"]; ok {
		if paramList, ok := params.([]any); ok && len(paramList) > 0 {
			props := make(map[string]any, len(paramList))
			var required []string
			for _, p := range paramList {
				pm, ok := p.(map[string]any)
				if !ok {
					continue
				}
				name, _ := pm["name"].(string)
				if name == "" {
					continue
				}
				prop := map[string]any{}
				if t, ok := pm["type"].(string); ok && t != "" {
					prop["type"] = t
				} else {
					prop["type"] = "string"
				}
				if desc, ok := pm["description"].(string); ok && desc != "" {
					prop["description"] = desc
				}
				if def, ok := pm["default"]; ok {
					prop["default"] = def
				}
				props[name] = prop
				if req, _ := pm["required"].(bool); req {
					required = append(required, name)
				}
			}
			if len(props) > 0 {
				schema := map[string]any{
					"type":       "object",
					"properties": props,
				}
				if len(required) > 0 {
					schema["required"] = required
				}
				if data, err := json.Marshal(schema); err == nil {
					return data
				}
			}
		}
	}

	// For HTTP tools: auto-derive schema from {variable} placeholders in url/body_template.
	// This ensures the LLM knows what parameters to pass even when inputSchema is absent.
	seen := map[string]bool{}
	var vars []string
	for _, field := range []string{"url", "urlTemplate", "body_template"} {
		if s, ok := cfg[field].(string); ok {
			for _, m := range templateVarRe.FindAllStringSubmatch(s, -1) {
				// Group 1: {{input.key}} form; Group 2: {key} form.
				name := m[1]
				if name == "" {
					name = m[2]
				}
				if name != "" && !seen[name] {
					seen[name] = true
					vars = append(vars, name)
				}
			}
		}
	}
	if len(vars) == 0 {
		return nil
	}
	props := make(map[string]any, len(vars))
	for _, v := range vars {
		props[v] = map[string]any{
			"type":        "string",
			"description": v,
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
		"required":   vars,
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	return data
}

// FilterActiveKBs returns only knowledge bases with status ACTIVE.
// P-C179-1: PAUSED KBs are excluded so document_search is only offered when
// there is at least one operational knowledge base.
func FilterActiveKBs(kbs []knowledgebase.KnowledgeBase) []knowledgebase.KnowledgeBase {
	var active []knowledgebase.KnowledgeBase
	for _, kb := range kbs {
		if kb.Status == knowledgebase.StatusActive {
			active = append(active, kb)
		}
	}
	return active
}

// documentSearchTool returns the builtin document_search tool definition.
func documentSearchTool(kbs []knowledgebase.KnowledgeBase) LLMTool {
	var names []string
	for _, kb := range kbs {
		names = append(names, kb.Name)
	}
	desc := fmt.Sprintf(
		"Search documents in the knowledge base(s): %s. "+
			"Use this when the user asks about information contained in the knowledge bases. "+
			"The query parameter should be a descriptive sentence, not keywords.",
		joinNames(names),
	)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Semantic search query"},
			"knowledge_base_id": {"type": "string", "description": "Optional UUID of a specific knowledge base to search"},
			"metadataFilter": {
				"type": "object",
				"description": "Optional V1 document metadata filter. Use exactly one expression: a predicate {field,op,value}, {all:[...]}, {any:[...]}, or {not:{...}}. Field accepts one or two metadata levels separated by a dot, for example customer.region. Operators: eq, neq, in, notIn, exists, notExists, gt, gte, lt, lte, like, ilike, containsAny, containsAll. The encoded filter is limited to 16 KiB; the server also enforces depth 8, 64 predicates, 32 expressions per group, and 64 list values.",
				"$ref": "#/$defs/metadataFilter"
			},
			"limit": {"type": "integer", "description": "Maximum number of results to return (default 5)"}
		},
		"required": ["query"],
		"$defs": {
			"metadataFilter": {"type": "object", "oneOf": [
				{"$ref": "#/$defs/metadataPredicate"},
				{"$ref": "#/$defs/metadataAllGroup"},
				{"$ref": "#/$defs/metadataAnyGroup"},
				{"$ref": "#/$defs/metadataNotGroup"}
			]},
			"metadataAllGroup": {"type": "object", "properties": {"all": {"type": "array", "minItems": 1, "maxItems": 32, "items": {"$ref": "#/$defs/metadataFilter"}}}, "required": ["all"], "additionalProperties": false},
			"metadataAnyGroup": {"type": "object", "properties": {"any": {"type": "array", "minItems": 1, "maxItems": 32, "items": {"$ref": "#/$defs/metadataFilter"}}}, "required": ["any"], "additionalProperties": false},
			"metadataNotGroup": {"type": "object", "properties": {"not": {"$ref": "#/$defs/metadataFilter"}}, "required": ["not"], "additionalProperties": false},
			"metadataPredicate": {"oneOf": [
				{"type": "object", "properties": {"field": {"$ref": "#/$defs/metadataField"}, "op": {"enum": ["eq", "neq"]}, "value": {"$ref": "#/$defs/metadataEqValue"}}, "required": ["field", "op", "value"], "additionalProperties": false},
				{"type": "object", "properties": {"field": {"$ref": "#/$defs/metadataField"}, "op": {"enum": ["in", "notIn"]}, "value": {"type": "array", "maxItems": 64, "items": {"$ref": "#/$defs/metadataScalar"}}}, "required": ["field", "op", "value"], "additionalProperties": false},
				{"type": "object", "properties": {"field": {"$ref": "#/$defs/metadataField"}, "op": {"enum": ["exists", "notExists"]}}, "required": ["field", "op"], "additionalProperties": false},
				{"type": "object", "properties": {"field": {"$ref": "#/$defs/metadataField"}, "op": {"enum": ["gt", "gte", "lt", "lte"]}, "value": {"type": "number"}}, "required": ["field", "op", "value"], "additionalProperties": false},
				{"type": "object", "properties": {"field": {"$ref": "#/$defs/metadataField"}, "op": {"enum": ["like", "ilike"]}, "value": {"type": "string"}}, "required": ["field", "op", "value"], "additionalProperties": false},
				{"type": "object", "properties": {"field": {"$ref": "#/$defs/metadataField"}, "op": {"enum": ["containsAny", "containsAll"]}, "value": {"type": "array", "maxItems": 64, "items": {"type": "string"}}}, "required": ["field", "op", "value"], "additionalProperties": false}
			]},
			"metadataField": {"type": "string", "minLength": 1, "description": "One or two metadata keys separated by a dot, for example customer.region"},
			"metadataScalar": {"type": ["string", "number", "boolean"]},
			"metadataEqValue": {"oneOf": [{"$ref": "#/$defs/metadataScalar"}, {"$ref": "#/$defs/metadataStringArray"}, {"$ref": "#/$defs/metadataSecondLevelObject"}]},
			"metadataSecondLevelObject": {"type": "object", "minProperties": 1, "additionalProperties": {"oneOf": [{"$ref": "#/$defs/metadataScalar"}, {"$ref": "#/$defs/metadataStringArray"}]}},
			"metadataStringArray": {"type": "array", "maxItems": 64, "items": {"type": "string"}}
		}
	}`)
	return LLMTool{
		Name:        "document_search",
		Description: desc,
		InputSchema: schema,
		ReadOnly:    true,
		Builtin:     true,
	}
}

// memoryStoreTool returns the builtin memory_store tool definition.
// P-C339-1 (ACT-F3-17): description includes TTL notice and hallucination disclaimer
// so the LLM does not claim to remember things it has not explicitly stored.
func memoryStoreTool() LLMTool {
	return LLMTool{
		Name:    "memory_store",
		Builtin: true,
		Description: `Store a piece of information for long-term recall across sessions.
Use this to remember important facts, preferences, or decisions the user shares.

IMPORTANT:
- Only store information the user has EXPLICITLY told you — never infer or fabricate facts.
- Stored memories are best-effort; they may expire or be evicted over time.
- Do NOT claim to "remember" something unless you actually called this tool in a prior turn.
- Confirm to the user when you have stored something (e.g. "I've saved that preference.").`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {
					"type": "string",
					"description": "The exact information to remember, as stated by the user"
				},
				"category": {
					"type": "string",
					"description": "Category for the memory (e.g. preference, fact, decision)"
				}
			},
			"required": ["content"]
		}`),
	}
}

// memoryStoreBulkTool returns the builtin memory_store_bulk tool definition.
// BUG-MEM-STRESS1: single-call alternative for bulk memorisation; avoids models
// getting stuck in sequential memory_store loops.
func memoryStoreBulkTool() LLMTool {
	return LLMTool{
		Name:    "memory_store_bulk",
		Builtin: true,
		Description: `Store multiple facts at once for long-term recall across sessions.
Use this when the user provides a list of facts to remember — it is more reliable than
calling memory_store repeatedly for each item.

IMPORTANT:
- Only store information the user has EXPLICITLY told you.
- Each fact must have distinct content; duplicates are silently skipped.
- Returns a summary: how many were stored, how many already existed.`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"facts": {
					"type": "array",
					"description": "List of facts to store",
					"items": {
						"type": "object",
						"properties": {
							"content": {
								"type": "string",
								"description": "The exact information to remember"
							},
							"category": {
								"type": "string",
								"description": "Category for the memory (e.g. preference, fact, decision)"
							}
						},
						"required": ["content"]
					}
				}
			},
			"required": ["facts"]
		}`),
	}
}

// memoryRecallTool returns the builtin memory_recall tool definition.
// Bug 287: explicit recall when auto-recall (via system prompt injection) doesn't
// surface a needed fact. The LLM can use this to query stored memories on demand
// — useful for direct user questions like "what did I tell you about X?".
func memoryRecallTool() LLMTool {
	return LLMTool{
		Name:     "memory_recall",
		Builtin:  true,
		ReadOnly: true,
		Description: `Search the agent's long-term memory for facts previously stored via memory_store.
Use this when the user asks about something they may have told you in a previous session,
or when you need to confirm a stored preference/fact.

Returns the most relevant memories formatted as a markdown section. Empty result means
nothing matching was found.

IMPORTANT:
- Memories are scoped to the agent — only what was explicitly stored will be returned.
- Do NOT fabricate facts. If recall returns empty, tell the user nothing is stored.`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "Search query (natural language) describing what to recall"
				}
			},
			"required": ["query"]
		}`),
	}
}

// askUserTool returns the builtin ask_user tool definition.
// This tool lets the LLM request structured input from the user at any point
// during a run. The runner intercepts calls to this tool and routes them through
// the ElicitationHandler, which blocks until the user submits the form.
func askUserTool() LLMTool {
	return LLMTool{
		Name:     "ask_user",
		Builtin:  true,
		ReadOnly: true,
		Description: `Present a structured form to collect user input. You MUST use this tool whenever you need ANY information — never ask via plain text.

Parameters:
- message: Brief instruction shown above the form.
- questions: Array of question objects, each rendered as a form field.

Each question object:
- id: Field identifier (snake_case).
- question: Label displayed to the user.
- type: "text" (default), "select", or "confirm".
- required: Whether the field is mandatory (default true).
- options: Array of {label, description} for "select" type. 2-6 options. User can always type a custom value.

RULES:
- When the field has a known set of valid values (categories, statuses, types, environments), ALWAYS use type "select" with options.
- For each option, include a short description explaining when to choose it.
- Group related fields in a single ask_user call (1-6 questions).
- Use concise labels — the description carries the detail.

Example — creating a skill:
ask_user(
  message="Skill details",
  questions=[
    {id:"name", question:"Skill name", type:"text"},
    {id:"description", question:"Description", type:"text"},
    {id:"category", question:"Category", type:"select", options:[
      {label:"rag", description:"Document search and retrieval"},
      {label:"data", description:"SQL queries and data analysis"},
      {label:"integration", description:"HTTP APIs and external services"},
      {label:"platform", description:"AgentHub management operations"},
      {label:"compute", description:"Code execution and calculations"}
    ]}
  ]
)`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"message": {
					"type": "string",
					"description": "Brief instruction shown above the form"
				},
				"questions": {
					"type": "array",
					"description": "Form fields to present to the user",
					"items": {
						"type": "object",
						"properties": {
							"id": {
								"type": "string",
								"description": "Field identifier (snake_case)"
							},
							"question": {
								"type": "string",
								"description": "Label displayed to the user"
							},
							"type": {
								"type": "string",
								"enum": ["text", "select", "confirm"],
								"description": "Field type: text for free input, select for choices, confirm for yes/no"
							},
							"required": {
								"type": "boolean",
								"description": "Whether the field is mandatory (default true)"
							},
							"options": {
								"type": "array",
								"description": "Choices for select type. 2-6 options.",
								"items": {
									"type": "object",
									"properties": {
										"label": {"type": "string", "description": "Option value shown to user"},
										"description": {"type": "string", "description": "Brief explanation of this option"}
									},
									"required": ["label"]
								}
							}
						},
						"required": ["id", "question"]
					}
				}
			},
			"required": ["message", "questions"]
		}`),
	}
}

// agentTool returns the builtin agent tool for sub-agent spawning.
func agentTool(remainingLevels int) LLMTool {
	desc := fmt.Sprintf(
		"Spawn a sub-agent to handle a specific subtask autonomously. "+
			"The sub-agent inherits all your tools, knowledge bases, and permissions. "+
			"ONLY use this when the subtask is genuinely complex, requires independent context, or can be run in parallel with other subtasks. "+
			"Do NOT use this for simple calculations, direct answers, or tasks you can complete immediately yourself. "+
			"The sub-agent will return its result as text. "+
			"Remaining delegation depth: %d level(s).",
		remainingLevels,
	)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {
				"type": "string",
				"description": "Clear, specific description of the subtask for the sub-agent to complete"
			},
			"tools": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Optional list of specific tool names the sub-agent should use. If omitted, all tools are available."
			}
		},
		"required": ["prompt"]
	}`)
	return LLMTool{
		Name:        agentToolName,
		Description: desc,
		InputSchema: schema,
		Builtin:     true,
	}
}

// agentHubManageTool returns the builtin agenthub_manage tool definition.
// This is the core of "Auto-Reflection" — allowing the agent to perform
// CRUD operations on agents, skills, tools, and integrations.
func agentHubManageTool() LLMTool {
	return LLMTool{
		Name:    "agenthub_manage",
		Builtin: true,
		Description: `Manage AgentHub CONFIGURATION — agents, skills, tools, integrations, and MCP servers.
Use ONLY for administrative/CRUD tasks: listing or editing configurations, not for executing tasks or retrieving user data.
Do NOT use this tool to execute a skill or invoke a domain tool — each executable capability is a separate tool in the schema.
The 'operation' can be: list, get, create, update, delete.
The 'resource' can be: agent, skill, tool, integration, mcp_server.`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"operation": {
					"type": "string",
					"enum": ["list", "get", "create", "update", "delete"],
					"description": "The administrative operation to perform"
				},
				"resource": {
					"type": "string",
					"enum": ["agent", "skill", "tool", "integration", "mcp_server"],
					"description": "The type of resource to manage"
				},
				"id": {
					"type": "string",
					"description": "UUID of the resource (required for get, update, delete)"
				},
				"payload": {
					"type": "object",
					"description": "JSON payload for create or update operations"
				},
				"query": {
					"type": "string",
					"description": "Filter or search query for list operations"
				}
			},
			"required": ["operation", "resource"]
		}`),
	}
}

// toolSearchTool returns the builtin tool_search tool for loading deferred tools.
// Inspired by Claude Code's ToolSearchTool — enables the LLM to load tool schemas
// on demand rather than including all schemas in the initial prompt.
func toolSearchTool(deferred []LLMTool) LLMTool {
	var names []string
	for _, t := range deferred {
		names = append(names, t.Name)
	}
	desc := fmt.Sprintf(
		"Fetches full schema definitions for deferred tools so they can be called. "+
			"The following tools are available but require loading first: %s. "+
			"Use the 'query' parameter with a tool name to load its schema, "+
			"or use keywords to search for tools by capability.",
		join(names, ", "),
	)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Tool name to load, or keywords to search (e.g. 'create agent', 'delete knowledge base')"
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of tools to return (default 5)"
			}
		},
		"required": ["query"]
	}`)
	return LLMTool{
		Name:        "tool_search",
		Description: desc,
		InputSchema: schema,
		ReadOnly:    true,
		Builtin:     true,
	}
}

// readOnlySlugs lists skill slugs that are known to be read-only (no side effects).
// Used to determine concurrency safety during tool execution.
var readOnlySlugs = map[string]bool{
	"document-search": true,
	"document_search": true, // builtin uses underscore
	"web-scraper":     true,
	"http-get":        true,
	"memory-recall":   true,
	"troubleshoot":    true, // diagnosis only, no side effects
	"tool_search":     true, // builtin, no side effects
	"ask_user":        true, // builtin, only collects user input — no side effects
}

// IsReadOnlyTool returns true if the tool name is known to be read-only.
func IsReadOnlyTool(name string) bool {
	return readOnlySlugs[name]
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%s and %s", join(names[:len(names)-1], ", "), names[len(names)-1])
}

func join(elems []string, sep string) string {
	result := ""
	for i, e := range elems {
		if i > 0 {
			result += sep
		}
		result += e
	}
	return result
}

// summarizeMCPErrors parses one or more server failures from a raw MCP error and
// returns one concise, LLM-readable warning string per failing server.
// Handles two error formats:
//  1. Legacy: "no MCP tools available (s1: detail; s2: detail)"
//  2. Current: "mcpbridge: list tools: MCP server \"name\": detail; MCP server \"name2\": detail"
func summarizeMCPErrors(raw string) []string {
	// Format 2: current mcpbridge format — parse "MCP server \"name\": detail" segments.
	const mcpPrefix = "MCP server \""
	if strings.Contains(raw, mcpPrefix) {
		// Split on "; MCP server " to get individual server failure segments.
		segments := strings.Split(raw, "; MCP server \"")
		var warnings []string
		for i, seg := range segments {
			// First segment still has the full prefix; strip it.
			if i == 0 {
				if idx := strings.Index(seg, mcpPrefix); idx >= 0 {
					seg = seg[idx+len(mcpPrefix):]
				} else {
					continue
				}
			}
			// seg is now: name\": detail
			end := strings.IndexByte(seg, '"')
			if end <= 0 {
				continue
			}
			serverName := seg[:end]
			detail := seg[end+1:]
			// Strip leading ": "
			detail = strings.TrimPrefix(detail, ": ")
			warnings = append(warnings, summarizeMCPError(serverName, detail))
		}
		if len(warnings) > 0 {
			return warnings
		}
	}

	// Format 1 (legacy): "no MCP tools available (s1: detail; s2: detail)"
	const multiPrefix = "no MCP tools available ("
	if idx := strings.Index(raw, multiPrefix); idx >= 0 {
		inner := raw[idx+len(multiPrefix):]
		// Strip trailing ")" if present.
		if end := strings.LastIndexByte(inner, ')'); end > 0 {
			inner = inner[:end]
		}
		parts := strings.Split(inner, "; ")
		var warnings []string
		for _, part := range parts {
			colon := strings.IndexByte(part, ':')
			if colon <= 0 {
				continue
			}
			serverName := strings.TrimSpace(part[:colon])
			detail := part[colon+1:]
			warnings = append(warnings, summarizeMCPError(serverName, detail))
		}
		if len(warnings) > 0 {
			return warnings
		}
	}

	// Single-server fallback: extract server name from "for server 'name'"
	serverName := ""
	if idx := strings.Index(raw, "for server '"); idx >= 0 {
		rest := raw[idx+len("for server '"):]
		if end := strings.IndexByte(rest, '\''); end > 0 {
			serverName = rest[:end]
		}
	}
	return []string{summarizeMCPError(serverName, raw)}
}

// summarizeMCPError builds a single LLM-readable warning for one server failure.
func summarizeMCPError(serverName, detail string) string {
	lower := strings.ToLower(detail)
	var reason string
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "oauth") || strings.Contains(lower, "token expired"):
		reason = "authentication required — OAuth token expired or missing"
	case strings.Contains(lower, "403") || strings.Contains(lower, "forbidden"):
		reason = "access forbidden"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "dial tcp") || strings.Contains(lower, "lookup "):
		reason = "server unreachable"
	default:
		reason = "unavailable"
	}

	if serverName != "" {
		return fmt.Sprintf("MCP server '%s' is unavailable (%s). You cannot use any tools from this server.", serverName, reason)
	}
	return fmt.Sprintf("An MCP server is unavailable (%s). You cannot use its tools.", reason)
}
