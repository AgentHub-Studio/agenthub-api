package agentic

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// OBS-008 — Generator/evaluator separation.
//
// PDF arXiv:2604.14228v1 Section 11 — "agents tend to confidently praise
// the work, even when quality is mediocre. The generator and the evaluator
// must be separated."
//
// This file defines the post-hoc evaluator surface:
//
//   - RunEvaluator: independent quality assessor invoked AFTER run_complete.
//   - QualityReport: deterministic envelope with score + per-dimension
//     breakdown + actionable findings + decision.
//   - EvaluationDecision: bounded outcome enum (pass / warn / fail).
//   - HeuristicRunEvaluator: deterministic baseline (no LLM call) — runs
//     cheap structural checks so every run gets a quality envelope even
//     when no LLM-based evaluator is configured.
//
// The evaluator MUST NOT see the generator's internal reasoning — only the
// observable RunOutput (final text + tool errors + turn count + cost). This
// is the architectural separation: generator opinions cannot bias the
// evaluator's score.

// EvaluationDecision is the bounded outcome of a quality evaluation.
type EvaluationDecision string

const (
	// EvalPass — quality acceptable; no action required.
	EvalPass EvaluationDecision = "pass"
	// EvalWarn — minor issues found; surface to operator but do not block.
	EvalWarn EvaluationDecision = "warn"
	// EvalFail — quality below threshold; flag for review and possible
	// retry under a stricter prompt.
	EvalFail EvaluationDecision = "fail"
)

// IsTerminalEvaluationDecision returns true for any of the 3 valid
// decisions. Used by tracing layer to validate envelope shape.
func IsTerminalEvaluationDecision(d EvaluationDecision) bool {
	return d == EvalPass || d == EvalWarn || d == EvalFail
}

// QualityFinding is a single actionable observation from the evaluator.
// It cites a dimension, a severity, and a short human-readable reason.
type QualityFinding struct {
	// Dimension names the quality axis evaluated (e.g. "length",
	// "tool_errors", "tool_failure_rate"). Stable strings — observability
	// layer aggregates by dimension.
	Dimension string `json:"dimension"`
	// Severity is one of: info, warn, error. Mirrors EvaluationDecision
	// granularity but applies per-finding.
	Severity string `json:"severity"`
	// Reason is a short human-readable explanation (≤ 200 chars).
	Reason string `json:"reason"`
}

// RunOutput is the observable surface the evaluator inspects. Crucially
// it does NOT include the generator's reasoning trace, system prompt, or
// internal hook decisions — only the user-visible result and aggregate
// metrics. This enforces the separation contract.
type RunOutput struct {
	// FinalText is the user-visible response text.
	FinalText string
	// TurnCount is the number of LLM turns the run consumed.
	TurnCount int
	// ToolCallCount is the total number of tool calls across the run.
	ToolCallCount int
	// ToolErrorCount is the number of tool calls that returned an error.
	ToolErrorCount int
	// TotalTokens is the aggregate token usage.
	TotalTokens int
	// TotalCostUSD is the aggregate cost in USD.
	TotalCostUSD float64
	// DurationMs is the wall-clock duration of the run.
	DurationMs int64
	// HitRetryLimit is true if any retry budget was exhausted.
	HitRetryLimit bool
	// HitBudgetCap is true if the cost or iteration cap was hit.
	HitBudgetCap bool
}

// QualityReport is the structured envelope an evaluator produces.
// Inspired by PDF Section 11 — "deterministic evaluator output feeds
// observability and circuit breakers".
type QualityReport struct {
	// EvaluatorName identifies which evaluator produced this report
	// (e.g. "heuristic", "llm-judge"). Stable string for analytics.
	EvaluatorName string `json:"evaluator"`
	// OverallScore is in [0.0, 1.0]. 1.0 = best.
	OverallScore float64 `json:"overallScore"`
	// Dimensions maps quality axis name → score in [0.0, 1.0].
	Dimensions map[string]float64 `json:"dimensions,omitempty"`
	// Findings lists actionable observations.
	Findings []QualityFinding `json:"findings,omitempty"`
	// Decision is the bounded outcome.
	Decision EvaluationDecision `json:"decision"`
	// EvaluatedAt is the wall-clock timestamp of evaluation.
	EvaluatedAt time.Time `json:"evaluatedAt"`
	// LatencyMs is the time the evaluator itself took.
	LatencyMs int64 `json:"latencyMs"`
}

// RunEvaluator is the post-hoc evaluator interface.
//
// Implementations MUST:
//   - Be deterministic OR clearly mark themselves as non-deterministic.
//   - Run AFTER the generator's run_complete event.
//   - NOT mutate RunOutput (caller passes by value).
//   - Return a fully-populated QualityReport (Decision is required).
//   - Honor ctx cancellation.
type RunEvaluator interface {
	// Evaluate produces a QualityReport for the given RunOutput.
	Evaluate(ctx context.Context, out RunOutput) (QualityReport, error)
}

// --- HeuristicRunEvaluator ---

// HeuristicRunEvaluatorConfig bounds the heuristic evaluator's checks.
type HeuristicRunEvaluatorConfig struct {
	// MinFinalTextLen — responses shorter than this trigger a finding.
	// Default 5. Set to 0 to disable.
	MinFinalTextLen int
	// MaxFinalTextLen — responses longer than this trigger a warn.
	// Default 50000. Set to 0 to disable.
	MaxFinalTextLen int
	// MaxToolFailureRate — tool failure rate above this triggers fail.
	// Default 0.5 (>= 50% failures = quality concern). Range [0.0, 1.0].
	MaxToolFailureRate float64
	// FailDecisionThreshold — overall score < this → EvalFail. Default 0.4.
	FailDecisionThreshold float64
	// WarnDecisionThreshold — overall score < this (and >= fail) → EvalWarn.
	// Default 0.7. Must be > FailDecisionThreshold.
	WarnDecisionThreshold float64
}

// DefaultHeuristicRunEvaluatorConfig returns sensible defaults.
func DefaultHeuristicRunEvaluatorConfig() HeuristicRunEvaluatorConfig {
	return HeuristicRunEvaluatorConfig{
		MinFinalTextLen:       5,
		MaxFinalTextLen:       50000,
		MaxToolFailureRate:    0.5,
		FailDecisionThreshold: 0.4,
		WarnDecisionThreshold: 0.7,
	}
}

// HeuristicRunEvaluator is a deterministic baseline evaluator. It runs
// cheap structural checks: response length bounds, tool failure rate,
// retry/budget exhaustion. Produces a QualityReport without LLM calls.
//
// This is the default platform evaluator — every run gets one even when
// no LLM-judge is configured.
type HeuristicRunEvaluator struct {
	cfg HeuristicRunEvaluatorConfig
}

// NewHeuristicRunEvaluator creates a HeuristicRunEvaluator with the given
// config. Pass DefaultHeuristicRunEvaluatorConfig() for sensible defaults.
func NewHeuristicRunEvaluator(cfg HeuristicRunEvaluatorConfig) *HeuristicRunEvaluator {
	return &HeuristicRunEvaluator{cfg: cfg}
}

// Name returns the stable evaluator identifier.
func (h *HeuristicRunEvaluator) Name() string {
	return "heuristic"
}

// Evaluate inspects the RunOutput and produces a deterministic
// QualityReport. Honors ctx cancellation.
func (h *HeuristicRunEvaluator) Evaluate(ctx context.Context, out RunOutput) (QualityReport, error) {
	if err := ctx.Err(); err != nil {
		return QualityReport{}, fmt.Errorf("evaluator: context cancelled: %w", err)
	}

	start := time.Now()
	report := QualityReport{
		EvaluatorName: h.Name(),
		Dimensions:    map[string]float64{},
		Findings:      []QualityFinding{},
	}

	// Dimension: length. 1.0 = within bounds.
	lengthScore := 1.0
	textLen := len(strings.TrimSpace(out.FinalText))
	if h.cfg.MinFinalTextLen > 0 && textLen < h.cfg.MinFinalTextLen {
		lengthScore = 0.0
		report.Findings = append(report.Findings, QualityFinding{
			Dimension: "length",
			Severity:  "error",
			Reason:    fmt.Sprintf("final response too short (%d < %d chars)", textLen, h.cfg.MinFinalTextLen),
		})
	} else if h.cfg.MaxFinalTextLen > 0 && textLen > h.cfg.MaxFinalTextLen {
		lengthScore = 0.5
		report.Findings = append(report.Findings, QualityFinding{
			Dimension: "length",
			Severity:  "warn",
			Reason:    fmt.Sprintf("final response very long (%d > %d chars)", textLen, h.cfg.MaxFinalTextLen),
		})
	}
	report.Dimensions["length"] = lengthScore

	// Dimension: tool_failure_rate. 1.0 = no errors.
	failureRateScore := 1.0
	if out.ToolCallCount > 0 {
		rate := float64(out.ToolErrorCount) / float64(out.ToolCallCount)
		failureRateScore = 1.0 - rate
		if rate >= h.cfg.MaxToolFailureRate {
			report.Findings = append(report.Findings, QualityFinding{
				Dimension: "tool_failure_rate",
				Severity:  "error",
				Reason:    fmt.Sprintf("tool failure rate %.0f%% >= threshold %.0f%%", rate*100, h.cfg.MaxToolFailureRate*100),
			})
		} else if rate > 0 {
			report.Findings = append(report.Findings, QualityFinding{
				Dimension: "tool_failure_rate",
				Severity:  "warn",
				Reason:    fmt.Sprintf("%d of %d tool calls failed", out.ToolErrorCount, out.ToolCallCount),
			})
		}
	}
	report.Dimensions["tool_failure_rate"] = failureRateScore

	// Dimension: budget. 1.0 = no caps hit.
	budgetScore := 1.0
	if out.HitRetryLimit {
		budgetScore = 0.4
		report.Findings = append(report.Findings, QualityFinding{
			Dimension: "budget",
			Severity:  "warn",
			Reason:    "retry limit reached during run",
		})
	}
	if out.HitBudgetCap {
		budgetScore = 0.3
		report.Findings = append(report.Findings, QualityFinding{
			Dimension: "budget",
			Severity:  "error",
			Reason:    "iteration or cost cap hit — response may be truncated",
		})
	}
	report.Dimensions["budget"] = budgetScore

	// Overall score = unweighted mean of dimensions. Keep the addition order
	// explicit: Go deliberately randomizes map iteration, which would otherwise
	// cause tiny floating-point differences for identical evaluations.
	report.OverallScore = (lengthScore + failureRateScore + budgetScore) / 3

	// Decision from thresholds.
	switch {
	case report.OverallScore < h.cfg.FailDecisionThreshold:
		report.Decision = EvalFail
	case report.OverallScore < h.cfg.WarnDecisionThreshold:
		report.Decision = EvalWarn
	default:
		report.Decision = EvalPass
	}

	report.EvaluatedAt = time.Now()
	report.LatencyMs = time.Since(start).Milliseconds()
	return report, nil
}

// EvaluationData is the SSE event payload emitted when an evaluator runs.
// Wire format is the JSON of this struct.
type EvaluationData struct {
	RunID  string        `json:"runId"`
	Report QualityReport `json:"report"`
}
