package agentic

// ContextWindowAccessTierRegistry models the six access/mutability tiers
// depicted on the left-hand ACCESS axis of Figure 6 in arXiv:2604.14228v1.
//
// §7.1 / Figure 6: "Context construction and memory hierarchy. Sources
// converging on the context window include system prompt, output styles,
// environment info, the CLAUDE.md hierarchy (managed through
// directory-specific), auto memory, path-scoped rules, MCP tool names,
// deferred tool definitions via ToolSearch, conversation history, file reads,
// command outputs, tool results, subagent summaries, and compact summaries."
//
// The figure annotates each group of sources with one of six ACCESS labels on
// the left and notes that "ACCESS / Mutability increases" downward:
//
//	1. Read-only       — written once at session start; never changes
//	2. Hot-reload      — reloaded when files change; never mutated directly
//	3. Sys-write       — written by the system (compaction engine, memory system)
//	4. Append          — grows monotonically as turns accumulate
//	5. Model-trigger   — added as a result of model-initiated tool execution
//	6. Lazy-load       — loaded on demand (deferred schemas, ToolSearch)
//
// Context sources are mapped to these tiers in ContextAssemblySourceAccessTier
// (see context_assembly.go for the nine-source list).
//
// This registry is distinct from:
//   - context_assembly.go — the nine ordered context sources (§7.1 bullet list)
//   - context_management_approach.go — compaction strategy enumeration
//   - context_bottleneck.go — §3.6 context-as-bottleneck optimization strategies

// ContextWindowAccessTierID identifies one of the six mutability tiers from
// the Figure 6 ACCESS axis (arXiv:2604.14228v1).
type ContextWindowAccessTierID string

const (
	// ContextAccessTierReadOnly — Tier 1 (lowest mutability).
	// Content is established once at session start from static configurations
	// and never changes during the conversation.
	// Figure 6 group: system prompt, environment info, output styles.
	ContextAccessTierReadOnly ContextWindowAccessTierID = "read_only"

	// ContextAccessTierHotReload — Tier 2.
	// Content is reloaded when the agent reads files in new directories, but the
	// agent itself does not write to these sources. The instruction hierarchy
	// can evolve as different parts of the codebase are explored.
	// Figure 6 group: CLAUDE.md hierarchy (5 levels), path-scoped rules.
	ContextAccessTierHotReload ContextWindowAccessTierID = "hot_reload"

	// ContextAccessTierSysWrite — Tier 3.
	// Content is written by system subsystems (compaction engine, auto-memory
	// prefetch). The model sees it, but only the system mutates it.
	// Figure 6 group: auto memory entries, compact summaries.
	ContextAccessTierSysWrite ContextWindowAccessTierID = "sys_write"

	// ContextAccessTierAppend — Tier 4.
	// Content grows monotonically as the conversation progresses. Messages are
	// appended but never retroactively edited (JSONL append-only log design).
	// Figure 6 group: conversation history, subagent summaries.
	ContextAccessTierAppend ContextWindowAccessTierID = "append"

	// ContextAccessTierModelTrigger — Tier 5.
	// Content is added as a direct consequence of model-initiated tool calls.
	// Each tool execution appends its result; the model's own actions drive growth.
	// Figure 6 group: read files, command outputs, tool results.
	ContextAccessTierModelTrigger ContextWindowAccessTierID = "model_trigger"

	// ContextAccessTierLazyLoad — Tier 6 (highest mutability).
	// Content is injected on demand when the model explicitly invokes ToolSearch.
	// Full schemas replace stub entries, expanding the context at model request.
	// Figure 6 group: deferred tool definitions.
	ContextAccessTierLazyLoad ContextWindowAccessTierID = "lazy_load"
)

// ContextWindowAccessTierProfile holds the immutable Figure 6 metadata for
// one access/mutability tier.
type ContextWindowAccessTierProfile struct {
	// TierID is the canonical identifier for this tier.
	TierID ContextWindowAccessTierID

	// MutabilityRank is the 1-based position on the Figure 6 ACCESS axis.
	// Rank 1 = least mutable (read_only) → Rank 6 = most mutable (lazy_load).
	MutabilityRank int

	// Label is the short access label shown on Figure 6's left axis.
	Label string

	// Description explains the mutability semantics for this tier.
	Description string

	// PDFSection cites the primary paper section anchoring this tier.
	PDFSection string

	// ExampleSources lists the Figure 6 content groups that populate this tier.
	ExampleSources []string

	// WrittenBySystem indicates the content is produced by a system subsystem
	// (compaction engine, memory prefetch) rather than external input or the model.
	WrittenBySystem bool

	// WrittenByModel indicates the content is added as a direct result of
	// the model issuing tool calls (model-trigger tier only).
	WrittenByModel bool

	// IsLazyResolved indicates the content is not present at context assembly
	// time but is injected on demand during the turn (lazy-load tier only).
	IsLazyResolved bool

	// IsAppendOnly indicates content in this tier is only ever appended
	// and never retroactively modified or deleted (append and model-trigger tiers).
	IsAppendOnly bool
}

// contextWindowAccessTierProfiles is the package-level seed for all six tiers.
var contextWindowAccessTierProfiles = []*ContextWindowAccessTierProfile{
	{
		TierID:          ContextAccessTierReadOnly,
		MutabilityRank:  1,
		Label:           "Read-only",
		Description:     "Established once at session start from static configuration; never changes during the conversation.",
		PDFSection:      "7.1",
		ExampleSources:  []string{"system_prompt", "environment_info", "output_styles"},
		WrittenBySystem: false,
		WrittenByModel:  false,
		IsLazyResolved:  false,
		IsAppendOnly:    false,
	},
	{
		TierID:          ContextAccessTierHotReload,
		MutabilityRank:  2,
		Label:           "Hot-reload",
		Description:     "Reloaded when the agent reads files in new directories; the instruction hierarchy evolves as different parts of the codebase are explored, but the agent never writes to these sources.",
		PDFSection:      "7.1",
		ExampleSources:  []string{"claude_md_hierarchy", "path_scoped_rules"},
		WrittenBySystem: false,
		WrittenByModel:  false,
		IsLazyResolved:  false,
		IsAppendOnly:    false,
	},
	{
		TierID:          ContextAccessTierSysWrite,
		MutabilityRank:  3,
		Label:           "Sys-write",
		Description:     "Written by system subsystems (compaction engine, auto-memory prefetch). The model sees the content but only the system mutates it.",
		PDFSection:      "7.1",
		ExampleSources:  []string{"auto_memory", "compact_summaries"},
		WrittenBySystem: true,
		WrittenByModel:  false,
		IsLazyResolved:  false,
		IsAppendOnly:    false,
	},
	{
		TierID:          ContextAccessTierAppend,
		MutabilityRank:  4,
		Label:           "Append",
		Description:     "Grows monotonically as the conversation progresses; messages are appended to the JSONL transcript log and never retroactively edited.",
		PDFSection:      "7.1",
		ExampleSources:  []string{"conversation_history", "subagent_summaries"},
		WrittenBySystem: false,
		WrittenByModel:  false,
		IsLazyResolved:  false,
		IsAppendOnly:    true,
	},
	{
		TierID:          ContextAccessTierModelTrigger,
		MutabilityRank:  5,
		Label:           "Model-trigger",
		Description:     "Added as a direct consequence of model-initiated tool calls. Each tool execution appends its result; the model's own actions drive context growth.",
		PDFSection:      "7.1",
		ExampleSources:  []string{"tool_results", "read_files", "command_outputs"},
		WrittenBySystem: false,
		WrittenByModel:  true,
		IsLazyResolved:  false,
		IsAppendOnly:    true,
	},
	{
		TierID:          ContextAccessTierLazyLoad,
		MutabilityRank:  6,
		Label:           "Lazy-load",
		Description:     "Injected on demand when the model explicitly invokes ToolSearch. Full tool schemas replace deferred stubs, expanding the context at model request time.",
		PDFSection:      "7.1",
		ExampleSources:  []string{"deferred_tool_definitions"},
		WrittenBySystem: false,
		WrittenByModel:  true,
		IsLazyResolved:  true,
		IsAppendOnly:    false,
	},
}

// SeedContextWindowAccessTierCount is the number of Figure 6 access tiers.
const SeedContextWindowAccessTierCount = 6

// SeedContextWindowAccessTierIDs is the ordered list of tier IDs used for
// testing invariants.
var SeedContextWindowAccessTierIDs = []ContextWindowAccessTierID{
	ContextAccessTierReadOnly,
	ContextAccessTierHotReload,
	ContextAccessTierSysWrite,
	ContextAccessTierAppend,
	ContextAccessTierModelTrigger,
	ContextAccessTierLazyLoad,
}

// contextWindowAccessTierByID is the fast-lookup index built at init time.
var contextWindowAccessTierByID map[ContextWindowAccessTierID]*ContextWindowAccessTierProfile

func init() {
	contextWindowAccessTierByID = make(map[ContextWindowAccessTierID]*ContextWindowAccessTierProfile, SeedContextWindowAccessTierCount)
	for _, p := range contextWindowAccessTierProfiles {
		contextWindowAccessTierByID[p.TierID] = p
	}
}

// ContextWindowAccessTierRegistry provides Figure 6 queries over the six
// context-window access/mutability tiers from arXiv:2604.14228v1.
//
// The registry is read-only; callers must not modify the returned profiles.
type ContextWindowAccessTierRegistry struct{}

// NewContextWindowAccessTierRegistry returns a ready-to-use registry.
func NewContextWindowAccessTierRegistry() *ContextWindowAccessTierRegistry {
	return &ContextWindowAccessTierRegistry{}
}

// FindContextWindowAccessTierBySlug looks up a tier profile by its ID string.
// Returns (profile, true) if found; (nil, false) if unknown.
func (r *ContextWindowAccessTierRegistry) FindContextWindowAccessTierBySlug(slug string) (*ContextWindowAccessTierProfile, bool) {
	p, ok := contextWindowAccessTierByID[ContextWindowAccessTierID(slug)]
	return p, ok
}

// AllTiers returns all six access tier profiles in mutability-ascending order
// (read_only first → lazy_load last). Returns a defensive copy of the slice.
func (r *ContextWindowAccessTierRegistry) AllTiers() []*ContextWindowAccessTierProfile {
	result := make([]*ContextWindowAccessTierProfile, len(contextWindowAccessTierProfiles))
	copy(result, contextWindowAccessTierProfiles)
	return result
}

// Count returns the number of registered access tiers.
func (r *ContextWindowAccessTierRegistry) Count() int {
	return len(contextWindowAccessTierProfiles)
}

// IsValidAccessTierID returns true if the given string is one of the six
// canonical Figure 6 tier identifiers.
func (r *ContextWindowAccessTierRegistry) IsValidAccessTierID(id string) bool {
	_, ok := contextWindowAccessTierByID[ContextWindowAccessTierID(id)]
	return ok
}

// TierByMutabilityRank returns the tier at the given 1-based rank.
// Returns (nil, false) if rank is out of range [1, 6].
func (r *ContextWindowAccessTierRegistry) TierByMutabilityRank(rank int) (*ContextWindowAccessTierProfile, bool) {
	for _, p := range contextWindowAccessTierProfiles {
		if p.MutabilityRank == rank {
			return p, true
		}
	}
	return nil, false
}

// LeastMutableTier returns the tier with MutabilityRank == 1 (read_only).
func (r *ContextWindowAccessTierRegistry) LeastMutableTier() *ContextWindowAccessTierProfile {
	p, _ := r.TierByMutabilityRank(1)
	return p
}

// MostMutableTier returns the tier with the highest MutabilityRank (lazy_load).
func (r *ContextWindowAccessTierRegistry) MostMutableTier() *ContextWindowAccessTierProfile {
	p, _ := r.TierByMutabilityRank(SeedContextWindowAccessTierCount)
	return p
}

// TiersWrittenBySystem returns all tiers where the content is produced by a
// system subsystem rather than external input or the model.
// Figure 6: sys_write (auto memory, compact summaries).
func (r *ContextWindowAccessTierRegistry) TiersWrittenBySystem() []*ContextWindowAccessTierProfile {
	var result []*ContextWindowAccessTierProfile
	for _, p := range contextWindowAccessTierProfiles {
		if p.WrittenBySystem {
			result = append(result, p)
		}
	}
	return result
}

// TiersWrittenByModel returns all tiers whose content is appended as a direct
// consequence of model-initiated tool calls.
// Figure 6: model_trigger and lazy_load.
func (r *ContextWindowAccessTierRegistry) TiersWrittenByModel() []*ContextWindowAccessTierProfile {
	var result []*ContextWindowAccessTierProfile
	for _, p := range contextWindowAccessTierProfiles {
		if p.WrittenByModel {
			result = append(result, p)
		}
	}
	return result
}

// AppendOnlyTiers returns tiers where content is only ever appended and never
// retroactively modified (append and model_trigger tiers).
func (r *ContextWindowAccessTierRegistry) AppendOnlyTiers() []*ContextWindowAccessTierProfile {
	var result []*ContextWindowAccessTierProfile
	for _, p := range contextWindowAccessTierProfiles {
		if p.IsAppendOnly {
			result = append(result, p)
		}
	}
	return result
}

// LazyResolvedTiers returns tiers where content is not present at assembly time
// but injected on demand.
// Figure 6: lazy_load only.
func (r *ContextWindowAccessTierRegistry) LazyResolvedTiers() []*ContextWindowAccessTierProfile {
	var result []*ContextWindowAccessTierProfile
	for _, p := range contextWindowAccessTierProfiles {
		if p.IsLazyResolved {
			result = append(result, p)
		}
	}
	return result
}

// IsMutableAtRuntime returns true if a tier's content can change after the
// initial session assembly. Read-only and hot-reload tiers are considered
// functionally immutable within a single turn; all others can change.
func (r *ContextWindowAccessTierRegistry) IsMutableAtRuntime(id ContextWindowAccessTierID) bool {
	p, ok := contextWindowAccessTierByID[id]
	if !ok {
		return false
	}
	return p.MutabilityRank >= 3
}

// TiersMoreMutableThan returns all tiers with a higher MutabilityRank than
// the given tier, in ascending rank order.
func (r *ContextWindowAccessTierRegistry) TiersMoreMutableThan(id ContextWindowAccessTierID) []*ContextWindowAccessTierProfile {
	p, ok := contextWindowAccessTierByID[id]
	if !ok {
		return nil
	}
	var result []*ContextWindowAccessTierProfile
	for _, tier := range contextWindowAccessTierProfiles {
		if tier.MutabilityRank > p.MutabilityRank {
			result = append(result, tier)
		}
	}
	return result
}

// TiersLessMutableThan returns all tiers with a lower MutabilityRank than
// the given tier, in ascending rank order.
func (r *ContextWindowAccessTierRegistry) TiersLessMutableThan(id ContextWindowAccessTierID) []*ContextWindowAccessTierProfile {
	p, ok := contextWindowAccessTierByID[id]
	if !ok {
		return nil
	}
	var result []*ContextWindowAccessTierProfile
	for _, tier := range contextWindowAccessTierProfiles {
		if tier.MutabilityRank < p.MutabilityRank {
			result = append(result, tier)
		}
	}
	return result
}

// --- Structural invariants ---

// ContextWindowAccessTierMutabilityRanksAreContiguous validates that the six
// tiers have MutabilityRank values 1, 2, 3, 4, 5, 6 with no gaps.
func ContextWindowAccessTierMutabilityRanksAreContiguous() bool {
	seen := make(map[int]bool, SeedContextWindowAccessTierCount)
	for _, p := range contextWindowAccessTierProfiles {
		if p.MutabilityRank < 1 || p.MutabilityRank > SeedContextWindowAccessTierCount {
			return false
		}
		if seen[p.MutabilityRank] {
			return false
		}
		seen[p.MutabilityRank] = true
	}
	return len(seen) == SeedContextWindowAccessTierCount
}

// ContextWindowAccessTierOrderMatchesRank validates that contextWindowAccessTierProfiles
// is sorted in ascending MutabilityRank order (the canonical Figure 6 top-to-bottom order).
func ContextWindowAccessTierOrderMatchesRank() bool {
	if len(contextWindowAccessTierProfiles) != len(contextWindowAccessTierOrder) {
		return false
	}
	for i, p := range contextWindowAccessTierProfiles {
		if p.TierID != contextWindowAccessTierOrder[i] || p.MutabilityRank != i+1 {
			return false
		}
	}
	return true
}

// ContextWindowAccessTierExactlyOneLazyResolved validates that exactly one tier
// has IsLazyResolved = true. Figure 6: only lazy_load is deferred.
func ContextWindowAccessTierExactlyOneLazyResolved() bool {
	count := 0
	for _, p := range contextWindowAccessTierProfiles {
		if p.IsLazyResolved {
			count++
		}
	}
	return count == 1
}

// ContextWindowAccessTierLazyLoadIsHighestRank validates that the lazy_load tier
// has MutabilityRank == SeedContextWindowAccessTierCount (6).
// Figure 6 places lazy-load at the bottom of the ACCESS axis (most mutable).
func ContextWindowAccessTierLazyLoadIsHighestRank() bool {
	p, ok := contextWindowAccessTierByID[ContextAccessTierLazyLoad]
	return ok && p.MutabilityRank == SeedContextWindowAccessTierCount
}

// ContextWindowAccessTierReadOnlyIsLowestRank validates that the read_only tier
// has MutabilityRank == 1.
func ContextWindowAccessTierReadOnlyIsLowestRank() bool {
	p, ok := contextWindowAccessTierByID[ContextAccessTierReadOnly]
	return ok && p.MutabilityRank == 1
}
