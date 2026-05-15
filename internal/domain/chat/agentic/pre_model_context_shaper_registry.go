package agentic

// PreModelContextShaperRegistry models the five sequential context shapers that
// execute in query.ts before every model call (§4.3 "Pre-Model Context Shapers").
//
// §4.3: "Five context shapers execute sequentially in query.ts before every model
// call, each operating on the messagesForQuery array. The five shapers run in
// sequence, with earlier steps applying lighter reductions before later steps
// apply broader compaction."
//
// Key differences from CompactionPipelineLayer (§7.3):
//   - §4.3 describes the per-turn per-call execution contract inside query.ts
//   - §7.3 describes the broader graduated pipeline architecture
//   - §4.3 records feature-flag gating, source function, token-savings reporting,
//     and composability notes that are specific to the query.ts integration
//
// The five shapers in execution order:
//  1. Budget reduction  — applyToolResultBudget()         always active
//  2. Snip             — snipCompactIfNeeded()             HISTORY_SNIP flag
//  3. Microcompact     — microcompact()                   CACHED_MICROCOMPACT flag
//  4. Context collapse — applyCollapsesIfNeeded()          CONTEXT_COLLAPSE flag
//  5. Auto-compact     — compactConversation()             enabled by default

// PreModelContextShaperID is the canonical slug of one of the five §4.3 shapers.
type PreModelContextShaperID string

const (
	// ShaperBudgetReduction is shaper 1 (§4.3): applyToolResultBudget().
	//
	// Enforces per-message size limits on tool results, replacing oversized
	// outputs with content references. Exempt tools (maxResultSizeChars is not
	// finite) retain their full output. Content replacements are persisted for
	// agent and session query sources to enable reconstruction on resume.
	//
	// §4.3: "Budget reduction runs before microcompact because microcompact
	// operates purely by tool_use_id and never inspects content; the two
	// compose cleanly."
	ShaperBudgetReduction PreModelContextShaperID = "budget_reduction"

	// ShaperSnip is shaper 2 (§4.3): snipCompactIfNeeded(), gated by HISTORY_SNIP.
	//
	// A lightweight trim that removes older history segments, returning
	// {messages, tokensFreed, boundaryMessage}. The snipTokensFreed value is
	// plumbed to auto-compact because the main token counter derives context
	// size from the usage field on the most recent assistant message, and that
	// message survives snip with its pre-snip input_tokens still attached;
	// snip's savings are therefore invisible to the counter unless passed
	// explicitly.
	ShaperSnip PreModelContextShaperID = "snip"

	// ShaperMicrocompact is shaper 3 (§4.3): microcompact(), gated by CACHED_MICROCOMPACT.
	//
	// Fine-grained compression that always runs a time-based path and optionally
	// a cache-aware path. When the cached path is enabled, boundary messages are
	// deferred until after the API response so they can use actual
	// cache_deleted_input_tokens rather than estimates. Returns
	// {messages, compactionInfo} where compactionInfo may include pendingCacheEdits.
	ShaperMicrocompact PreModelContextShaperID = "microcompact"

	// ShaperContextCollapse is shaper 4 (§4.3): applyCollapsesIfNeeded(), gated by CONTEXT_COLLAPSE.
	//
	// A read-time projection over the conversation history. The source comments
	// explain: "Nothing is yielded; the collapsed view is a read-time projection
	// over the REPL's full history. Summary messages live in the collapse store,
	// not the REPL array. This is what makes collapses persist across turns."
	// Unlike the other shapers, context collapse does not mutate the REPL's
	// stored history; it replaces the messagesForQuery array with a projected
	// view, so the model sees the collapsed version while the full history
	// remains available for reconstruction.
	ShaperContextCollapse PreModelContextShaperID = "context_collapse"

	// ShaperAutoCompact is shaper 5 (§4.3): compactConversation() in compact.ts.
	//
	// The fifth shaper, triggering a full model-generated summary via
	// compactConversation(). This function executes PreCompact hooks, creates a
	// summary request using getCompactPrompt(), and calls the model to produce a
	// compressed summary. The result feeds into buildPostCompactMessages().
	// Auto-compact fires only when the context still exceeds the pressure
	// threshold after all four previous shapers have run.
	ShaperAutoCompact PreModelContextShaperID = "auto_compact"
)

// ShaperFeatureFlag is the identifier of the feature flag that gates a shaper.
// An empty string means the shaper has no flag (always active or user-toggle).
type ShaperFeatureFlag string

const (
	// ShaperFlagNone means the shaper is always active — no feature flag gates it.
	ShaperFlagNone ShaperFeatureFlag = ""

	// ShaperFlagHistorySnip is the HISTORY_SNIP feature flag (gates ShaperSnip).
	ShaperFlagHistorySnip ShaperFeatureFlag = "HISTORY_SNIP"

	// ShaperFlagCachedMicrocompact is the CACHED_MICROCOMPACT flag (gates ShaperMicrocompact).
	ShaperFlagCachedMicrocompact ShaperFeatureFlag = "CACHED_MICROCOMPACT"

	// ShaperFlagContextCollapse is the CONTEXT_COLLAPSE flag (gates ShaperContextCollapse).
	ShaperFlagContextCollapse ShaperFeatureFlag = "CONTEXT_COLLAPSE"
)

// ShaperActivationMode describes how a shaper is enabled.
type ShaperActivationMode string

const (
	// ShaperActivationAlways means the shaper is always applied — not gated by any flag.
	ShaperActivationAlways ShaperActivationMode = "always"

	// ShaperActivationFeatureFlag means the shaper runs only when its feature flag is active.
	ShaperActivationFeatureFlag ShaperActivationMode = "feature_flag"

	// ShaperActivationDefaultOn means the shaper is enabled by default but the user
	// may disable it via configuration (ShaperAutoCompact).
	ShaperActivationDefaultOn ShaperActivationMode = "default_on"
)

// ShaperMutationStyle describes how the shaper modifies the messagesForQuery array.
type ShaperMutationStyle string

const (
	// ShaperMutationInPlace means the shaper replaces message content in the existing array.
	// The original REPL history is mutated or referenced changes are persisted.
	ShaperMutationInPlace ShaperMutationStyle = "in_place"

	// ShaperMutationTrimming means the shaper removes messages from the tail/head of the array.
	ShaperMutationTrimming ShaperMutationStyle = "trimming"

	// ShaperMutationProjection means the shaper replaces the entire messagesForQuery array with
	// a projected view; the underlying REPL history is NOT mutated.
	// §4.3 (context collapse): "collapses persist across turns" via a separate store.
	ShaperMutationProjection ShaperMutationStyle = "projection"

	// ShaperMutationReplacement means the shaper replaces the entire history with a
	// model-generated summary (auto-compact).
	ShaperMutationReplacement ShaperMutationStyle = "replacement"
)

// PreModelContextShaperProfile is the immutable metadata for one §4.3 shaper.
//
// The profile captures all structural properties the architecture paper specifies:
// execution order, source function, feature flag, composability notes, and
// whether the shaper produces a tokensFreed signal for the downstream auto-compact.
type PreModelContextShaperProfile struct {
	// ID is the canonical shaper identifier.
	ID PreModelContextShaperID

	// PDFSection is the arXiv:2604.14228v1 section that specifies this shaper.
	PDFSection string

	// ExecutionOrder is the position in the sequential run (1=first, 5=last).
	// §4.3: "Five shapers run in sequence, with earlier steps applying lighter
	// reductions before later steps apply broader compaction."
	ExecutionOrder int

	// SourceFunction is the query.ts function that implements this shaper.
	SourceFunction string

	// ActivationMode describes how this shaper is enabled.
	ActivationMode ShaperActivationMode

	// FeatureFlag is the flag that gates this shaper (empty if ActivationAlways or ActivationDefaultOn).
	FeatureFlag ShaperFeatureFlag

	// MutationStyle describes how the shaper modifies messagesForQuery.
	MutationStyle ShaperMutationStyle

	// ReturnsTokensFreed indicates whether this shaper returns a tokensFreed
	// count that must be passed explicitly to auto-compact (shaper 5).
	//
	// §4.3 (snip): "The snipTokensFreed value is plumbed to auto-compact because
	// the main token counter derives context size from the usage field on the most
	// recent assistant message ... snip's savings are therefore invisible to the
	// counter unless passed explicitly."
	ReturnsTokensFreed bool

	// ComposabilityNote is a short note from §4.3 describing why this shaper
	// composes cleanly (or not) with its neighbors.
	ComposabilityNote string

	// PersistsAcrossTurns indicates whether the shaper's effects survive into
	// the next loop iteration without re-running the shaper.
	// Only ShaperContextCollapse persists via its separate collapse store.
	PersistsAcrossTurns bool

	// IsConditionalOnPressure indicates whether the shaper only fires when the
	// context window is near the pressure threshold.
	// Only ShaperAutoCompact is pressure-conditional.
	IsConditionalOnPressure bool

	// UserCanDisable indicates whether the user can toggle off this shaper via
	// configuration. Only ShaperAutoCompact can be disabled by the user.
	UserCanDisable bool
}

// preMCSProfiles is the authoritative §4.3 registry, indexed by shaper ID.
var preMCSProfiles = map[PreModelContextShaperID]PreModelContextShaperProfile{
	ShaperBudgetReduction: {
		ID:             ShaperBudgetReduction,
		PDFSection:     "§4.3",
		ExecutionOrder: 1,
		SourceFunction: "applyToolResultBudget()",
		ActivationMode: ShaperActivationAlways,
		FeatureFlag:    ShaperFlagNone,
		MutationStyle:  ShaperMutationInPlace,
		// Budget reduction replaces oversized outputs with content references;
		// the token count reduction is structural (fewer chars) not an explicit
		// tokensFreed signal — microcompact consumes tool_use_id not content.
		ReturnsTokensFreed:      false,
		ComposabilityNote:       "runs before microcompact because microcompact operates purely by tool_use_id and never inspects content; the two compose cleanly",
		PersistsAcrossTurns:     true,  // content replacements are persisted for resume reconstruction
		IsConditionalOnPressure: false,
		UserCanDisable:          false,
	},
	ShaperSnip: {
		ID:             ShaperSnip,
		PDFSection:     "§4.3",
		ExecutionOrder: 2,
		SourceFunction: "snipCompactIfNeeded()",
		ActivationMode: ShaperActivationFeatureFlag,
		FeatureFlag:    ShaperFlagHistorySnip,
		MutationStyle:  ShaperMutationTrimming,
		// §4.3: snipTokensFreed must be plumbed to auto-compact explicitly because
		// the snipped message's pre-snip input_tokens survive on the most-recent
		// assistant message and the token counter would otherwise double-count.
		ReturnsTokensFreed:      true,
		ComposabilityNote:       "returns tokensFreed that must be passed explicitly to auto-compact (shaper 5) because snip savings are invisible to the main token counter",
		PersistsAcrossTurns:     false,
		IsConditionalOnPressure: false,
		UserCanDisable:          false,
	},
	ShaperMicrocompact: {
		ID:             ShaperMicrocompact,
		PDFSection:     "§4.3",
		ExecutionOrder: 3,
		SourceFunction: "microcompact()",
		ActivationMode: ShaperActivationFeatureFlag,
		FeatureFlag:    ShaperFlagCachedMicrocompact,
		MutationStyle:  ShaperMutationInPlace,
		// microcompact may defer boundary messages until after the API response
		// (pendingCacheEdits) so it can use actual cache_deleted_input_tokens.
		ReturnsTokensFreed:      false,
		ComposabilityNote:       "operates purely by tool_use_id without inspecting content; cache-aware path defers boundary messages via pendingCacheEdits until actual cache token counts are available",
		PersistsAcrossTurns:     false,
		IsConditionalOnPressure: false,
		UserCanDisable:          false,
	},
	ShaperContextCollapse: {
		ID:             ShaperContextCollapse,
		PDFSection:     "§4.3",
		ExecutionOrder: 4,
		SourceFunction: "applyCollapsesIfNeeded()",
		ActivationMode: ShaperActivationFeatureFlag,
		FeatureFlag:    ShaperFlagContextCollapse,
		// §4.3: "Nothing is yielded; the collapsed view is a read-time projection
		// over the REPL's full history." The REPL array is NOT mutated.
		MutationStyle:      ShaperMutationProjection,
		ReturnsTokensFreed: false,
		ComposabilityNote:  "does not mutate the REPL's stored history; replaces messagesForQuery with a projected view so the model sees collapsed version while full history remains for reconstruction",
		// §4.3: "This is what makes collapses persist across turns" — the collapse
		// store is separate from the REPL array.
		PersistsAcrossTurns:     true,
		IsConditionalOnPressure: false,
		UserCanDisable:          false,
	},
	ShaperAutoCompact: {
		ID:             ShaperAutoCompact,
		PDFSection:     "§4.3",
		ExecutionOrder: 5,
		SourceFunction: "compactConversation()",
		// §4.3: "Auto-compact fires only when the context still exceeds the
		// pressure threshold after all four previous shapers have run."
		ActivationMode:          ShaperActivationDefaultOn,
		FeatureFlag:             ShaperFlagNone,
		MutationStyle:           ShaperMutationReplacement,
		ReturnsTokensFreed:      false,
		ComposabilityNote:       "fires only when context still exceeds pressure threshold after all four lighter shapers have run; receives snipTokensFreed from shaper 2 to correct token counter",
		PersistsAcrossTurns:     true, // summary persists as the new conversation base
		IsConditionalOnPressure: true,
		UserCanDisable:          true,
	},
}

// SeedPreModelContextShaperCount is the canonical count of §4.3 shapers.
// Tests MUST assert this value to detect accidental additions or deletions.
const SeedPreModelContextShaperCount = 5

// FindPreModelContextShaper returns the profile for the given shaper ID, and
// whether it was found.
func FindPreModelContextShaper(id PreModelContextShaperID) (PreModelContextShaperProfile, bool) {
	p, ok := preMCSProfiles[id]
	return p, ok
}

// AllPreModelContextShapers returns all five §4.3 shaper profiles sorted by
// ExecutionOrder (1→5), matching the sequential run order in query.ts.
func AllPreModelContextShapers() []PreModelContextShaperProfile {
	out := make([]PreModelContextShaperProfile, 0, len(preMCSProfiles))
	for _, p := range preMCSProfiles {
		out = append(out, p)
	}
	// Sort by ExecutionOrder (insertion-sort on small slice)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].ExecutionOrder < out[j-1].ExecutionOrder; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ShapersByActivationMode returns all shapers with the given activation mode.
func ShapersByActivationMode(mode ShaperActivationMode) []PreModelContextShaperProfile {
	var out []PreModelContextShaperProfile
	for _, p := range AllPreModelContextShapers() {
		if p.ActivationMode == mode {
			out = append(out, p)
		}
	}
	return out
}

// ShapersByFeatureFlag returns all shapers gated by the given feature flag.
// Pass ShaperFlagNone to get shapers that require no flag.
func ShapersByFeatureFlag(flag ShaperFeatureFlag) []PreModelContextShaperProfile {
	var out []PreModelContextShaperProfile
	for _, p := range AllPreModelContextShapers() {
		if p.FeatureFlag == flag {
			out = append(out, p)
		}
	}
	return out
}

// ShapersThatReturnTokensFreed returns shapers that produce an explicit
// tokensFreed signal that must be forwarded to ShaperAutoCompact.
//
// §4.3 specifies that only ShaperSnip returns this signal; the function exists
// so future changes must update the registry rather than scattered call sites.
func ShapersThatReturnTokensFreed() []PreModelContextShaperProfile {
	var out []PreModelContextShaperProfile
	for _, p := range AllPreModelContextShapers() {
		if p.ReturnsTokensFreed {
			out = append(out, p)
		}
	}
	return out
}

// ShapersThatPersistAcrossTurns returns shapers whose effects survive into
// subsequent loop iterations without re-running the shaper.
func ShapersThatPersistAcrossTurns() []PreModelContextShaperProfile {
	var out []PreModelContextShaperProfile
	for _, p := range AllPreModelContextShapers() {
		if p.PersistsAcrossTurns {
			out = append(out, p)
		}
	}
	return out
}

// ShaperPressureConditional returns the shaper(s) that fire only when context
// pressure exceeds the threshold — per §4.3 this is ShaperAutoCompact alone.
func ShaperPressureConditional() []PreModelContextShaperProfile {
	var out []PreModelContextShaperProfile
	for _, p := range AllPreModelContextShapers() {
		if p.IsConditionalOnPressure {
			out = append(out, p)
		}
	}
	return out
}

// EarlierShapersMustRunFirst returns true if shadingA must execute before
// shaperB according to §4.3 sequential execution order.
func EarlierShapersMustRunFirst(a, b PreModelContextShaperID) (bool, bool) {
	pa, okA := FindPreModelContextShaper(a)
	pb, okB := FindPreModelContextShaper(b)
	if !okA || !okB {
		return false, false
	}
	return pa.ExecutionOrder < pb.ExecutionOrder, true
}
