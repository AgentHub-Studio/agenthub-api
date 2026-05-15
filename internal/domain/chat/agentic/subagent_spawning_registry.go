package agentic

// SubagentSpawningRegistry models the four-phase subagent lifecycle described
// in arXiv:2604.14228v1 §8 ("Subagents").
//
// §8 establishes that when the parent agent calls AgentTool (the "agent"
// builtin), a child agent process is spawned, executes its own queryLoop(),
// returns only a structured summary to the parent (never the full transcript),
// and then has its context discarded to reclaim memory:
//
//	spawned → running → summary_returned → context_discarded
//
// The lifecycle is deliberately linear: each transition has a defined
// TransitionRank (1 → 4), exactly one initial state (spawned), and exactly
// one terminal state (context_discarded). The intermediate state
// summary_returned is the pivot point where the parent receives the bounded
// result and the child's full context is still alive — one tick later the
// context is freed.
//
// Relationships to neighbouring constructs:
//   - subagent_summary_return.go (SUB-010) models the *shape* of the summary
//     payload that moves from summary_returned back to the parent; this
//     registry models the *lifecycle states* independent of payload shape.
//   - background_subagents.go (SUB-008) uses its own BackgroundSubagentState
//     (queued/running/succeeded/…) for fire-and-forget async subagents; this
//     registry covers the synchronous (blocking) spawn path in §8.
//   - subagent_isolation.go (§8.2) describes the three isolation modes
//     (in_process, remote, worktree) that govern WHERE the spawned subagent
//     runs; the spawning lifecycle applies to all modes equally.

// SubagentLifecycleState is the type-safe identifier for a spawning lifecycle
// phase from arXiv:2604.14228v1 §8.
type SubagentLifecycleState string

const (
	// SubagentStateSpawned is the initial state. The parent has called AgentTool
	// and the child agent process has been created but has not yet issued any
	// LLM call or tool invocation of its own.
	SubagentStateSpawned SubagentLifecycleState = "spawned"

	// SubagentStateRunning is the active execution state. The child agent's
	// queryLoop() is live: it is issuing LLM calls, invoking tools, and
	// accumulating its own context window and sidechain transcript.
	SubagentStateRunning SubagentLifecycleState = "running"

	// SubagentStateSummaryReturned is the pivot state. The child agent has
	// finished its loop (either naturally or via a stop condition), produced a
	// structured SubagentReturnSummary, and handed it back to the parent.
	// The child's full context is still resident in memory at this point;
	// it has not yet been freed.
	SubagentStateSummaryReturned SubagentLifecycleState = "summary_returned"

	// SubagentStateContextDiscarded is the terminal state. The child's in-memory
	// context window, intermediate state, and any cached model activations have
	// been released. Only the sidechain transcript (sidechain JSONL + meta.json)
	// and the summary payload remain durable — exactly as §8.3 requires so the
	// parent's context budget stays bounded.
	SubagentStateContextDiscarded SubagentLifecycleState = "context_discarded"
)

// SubagentSpawnProfile is the immutable §8 metadata for one lifecycle state.
type SubagentSpawnProfile struct {
	// StateID is the canonical identifier for this lifecycle phase.
	StateID SubagentLifecycleState

	// TransitionRank is the 1-based position in the linear spawning lifecycle.
	// Rank 1 = spawned (first) → Rank 4 = context_discarded (last).
	TransitionRank int

	// Label is the short human-readable name for this state.
	Label string

	// Description explains the semantic meaning and invariants for this phase.
	Description string

	// PDFSection cites the paper section that defines this state.
	PDFSection string

	// IsTerminal is true when no further state transitions are possible.
	// Exactly one state is terminal: context_discarded.
	IsTerminal bool

	// IsContextRetained is true when the child's in-memory context window is
	// still resident (has not yet been released). True for spawned, running,
	// and summary_returned; false for context_discarded.
	IsContextRetained bool

	// SummaryAvailable is true when the structured SubagentReturnSummary has
	// been produced and can be consumed by the parent. True only for
	// summary_returned and context_discarded.
	SummaryAvailable bool

	// ParentNotified is true when the parent's queryLoop() has received the
	// summary and can continue its own execution. True only for
	// summary_returned and context_discarded.
	ParentNotified bool
}

// subagentSpawnProfiles is the package-level seed for all four lifecycle states.
var subagentSpawnProfiles = []*SubagentSpawnProfile{
	{
		StateID:           SubagentStateSpawned,
		TransitionRank:    1,
		Label:             "Spawned",
		Description:       "The parent agent has invoked AgentTool and the child process has been created. No LLM calls or tool invocations have occurred yet. The child's context window is allocated but empty.",
		PDFSection:        "8",
		IsTerminal:        false,
		IsContextRetained: true,
		SummaryAvailable:  false,
		ParentNotified:    false,
	},
	{
		StateID:           SubagentStateRunning,
		TransitionRank:    2,
		Label:             "Running",
		Description:       "The child agent's queryLoop() is active. It is issuing LLM calls, invoking tools, accumulating its own context window, and writing its sidechain transcript. The parent blocks until this phase completes.",
		PDFSection:        "8",
		IsTerminal:        false,
		IsContextRetained: true,
		SummaryAvailable:  false,
		ParentNotified:    false,
	},
	{
		StateID:           SubagentStateSummaryReturned,
		TransitionRank:    3,
		Label:             "Summary Returned",
		Description:       "The child agent has completed its loop and produced a structured SubagentReturnSummary. The summary has been handed back to the parent queryLoop(). The child's full in-memory context is still resident — it has not yet been released.",
		PDFSection:        "8",
		IsTerminal:        false,
		IsContextRetained: true,
		SummaryAvailable:  true,
		ParentNotified:    true,
	},
	{
		StateID:           SubagentStateContextDiscarded,
		TransitionRank:    4,
		Label:             "Context Discarded",
		Description:       "The child's in-memory context window, intermediate state, and any cached model activations have been released. The durable sidechain transcript (JSONL + meta.json) and the summary payload remain available for audit. The parent's context contains only the bounded summary — never the full transcript.",
		PDFSection:        "8",
		IsTerminal:        true,
		IsContextRetained: false,
		SummaryAvailable:  true,
		ParentNotified:    true,
	},
}

// SeedSubagentSpawnStateCount is the number of lifecycle states in §8.
const SeedSubagentSpawnStateCount = 4

// subagentSpawnByID is the fast-lookup index built at init time.
var subagentSpawnByID map[SubagentLifecycleState]*SubagentSpawnProfile

func init() {
	subagentSpawnByID = make(map[SubagentLifecycleState]*SubagentSpawnProfile, SeedSubagentSpawnStateCount)
	for _, p := range subagentSpawnProfiles {
		subagentSpawnByID[p.StateID] = p
	}
}

// SubagentSpawningRegistry provides §8 queries over the four subagent
// lifecycle states from arXiv:2604.14228v1.
//
// The registry is read-only; callers must not modify the returned profiles.
type SubagentSpawningRegistry struct{}

// NewSubagentSpawningRegistry returns a ready-to-use registry.
func NewSubagentSpawningRegistry() *SubagentSpawningRegistry {
	return &SubagentSpawningRegistry{}
}

// FindStateByID looks up a lifecycle state profile by its ID string.
// Returns (profile, true) if found; (nil, false) if unknown.
func (r *SubagentSpawningRegistry) FindStateByID(id SubagentLifecycleState) (*SubagentSpawnProfile, bool) {
	p, ok := subagentSpawnByID[id]
	return p, ok
}

// AllStates returns all four lifecycle state profiles in transition-rank order
// (spawned first → context_discarded last). Returns a defensive copy of the slice.
func (r *SubagentSpawningRegistry) AllStates() []*SubagentSpawnProfile {
	result := make([]*SubagentSpawnProfile, len(subagentSpawnProfiles))
	copy(result, subagentSpawnProfiles)
	return result
}

// Count returns the number of registered lifecycle states.
func (r *SubagentSpawningRegistry) Count() int {
	return len(subagentSpawnProfiles)
}

// IsValidStateID returns true if the given string is one of the four
// canonical §8 lifecycle state identifiers.
func (r *SubagentSpawningRegistry) IsValidStateID(id SubagentLifecycleState) bool {
	_, ok := subagentSpawnByID[id]
	return ok
}

// TerminalStates returns all states where IsTerminal == true.
// §8 has exactly one terminal state: context_discarded.
func (r *SubagentSpawningRegistry) TerminalStates() []*SubagentSpawnProfile {
	var result []*SubagentSpawnProfile
	for _, p := range subagentSpawnProfiles {
		if p.IsTerminal {
			result = append(result, p)
		}
	}
	return result
}

// ActiveStates returns all states where IsTerminal == false.
// §8: spawned, running, summary_returned.
func (r *SubagentSpawningRegistry) ActiveStates() []*SubagentSpawnProfile {
	var result []*SubagentSpawnProfile
	for _, p := range subagentSpawnProfiles {
		if !p.IsTerminal {
			result = append(result, p)
		}
	}
	return result
}

// StatesWithSummaryAvailable returns states where the structured
// SubagentReturnSummary has been produced. §8: summary_returned and
// context_discarded.
func (r *SubagentSpawningRegistry) StatesWithSummaryAvailable() []*SubagentSpawnProfile {
	var result []*SubagentSpawnProfile
	for _, p := range subagentSpawnProfiles {
		if p.SummaryAvailable {
			result = append(result, p)
		}
	}
	return result
}

// StatesWhereContextRetained returns states where the child's in-memory
// context window is still resident. §8: spawned, running, summary_returned.
func (r *SubagentSpawningRegistry) StatesWhereContextRetained() []*SubagentSpawnProfile {
	var result []*SubagentSpawnProfile
	for _, p := range subagentSpawnProfiles {
		if p.IsContextRetained {
			result = append(result, p)
		}
	}
	return result
}

// StatesWhereParentNotified returns states where the parent queryLoop() has
// received the summary. §8: summary_returned and context_discarded.
func (r *SubagentSpawningRegistry) StatesWhereParentNotified() []*SubagentSpawnProfile {
	var result []*SubagentSpawnProfile
	for _, p := range subagentSpawnProfiles {
		if p.ParentNotified {
			result = append(result, p)
		}
	}
	return result
}

// StateByTransitionRank returns the state at the given 1-based transition rank.
// Returns (nil, false) if rank is out of range [1, SeedSubagentSpawnStateCount].
func (r *SubagentSpawningRegistry) StateByTransitionRank(rank int) (*SubagentSpawnProfile, bool) {
	for _, p := range subagentSpawnProfiles {
		if p.TransitionRank == rank {
			return p, true
		}
	}
	return nil, false
}

// InitialState returns the state with TransitionRank == 1 (spawned).
func (r *SubagentSpawningRegistry) InitialState() *SubagentSpawnProfile {
	p, _ := r.StateByTransitionRank(1)
	return p
}

// TerminalStateAfterSummary returns the state that follows summary_returned —
// i.e., context_discarded. This is the state where the child's memory is freed
// and only the durable sidechain transcript remains.
func (r *SubagentSpawningRegistry) TerminalStateAfterSummary() *SubagentSpawnProfile {
	p, _ := r.StateByTransitionRank(SeedSubagentSpawnStateCount)
	return p
}

// --- Structural invariants ---

// ExactlyOneInitialState validates that exactly one state has TransitionRank == 1.
// §8: the lifecycle always begins at spawned.
func ExactlyOneInitialState() bool {
	count := 0
	for _, p := range subagentSpawnProfiles {
		if p.TransitionRank == 1 {
			count++
		}
	}
	return count == 1
}

// TerminalStatesHaveNoContext validates that every terminal state has
// IsContextRetained == false. §8: once the context is discarded the
// lifecycle ends — no terminal state can still hold context.
func TerminalStatesHaveNoContext() bool {
	for _, p := range subagentSpawnProfiles {
		if p.IsTerminal && p.IsContextRetained {
			return false
		}
	}
	return true
}

// SummaryReturnedPrecedesContextDiscarded validates that the
// summary_returned state has a strictly lower TransitionRank than
// context_discarded. §8: the parent must receive the summary before
// the child's context is freed.
func SummaryReturnedPrecedesContextDiscarded() bool {
	sr, okSR := subagentSpawnByID[SubagentStateSummaryReturned]
	cd, okCD := subagentSpawnByID[SubagentStateContextDiscarded]
	if !okSR || !okCD {
		return false
	}
	return sr.TransitionRank < cd.TransitionRank
}
