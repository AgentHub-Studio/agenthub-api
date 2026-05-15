package agentic

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// CTX-008 — Tool result budget.
//
// PDF arXiv:2604.14228v1 §7.6 (tool results are typically the largest
// single source of context bloat — a 100k-row SQL query, a long HTTP
// response, a verbose stack trace can each consume the entire window).
//
// Distinct from existing tool-result plumbing:
//   - toolresultstorage.go = ContentReplacementState — REMEMBERS a
//     replacement decision across turns for cache stability.
//   - tool_result_budget.go (this file) = DECIDES whether to keep /
//     truncate / summarize / drop the result based on per-call,
//     per-turn, and per-run cumulative budgets.
//
// The enforcer is stateful per RUN: it tracks cumulative tokens used
// across multiple tool calls so a single huge result doesn't sneak
// through "under per-call cap" only to push the run over its total budget.

// ToolResultBudgetAction bounded enum for what the enforcer decided.
type ToolResultBudgetAction string

const (
	// ToolResultBudgetKeep — result fits all budgets; emit verbatim.
	ToolResultBudgetKeep ToolResultBudgetAction = "keep"
	// ToolResultBudgetTruncate — result was clipped to per-call cap.
	ToolResultBudgetTruncate ToolResultBudgetAction = "truncate"
	// ToolResultBudgetSummarize — result replaced with summary placeholder
	// (caller asked for a summarized form — typically followed by an
	// actual summarization step).
	ToolResultBudgetSummarize ToolResultBudgetAction = "summarize"
	// ToolResultBudgetDrop — result fully replaced with a one-line note.
	// Happens when the result is so big that even truncation wouldn't
	// fit the remaining run budget.
	ToolResultBudgetDrop ToolResultBudgetAction = "drop"
)

var allToolResultBudgetActions = []ToolResultBudgetAction{
	ToolResultBudgetKeep, ToolResultBudgetTruncate,
	ToolResultBudgetSummarize, ToolResultBudgetDrop,
}

// IsValidToolResultBudgetAction returns true for the bounded set.
func IsValidToolResultBudgetAction(a ToolResultBudgetAction) bool {
	for _, v := range allToolResultBudgetActions {
		if a == v {
			return true
		}
	}
	return false
}

// AllToolResultBudgetActions returns a copy.
func AllToolResultBudgetActions() []ToolResultBudgetAction {
	out := make([]ToolResultBudgetAction, len(allToolResultBudgetActions))
	copy(out, allToolResultBudgetActions)
	return out
}

// ToolResultBudgetConfig tunes the enforcer.
type ToolResultBudgetConfig struct {
	// PerCallMaxTokens is the max tokens any single tool result may
	// occupy. Results above are truncated. 0 = unlimited.
	PerCallMaxTokens int
	// PerTurnMaxTokens caps cumulative tool-result tokens within ONE
	// turn (multiple tool calls within the same agent turn). 0 = unlimited.
	PerTurnMaxTokens int
	// PerRunMaxTokens caps cumulative tool-result tokens across the
	// entire run (all turns). 0 = unlimited.
	PerRunMaxTokens int
	// ForceSummarizeOverTokens, if >0, triggers ToolResultBudgetSummarize
	// (vs Truncate) when the original size exceeds this threshold —
	// summarization is more useful than blunt truncation for very large
	// results.
	ForceSummarizeOverTokens int
}

// DefaultToolResultBudgetConfig returns sensible defaults.
func DefaultToolResultBudgetConfig() ToolResultBudgetConfig {
	return ToolResultBudgetConfig{
		PerCallMaxTokens:         4000,
		PerTurnMaxTokens:         12000,
		PerRunMaxTokens:          50000,
		ForceSummarizeOverTokens: 8000,
	}
}

// ToolResultBudgetDecision is the enforcer's per-call output.
type ToolResultBudgetDecision struct {
	Action          ToolResultBudgetAction `json:"action"`
	Reason          string                 `json:"reason"`
	// OriginalTokens is the estimated tokens BEFORE enforcement.
	OriginalTokens  int                    `json:"originalTokens"`
	// FinalTokens is the estimated tokens AFTER enforcement (what the
	// caller will actually emit downstream).
	FinalTokens     int                    `json:"finalTokens"`
	// FinalBody is the result body the caller should emit (truncated /
	// summary placeholder / drop note / verbatim).
	FinalBody       string                 `json:"finalBody"`
	// PerTurnUsedAfter is cumulative per-turn tokens AFTER this decision.
	PerTurnUsedAfter int                   `json:"perTurnUsedAfter"`
	// PerRunUsedAfter is cumulative per-run tokens AFTER this decision.
	PerRunUsedAfter  int                   `json:"perRunUsedAfter"`
}

// Sentinels.
var (
	ErrToolResultBudgetEmptyBody = errors.New("tool result budget: result body required")
)

// ToolResultBudgetEnforcer is stateful — tracks cumulative use per turn
// and per run. Concurrent-safe.
type ToolResultBudgetEnforcer struct {
	mu           sync.Mutex
	config       ToolResultBudgetConfig
	perTurnUsed  int
	perRunUsed   int
}

// NewToolResultBudgetEnforcer returns a fresh enforcer with the given config.
func NewToolResultBudgetEnforcer(cfg ToolResultBudgetConfig) *ToolResultBudgetEnforcer {
	return &ToolResultBudgetEnforcer{config: cfg}
}

// EndTurn resets the per-turn counter (caller invokes when an agent
// turn ends — keeps cumulative per-turn budget local to a single turn).
func (e *ToolResultBudgetEnforcer) EndTurn() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.perTurnUsed = 0
}

// EndRun resets BOTH per-turn and per-run counters.
func (e *ToolResultBudgetEnforcer) EndRun() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.perTurnUsed = 0
	e.perRunUsed = 0
}

// PerTurnUsed returns current per-turn cumulative tokens.
func (e *ToolResultBudgetEnforcer) PerTurnUsed() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.perTurnUsed
}

// PerRunUsed returns current per-run cumulative tokens.
func (e *ToolResultBudgetEnforcer) PerRunUsed() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.perRunUsed
}

// truncateBody clips body to roughly maxTokens worth (chars/4 heuristic).
func truncateBody(body string, maxTokens int) string {
	if maxTokens <= 0 {
		return body
	}
	maxChars := maxTokens * 4
	if len(body) <= maxChars {
		return body
	}
	if maxChars < 4 {
		return body[:maxChars]
	}
	return body[:maxChars-3] + "..."
}

// Enforce decides per-call. Returns the action + final body + reason.
//
// Algorithm:
//  1. Validate non-empty body.
//  2. Compute original tokens.
//  3. If ForceSummarizeOverTokens triggers, return Summarize placeholder.
//  4. Else if PerCallMaxTokens triggers, truncate.
//  5. Then compute would-be cumulative (per-turn, per-run).
//  6. If even truncated body would push run over PerRunMaxTokens,
//     drop the result entirely and return a one-line note.
//  7. Else accept and update counters.
func (e *ToolResultBudgetEnforcer) Enforce(toolName, body string) (ToolResultBudgetDecision, error) {
	if body == "" {
		return ToolResultBudgetDecision{}, ErrToolResultBudgetEmptyBody
	}
	original := estimateTokens(body)

	e.mu.Lock()
	defer e.mu.Unlock()

	cfg := e.config

	// Default decision: keep verbatim.
	decision := ToolResultBudgetDecision{
		Action:         ToolResultBudgetKeep,
		Reason:         "fits all budgets",
		OriginalTokens: original,
		FinalTokens:    original,
		FinalBody:      body,
	}

	// Step 3: force summarize when very large.
	if cfg.ForceSummarizeOverTokens > 0 && original > cfg.ForceSummarizeOverTokens {
		summary := fmt.Sprintf(
			"[summarized: original tool=%q result was %d tokens; "+
				"replaced with placeholder per ForceSummarizeOverTokens=%d]",
			toolName, original, cfg.ForceSummarizeOverTokens)
		decision.Action = ToolResultBudgetSummarize
		decision.Reason = fmt.Sprintf("original %d > ForceSummarizeOverTokens %d",
			original, cfg.ForceSummarizeOverTokens)
		decision.FinalBody = summary
		decision.FinalTokens = estimateTokens(summary)
	} else if cfg.PerCallMaxTokens > 0 && original > cfg.PerCallMaxTokens {
		// Step 4: truncate to per-call cap.
		clipped := truncateBody(body, cfg.PerCallMaxTokens)
		decision.Action = ToolResultBudgetTruncate
		decision.Reason = fmt.Sprintf("original %d > PerCallMaxTokens %d",
			original, cfg.PerCallMaxTokens)
		decision.FinalBody = clipped
		decision.FinalTokens = estimateTokens(clipped)
	}

	// Step 5-6: would-be cumulative checks. If FinalTokens would push
	// per-run over the limit, DROP entirely.
	prospectiveRun := e.perRunUsed + decision.FinalTokens
	if cfg.PerRunMaxTokens > 0 && prospectiveRun > cfg.PerRunMaxTokens {
		dropNote := fmt.Sprintf(
			"[dropped: tool=%q result of %d tokens would push run cumulative to %d, "+
				"over PerRunMaxTokens=%d]",
			toolName, decision.FinalTokens, prospectiveRun, cfg.PerRunMaxTokens)
		decision.Action = ToolResultBudgetDrop
		decision.Reason = fmt.Sprintf("prospective run cumulative %d > PerRunMaxTokens %d",
			prospectiveRun, cfg.PerRunMaxTokens)
		decision.FinalBody = dropNote
		decision.FinalTokens = estimateTokens(dropNote)
	}

	// Likewise, per-turn cumulative.
	prospectiveTurn := e.perTurnUsed + decision.FinalTokens
	if cfg.PerTurnMaxTokens > 0 && prospectiveTurn > cfg.PerTurnMaxTokens && decision.Action != ToolResultBudgetDrop {
		dropNote := fmt.Sprintf(
			"[dropped: tool=%q result of %d tokens would push turn cumulative to %d, "+
				"over PerTurnMaxTokens=%d]",
			toolName, decision.FinalTokens, prospectiveTurn, cfg.PerTurnMaxTokens)
		decision.Action = ToolResultBudgetDrop
		decision.Reason = fmt.Sprintf("prospective turn cumulative %d > PerTurnMaxTokens %d",
			prospectiveTurn, cfg.PerTurnMaxTokens)
		decision.FinalBody = dropNote
		decision.FinalTokens = estimateTokens(dropNote)
	}

	// Step 7: commit the actual final tokens to counters.
	e.perTurnUsed += decision.FinalTokens
	e.perRunUsed += decision.FinalTokens
	decision.PerTurnUsedAfter = e.perTurnUsed
	decision.PerRunUsedAfter = e.perRunUsed

	return decision, nil
}

// EnforceMany is a convenience wrapper for batches.
func (e *ToolResultBudgetEnforcer) EnforceMany(items []struct {
	ToolName string
	Body     string
}) ([]ToolResultBudgetDecision, error) {
	out := make([]ToolResultBudgetDecision, 0, len(items))
	for _, it := range items {
		d, err := e.Enforce(it.ToolName, it.Body)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", it.ToolName, err)
		}
		out = append(out, d)
	}
	return out, nil
}

// ConfigSummary returns a short string describing the active config —
// useful for audit logs.
func (e *ToolResultBudgetEnforcer) ConfigSummary() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return fmt.Sprintf(
		"per_call=%d per_turn=%d per_run=%d force_summarize_over=%d",
		e.config.PerCallMaxTokens,
		e.config.PerTurnMaxTokens,
		e.config.PerRunMaxTokens,
		e.config.ForceSummarizeOverTokens)
}

// IsBudgetReason returns true when the reason text indicates a budget-
// triggered decision (vs the default "fits all budgets" baseline).
func (d ToolResultBudgetDecision) IsBudgetReason() bool {
	return strings.Contains(d.Reason, "PerCallMaxTokens") ||
		strings.Contains(d.Reason, "PerTurnMaxTokens") ||
		strings.Contains(d.Reason, "PerRunMaxTokens") ||
		strings.Contains(d.Reason, "ForceSummarizeOverTokens")
}
