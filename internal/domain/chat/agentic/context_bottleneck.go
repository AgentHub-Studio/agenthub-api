package agentic

// context_bottleneck.go — §3.6 Context as Bottleneck: Beyond Compaction
//
// Models the four context-window optimization strategies described in §3.6 of the
// Claude Code architecture paper (2604.14228v1): CLAUDE.md lazy loading, deferred
// tool schemas, subagent summary-only return, and per-tool-result token budget.
// A fifth strategy, progressive_summarization, is added as the paper's §4.3
// auto-compact shaper maps to the same token-savings category.
//
// These strategies are orthogonal to the five-shaper compaction pipeline (§4.3,
// budget_reduction → snip → microcompact → context_collapse → auto_compact) and
// apply before or around the model call rather than inside it.

import "strings"

// ContextOptimizationStrategy identifies a single §3.6 context management approach.
type ContextOptimizationStrategy string

const (
	// StrategyLazyFrontmatter defers loading nested CLAUDE.md files until the agent
	// actually reads files in those directories, preventing unused instruction files
	// from consuming context (§3.6, bullet 1: "CLAUDE.md lazy loading").
	StrategyLazyFrontmatter ContextOptimizationStrategy = "lazy_frontmatter"

	// StrategyDeferredToolSchema sends only tool names in the initial context; full
	// JSON schemas are injected on demand when ToolSearch is invoked (§3.6, bullet 2:
	// "Deferred tool schemas").
	StrategyDeferredToolSchema ContextOptimizationStrategy = "deferred_tool_schema"

	// StrategySummaryOnlySubagent causes subagents (AgentTool/runAgent) to return
	// only a text synopsis to the parent rather than their full conversation history,
	// preventing sidechain transcripts from inflating the parent's context (§3.6,
	// bullet 3: "Subagent summary-only return"; implementation in §8).
	StrategySummaryOnlySubagent ContextOptimizationStrategy = "summary_only_subagent"

	// StrategyPerResultTokenBudget caps each individual tool result at a configurable
	// token size, replacing oversized outputs with content references so a single
	// verbose tool call cannot consume disproportionate context (§3.6, bullet 4:
	// "Per-tool-result budget"; §4.3 applyToolResultBudget()).
	StrategyPerResultTokenBudget ContextOptimizationStrategy = "per_result_token_budget"

	// StrategyProgressiveSummarization collapses older conversation turns via the
	// auto-compact shaper (compactConversation, §4.3) when context pressure still
	// exceeds the threshold after the four lighter shapers have run.
	StrategyProgressiveSummarization ContextOptimizationStrategy = "progressive_summarization"
)

// allStrategies is the canonical ordered list of §3.6 strategies.
var allStrategies = []ContextOptimizationStrategy{
	StrategyLazyFrontmatter,
	StrategyDeferredToolSchema,
	StrategySummaryOnlySubagent,
	StrategyPerResultTokenBudget,
	StrategyProgressiveSummarization,
}

// TokenSavingsCategory classifies the expected context-token savings of a strategy.
type TokenSavingsCategory string

const (
	// TokenSavingsSmall saves up to ~5 % of context window in typical sessions.
	TokenSavingsSmall TokenSavingsCategory = "small"

	// TokenSavingsMedium saves roughly 5–25 % in typical sessions.
	TokenSavingsMedium TokenSavingsCategory = "medium"

	// TokenSavingsLarge saves 25–60 % in agentic sessions with many tools or subagents.
	TokenSavingsLarge TokenSavingsCategory = "large"

	// TokenSavingsExtreme may save >60 % by collapsing most of the history (auto-compact).
	TokenSavingsExtreme TokenSavingsCategory = "extreme"
)

// ContextOptimizationProfile describes one §3.6 strategy with its metadata.
type ContextOptimizationProfile struct {
	// Strategy is the canonical identifier.
	Strategy ContextOptimizationStrategy

	// Description is a human-readable explanation aligned with the paper text.
	Description string

	// TokenSavingsCategory classifies the expected token savings.
	TokenSavingsCategory TokenSavingsCategory

	// AppliesTo lists the context segments this strategy targets.
	// Typical values: "agent_frontmatter", "tool_schemas", "subagent_results",
	// "tool_results", "conversation_history".
	AppliesTo []string

	// CLISpecific indicates strategies that are meaningful only in a terminal/CLI
	// surface (e.g., nested directory CLAUDE.md) and therefore require adaptation
	// for web/API deployments of AgentHub.
	CLISpecific bool

	// WebAdaptation describes how AgentHub adapts a CLI-specific strategy for a
	// web-service context. Empty when CLISpecific is false.
	WebAdaptation string
}

// ContextBottleneckRegistry is the §3.6 strategy catalogue.
// It is stateless and safe to use concurrently.
type ContextBottleneckRegistry struct {
	profiles map[ContextOptimizationStrategy]ContextOptimizationProfile
}

// NewContextBottleneckRegistry builds a registry pre-populated with all §3.6 strategies.
func NewContextBottleneckRegistry() *ContextBottleneckRegistry {
	r := &ContextBottleneckRegistry{
		profiles: make(map[ContextOptimizationStrategy]ContextOptimizationProfile, len(allStrategies)),
	}

	r.profiles[StrategyLazyFrontmatter] = ContextOptimizationProfile{
		Strategy: StrategyLazyFrontmatter,
		Description: "Base CLAUDE.md hierarchy loaded at session start; nested " +
			"per-directory instruction files injected only when the agent accesses " +
			"files in those directories (§3.6 §7.2).",
		TokenSavingsCategory: TokenSavingsMedium,
		AppliesTo:            []string{"agent_frontmatter"},
		CLISpecific:          true,
		WebAdaptation: "AgentHub agents carry a single system_prompt field; " +
			"lazy loading maps to deferred injection of per-skill or per-KB " +
			"instruction blocks until the relevant resource is first accessed.",
	}

	r.profiles[StrategyDeferredToolSchema] = ContextOptimizationProfile{
		Strategy: StrategyDeferredToolSchema,
		Description: "When ToolSearch is enabled, tools are registered by name only " +
			"in the initial context; full JSON schemas are fetched on demand, " +
			"preventing unused tool definitions from consuming tokens (§3.6).",
		TokenSavingsCategory: TokenSavingsLarge,
		AppliesTo:            []string{"tool_schemas"},
		CLISpecific:          false,
		WebAdaptation: "",
	}

	r.profiles[StrategySummaryOnlySubagent] = ContextOptimizationProfile{
		Strategy: StrategySummaryOnlySubagent,
		Description: "Subagents (AgentTool / runAgent) return only a text summary " +
			"to the parent agent; their full sidechain transcript is stored separately " +
			"and never inflates the parent context window (§3.6 §8).",
		TokenSavingsCategory: TokenSavingsLarge,
		AppliesTo:            []string{"subagent_results"},
		CLISpecific:          false,
		WebAdaptation: "",
	}

	r.profiles[StrategyPerResultTokenBudget] = ContextOptimizationProfile{
		Strategy: StrategyPerResultTokenBudget,
		Description: "Individual tool results are capped at a configurable token " +
			"limit (maxResultSizeChars); oversized outputs are replaced with content " +
			"references persisted in session storage (§3.6 §4.3 applyToolResultBudget).",
		TokenSavingsCategory: TokenSavingsMedium,
		AppliesTo:            []string{"tool_results"},
		CLISpecific:          false,
		WebAdaptation: "",
	}

	r.profiles[StrategyProgressiveSummarization] = ContextOptimizationProfile{
		Strategy: StrategyProgressiveSummarization,
		Description: "The auto-compact shaper triggers a model-generated compression " +
			"of the full conversation history when context pressure exceeds the threshold " +
			"after the four lighter shapers (§4.3 compactConversation / compact.ts).",
		TokenSavingsCategory: TokenSavingsExtreme,
		AppliesTo:            []string{"conversation_history"},
		CLISpecific:          false,
		WebAdaptation: "",
	}

	return r
}

// AllStrategies returns all registered optimization profiles in canonical order.
func (r *ContextBottleneckRegistry) AllStrategies() []ContextOptimizationProfile {
	out := make([]ContextOptimizationProfile, 0, len(allStrategies))
	for _, s := range allStrategies {
		if p, ok := r.profiles[s]; ok {
			out = append(out, p)
		}
	}
	return out
}

// StrategiesForAppliesTo returns profiles whose AppliesTo list contains target.
// target comparison is case-insensitive.
func (r *ContextBottleneckRegistry) StrategiesForAppliesTo(target string) []ContextOptimizationProfile {
	tl := strings.ToLower(target)
	var out []ContextOptimizationProfile
	for _, s := range allStrategies {
		p, ok := r.profiles[s]
		if !ok {
			continue
		}
		for _, a := range p.AppliesTo {
			if strings.ToLower(a) == tl {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// StrategiesBySavingsCategory returns profiles matching the given savings category.
func (r *ContextBottleneckRegistry) StrategiesBySavingsCategory(cat TokenSavingsCategory) []ContextOptimizationProfile {
	var out []ContextOptimizationProfile
	for _, s := range allStrategies {
		p, ok := r.profiles[s]
		if !ok {
			continue
		}
		if p.TokenSavingsCategory == cat {
			out = append(out, p)
		}
	}
	return out
}

// IsValidStrategy reports whether s is a registered strategy.
func (r *ContextBottleneckRegistry) IsValidStrategy(s ContextOptimizationStrategy) bool {
	_, ok := r.profiles[s]
	return ok
}

// AgentHubAdaptedStrategies returns profiles that are either not CLI-specific or have
// a web adaptation defined, making them applicable to AgentHub's web/API surface.
func (r *ContextBottleneckRegistry) AgentHubAdaptedStrategies() []ContextOptimizationProfile {
	var out []ContextOptimizationProfile
	for _, s := range allStrategies {
		p, ok := r.profiles[s]
		if !ok {
			continue
		}
		if !p.CLISpecific || p.WebAdaptation != "" {
			out = append(out, p)
		}
	}
	return out
}

// Profile returns the profile for a given strategy and a boolean indicating presence.
func (r *ContextBottleneckRegistry) Profile(s ContextOptimizationStrategy) (ContextOptimizationProfile, bool) {
	p, ok := r.profiles[s]
	return p, ok
}
