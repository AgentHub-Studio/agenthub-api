package agentic

// ContextAssemblySource identifies one of the nine ordered sources that compose
// the context window sent to the model on each turn.
// §7.1: "Nine sources assembled in a fixed order before each model call."
type ContextAssemblySource string

const (
	// ContextSourceSystemPrompt is the agent system prompt with output-style
	// modifications and any --append-system-prompt content.
	ContextSourceSystemPrompt ContextAssemblySource = "system_prompt"

	// ContextSourceEnvironmentInfo is platform metadata from getSystemContext():
	// git status (skipped in remote mode), optional cache break injection.
	// Memoized once per session.
	ContextSourceEnvironmentInfo ContextAssemblySource = "environment_info"

	// ContextSourceClaudeMDHierarchy is the CLAUDE.md four-level hierarchy
	// loaded by getUserContext(). Memoized once per session.
	ContextSourceClaudeMDHierarchy ContextAssemblySource = "claude_md_hierarchy"

	// ContextSourcePathScopedRules are directory-matched instruction files loaded
	// lazily when the agent reads files in matching directories.
	ContextSourcePathScopedRules ContextAssemblySource = "path_scoped_rules"

	// ContextSourceAutoMemory is contextually relevant memory entries prefetched
	// asynchronously (file granularity, up to five files, no vector similarity).
	ContextSourceAutoMemory ContextAssemblySource = "auto_memory"

	// ContextSourceToolMetadata contains skill descriptions, MCP tool names, and
	// deferred tool definitions (resolved via ToolSearch on demand).
	ContextSourceToolMetadata ContextAssemblySource = "tool_metadata"

	// ContextSourceConversationHistory is the full message history carried
	// forward from prior turns, subject to compaction.
	ContextSourceConversationHistory ContextAssemblySource = "conversation_history"

	// ContextSourceToolResults contains the outputs of tool executions:
	// file reads, command outputs, subagent summaries.
	ContextSourceToolResults ContextAssemblySource = "tool_results"

	// ContextSourceCompactSummaries are LLM-generated summaries that replace
	// older history segments after compaction events.
	ContextSourceCompactSummaries ContextAssemblySource = "compact_summaries"
)

// ContextAssemblyDomain groups sources by their functional role.
type ContextAssemblyDomain string

const (
	ContextDomainPromptConstruction ContextAssemblyDomain = "prompt_construction" // sources 1
	ContextDomainPlatformContext    ContextAssemblyDomain = "platform_context"    // source 2
	ContextDomainInstructionFiles   ContextAssemblyDomain = "instruction_files"   // sources 3-4
	ContextDomainMemory             ContextAssemblyDomain = "memory"              // source 5
	ContextDomainToolDefinitions    ContextAssemblyDomain = "tool_definitions"    // source 6
	ContextDomainConversation       ContextAssemblyDomain = "conversation"        // sources 7-9
)

// ContextAssemblySourceProfile is the immutable characteristics of one context source.
type ContextAssemblySourceProfile struct {
	Source          ContextAssemblySource
	SourceOrder     int                   // 1–9, canonical assembly order per §7.1
	Domain          ContextAssemblyDomain
	IsMemoized      bool // content is computed once and cached for the session lifetime
	IsAsynchronous  bool // content is prefetched asynchronously (does not block assembly)
	IsAlwaysIncluded bool // always present in every turn (false = conditional / lazy)
}

var contextAssemblySourceProfiles = map[ContextAssemblySource]ContextAssemblySourceProfile{
	ContextSourceSystemPrompt: {
		Source: ContextSourceSystemPrompt, SourceOrder: 1, Domain: ContextDomainPromptConstruction,
		IsMemoized: false, IsAsynchronous: false, IsAlwaysIncluded: true,
	},
	ContextSourceEnvironmentInfo: {
		Source: ContextSourceEnvironmentInfo, SourceOrder: 2, Domain: ContextDomainPlatformContext,
		IsMemoized: true, IsAsynchronous: false, IsAlwaysIncluded: true,
	},
	ContextSourceClaudeMDHierarchy: {
		Source: ContextSourceClaudeMDHierarchy, SourceOrder: 3, Domain: ContextDomainInstructionFiles,
		IsMemoized: true, IsAsynchronous: false, IsAlwaysIncluded: true,
	},
	ContextSourcePathScopedRules: {
		Source: ContextSourcePathScopedRules, SourceOrder: 4, Domain: ContextDomainInstructionFiles,
		IsMemoized: false, IsAsynchronous: false, IsAlwaysIncluded: false,
	},
	ContextSourceAutoMemory: {
		Source: ContextSourceAutoMemory, SourceOrder: 5, Domain: ContextDomainMemory,
		IsMemoized: false, IsAsynchronous: true, IsAlwaysIncluded: false,
	},
	ContextSourceToolMetadata: {
		Source: ContextSourceToolMetadata, SourceOrder: 6, Domain: ContextDomainToolDefinitions,
		IsMemoized: false, IsAsynchronous: true, IsAlwaysIncluded: true,
	},
	ContextSourceConversationHistory: {
		Source: ContextSourceConversationHistory, SourceOrder: 7, Domain: ContextDomainConversation,
		IsMemoized: false, IsAsynchronous: false, IsAlwaysIncluded: true,
	},
	ContextSourceToolResults: {
		Source: ContextSourceToolResults, SourceOrder: 8, Domain: ContextDomainConversation,
		IsMemoized: false, IsAsynchronous: false, IsAlwaysIncluded: true,
	},
	ContextSourceCompactSummaries: {
		Source: ContextSourceCompactSummaries, SourceOrder: 9, Domain: ContextDomainConversation,
		IsMemoized: false, IsAsynchronous: false, IsAlwaysIncluded: false,
	},
}

// ContextAssemblySequence is the canonical ordered slice of all 9 sources per §7.1.
var ContextAssemblySequence = []ContextAssemblySource{
	ContextSourceSystemPrompt,
	ContextSourceEnvironmentInfo,
	ContextSourceClaudeMDHierarchy,
	ContextSourcePathScopedRules,
	ContextSourceAutoMemory,
	ContextSourceToolMetadata,
	ContextSourceConversationHistory,
	ContextSourceToolResults,
	ContextSourceCompactSummaries,
}

// ContextAssemblySourceRegistry provides queries over the §7.1 nine-source assembly order.
type ContextAssemblySourceRegistry struct{}

// NewContextAssemblySourceRegistry returns a ready-to-use registry.
func NewContextAssemblySourceRegistry() *ContextAssemblySourceRegistry {
	return &ContextAssemblySourceRegistry{}
}

// Profile returns the immutable profile for the given source.
// Returns false if the source is unknown.
func (r *ContextAssemblySourceRegistry) Profile(src ContextAssemblySource) (ContextAssemblySourceProfile, bool) {
	p, ok := contextAssemblySourceProfiles[src]
	return p, ok
}

// AllSources returns all nine sources in assembly order as a defensive copy.
func (r *ContextAssemblySourceRegistry) AllSources() []ContextAssemblySource {
	result := make([]ContextAssemblySource, len(ContextAssemblySequence))
	copy(result, ContextAssemblySequence)
	return result
}

// SourcesInDomain returns all sources belonging to the given domain, in assembly order.
func (r *ContextAssemblySourceRegistry) SourcesInDomain(domain ContextAssemblyDomain) []ContextAssemblySource {
	var result []ContextAssemblySource
	for _, src := range ContextAssemblySequence {
		if contextAssemblySourceProfiles[src].Domain == domain {
			result = append(result, src)
		}
	}
	return result
}

// AlwaysIncludedSources returns sources that are present in every turn, in assembly order.
func (r *ContextAssemblySourceRegistry) AlwaysIncludedSources() []ContextAssemblySource {
	var result []ContextAssemblySource
	for _, src := range ContextAssemblySequence {
		if contextAssemblySourceProfiles[src].IsAlwaysIncluded {
			result = append(result, src)
		}
	}
	return result
}

// AsyncSources returns sources that are prefetched asynchronously, in assembly order.
func (r *ContextAssemblySourceRegistry) AsyncSources() []ContextAssemblySource {
	var result []ContextAssemblySource
	for _, src := range ContextAssemblySequence {
		if contextAssemblySourceProfiles[src].IsAsynchronous {
			result = append(result, src)
		}
	}
	return result
}

// MemoizedSources returns sources whose content is cached for the session lifetime.
func (r *ContextAssemblySourceRegistry) MemoizedSources() []ContextAssemblySource {
	var result []ContextAssemblySource
	for _, src := range ContextAssemblySequence {
		if contextAssemblySourceProfiles[src].IsMemoized {
			result = append(result, src)
		}
	}
	return result
}

// IsContextAssemblySequentiallyOrdered validates that ContextAssemblySequence
// has source_orders 1..9 in order. This is a structural invariant of the §7.1 architecture.
func IsContextAssemblySequentiallyOrdered() bool {
	for i, src := range ContextAssemblySequence {
		if contextAssemblySourceProfiles[src].SourceOrder != i+1 {
			return false
		}
	}
	return true
}
