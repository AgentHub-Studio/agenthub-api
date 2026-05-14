package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CTX-012 — Context collapse / read-time projection.
//
// PDF arXiv:2604.14228v1 §7.11 (raw transcript is preserved verbatim;
// collapsed view is COMPUTED AT READ TIME by applying boundary +
// reference + budget rules, NOT at write time — so the same transcript
// can render differently for different consumers/contexts).
//
// Distinct from existing AgentHub plumbing:
//   - CTX-001 ContextAssembler = section-ordered envelope (build time).
//   - CTX-008 ToolResultBudget = per-call enforcement (write time).
//   - CTX-010 ContextManager = compaction lifecycle (write-time, mutates).
//   - CTX-011 CompactBoundary = projection helper (read-time, single boundary).
//   - context_collapser.go (this file) = COMPOSED READ-TIME PROJECTION:
//     given (raw_messages, boundaries, ref_expansions, budget), returns
//     a CollapsedView ready to render. Raw transcript is never mutated;
//     different consumers can project differently from the same source.
//
// This is the "pure function" view of the transcript: same input →
// same output → replay-deterministic.

// CollapseStrategy bounded enum identifies how aggressively to collapse.
type CollapseStrategy string

const (
	// CollapseStrategyMinimal — keep all messages, only apply boundary cut.
	CollapseStrategyMinimal CollapseStrategy = "minimal"
	// CollapseStrategyBalanced — boundary cut + budget enforcement on
	// tool results + content reference expansion for hot paths.
	CollapseStrategyBalanced CollapseStrategy = "balanced"
	// CollapseStrategyAggressive — drop intermediate tool calls without
	// errors, keep only assistant turns + errors + summaries.
	CollapseStrategyAggressive CollapseStrategy = "aggressive"
)

var allCollapseStrategies = []CollapseStrategy{
	CollapseStrategyMinimal, CollapseStrategyBalanced, CollapseStrategyAggressive,
}

// IsValidCollapseStrategy returns true for the bounded set.
func IsValidCollapseStrategy(s CollapseStrategy) bool {
	for _, v := range allCollapseStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// AllCollapseStrategies returns a copy.
func AllCollapseStrategies() []CollapseStrategy {
	out := make([]CollapseStrategy, len(allCollapseStrategies))
	copy(out, allCollapseStrategies)
	return out
}

// TranscriptMessage is the minimal shape the collapser consumes.
// Decoupled from chat.ChatMessage so the collapser stays domain-pure.
type TranscriptMessage struct {
	ID         uuid.UUID `json:"id"`
	Role       string    `json:"role"` // user/assistant/system
	Kind       string    `json:"kind"` // text/tool_use/tool_result/compact_summary
	Content    string    `json:"content"`
	IsError    bool      `json:"isError,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CollapseInput bundles everything the collapser needs.
type CollapseInput struct {
	Messages       []TranscriptMessage `json:"messages"`
	Strategy       CollapseStrategy    `json:"strategy"`
	LatestBoundary *CompactBoundary    `json:"latestBoundary,omitempty"`
}

// CollapsedView is the read-time projection result.
type CollapsedView struct {
	Messages         []TranscriptMessage `json:"messages"`
	OriginalCount    int                 `json:"originalCount"`
	DroppedCount     int                 `json:"droppedCount"`
	Strategy         CollapseStrategy    `json:"strategy"`
	ProjectedAt      time.Time           `json:"projectedAt"`
	// DroppedReasons aggregates per-message reasons (for audit/debug).
	DroppedReasons map[string]int `json:"droppedReasons,omitempty"`
}

// Sentinels.
var (
	ErrCollapserInvalidStrategy = errors.New("context collapser: invalid strategy")
	ErrCollapserNilInput        = errors.New("context collapser: input messages required")
)

// ContextCollapser is a stateless function-like collector.
// All Project calls are independent → safe to share across goroutines.
type ContextCollapser struct {
	now func() time.Time
	mu  sync.Mutex
}

// NewContextCollapser returns a new collapser.
func NewContextCollapser() *ContextCollapser {
	return &ContextCollapser{now: time.Now}
}

// SetClock allows tests to inject a deterministic clock.
func (c *ContextCollapser) SetClock(clock func() time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = clock
}

// Project applies the strategy to messages and returns the collapsed view.
// Pure function: same input → same output.
func (c *ContextCollapser) Project(ctx context.Context, in CollapseInput) (CollapsedView, error) {
	if err := ctx.Err(); err != nil {
		return CollapsedView{}, err
	}
	if in.Messages == nil {
		return CollapsedView{}, ErrCollapserNilInput
	}
	if !IsValidCollapseStrategy(in.Strategy) {
		return CollapsedView{}, fmt.Errorf("%w: %q", ErrCollapserInvalidStrategy, in.Strategy)
	}

	c.mu.Lock()
	now := c.now()
	c.mu.Unlock()

	view := CollapsedView{
		OriginalCount:  len(in.Messages),
		Strategy:       in.Strategy,
		ProjectedAt:    now,
		DroppedReasons: map[string]int{},
	}

	working := append([]TranscriptMessage{}, in.Messages...)

	// Step 1: boundary cut (drop pre-anchor messages).
	if in.LatestBoundary != nil {
		anchorIdx := -1
		for i, m := range working {
			if m.ID == in.LatestBoundary.AnchorUUID {
				anchorIdx = i
				break
			}
		}
		if anchorIdx > 0 {
			view.DroppedReasons["pre_boundary_anchor"] = anchorIdx
			working = working[anchorIdx:]
		}
	}

	// Step 2: per-strategy filtering.
	filtered := []TranscriptMessage{}
	for _, m := range working {
		keep, reason := keepForStrategy(in.Strategy, m)
		if !keep {
			view.DroppedReasons[reason]++
			continue
		}
		filtered = append(filtered, m)
	}

	view.Messages = filtered
	view.DroppedCount = view.OriginalCount - len(filtered)
	return view, nil
}

// keepForStrategy returns whether to keep a message + drop reason if not.
func keepForStrategy(strategy CollapseStrategy, m TranscriptMessage) (bool, string) {
	switch strategy {
	case CollapseStrategyMinimal:
		// Keep everything.
		return true, ""

	case CollapseStrategyBalanced:
		// Drop intermediate tool_use whose paired tool_result was successful;
		// since this is single-message scoped, we approximate: drop tool_use
		// (the LLM call), keep tool_result + errors + summaries + text.
		// Note: balanced is a compromise; aggressive drops more.
		if m.Kind == "tool_use" && !m.IsError {
			return false, "tool_use_redacted"
		}
		return true, ""

	case CollapseStrategyAggressive:
		// Keep only assistant text + errors + compact_summary.
		// Drop user-channel and tool_use/tool_result unless error.
		if m.IsError {
			return true, ""
		}
		if m.Kind == "compact_summary" {
			return true, ""
		}
		if m.Role == "assistant" && m.Kind == "text" {
			return true, ""
		}
		return false, fmt.Sprintf("aggressive_drop_%s_%s", m.Role, m.Kind)
	}
	return true, ""
}

// Render returns a flat string of the collapsed view, useful for
// prompt assembly + replay debugging.
func (v CollapsedView) Render() string {
	var b strings.Builder
	for i, m := range v.Messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("[")
		b.WriteString(m.Role)
		b.WriteString("/")
		b.WriteString(m.Kind)
		if m.IsError {
			b.WriteString("/error")
		}
		b.WriteString("]\n")
		b.WriteString(m.Content)
	}
	return b.String()
}

// MessageCount returns the live message count after projection.
func (v CollapsedView) MessageCount() int {
	return len(v.Messages)
}

// Compression returns the ratio (DroppedCount / OriginalCount), 0 if empty.
func (v CollapsedView) Compression() float64 {
	if v.OriginalCount == 0 {
		return 0
	}
	return float64(v.DroppedCount) / float64(v.OriginalCount)
}

// ProjectionTrace assembles audit details suitable for GOV-001.
type ProjectionTrace struct {
	OriginalCount  int            `json:"originalCount"`
	FinalCount     int            `json:"finalCount"`
	Compression    float64        `json:"compression"`
	Strategy       CollapseStrategy `json:"strategy"`
	DroppedReasons map[string]int `json:"droppedReasons"`
}

// Trace returns the audit-friendly summary.
func (v CollapsedView) Trace() ProjectionTrace {
	return ProjectionTrace{
		OriginalCount:  v.OriginalCount,
		FinalCount:     len(v.Messages),
		Compression:    v.Compression(),
		Strategy:       v.Strategy,
		DroppedReasons: v.DroppedReasons,
	}
}

// SortedDroppedReasons returns reasons sorted by count desc → key asc
// for stable rendering.
func (v CollapsedView) SortedDroppedReasons() []struct {
	Reason string
	Count  int
} {
	out := []struct {
		Reason string
		Count  int
	}{}
	for r, c := range v.DroppedReasons {
		out = append(out, struct {
			Reason string
			Count  int
		}{r, c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}
