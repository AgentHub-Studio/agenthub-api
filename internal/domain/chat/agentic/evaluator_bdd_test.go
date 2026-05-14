package agentic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// OBS-008 — Generator/evaluator separation BDD.
//
// PDF arXiv:2604.14228v1 Section 11: "agents tend to confidently praise
// the work, even when quality is mediocre. The generator and the evaluator
// must be separated."
//
// These scenarios validate the separation contract:
//
//  1. Evaluator sees ONLY observable RunOutput, not generator reasoning.
//  2. Decision enum is bounded (3 outcomes).
//  3. QualityReport is JSON-stable for SSE wire format.
//  4. HeuristicRunEvaluator is deterministic (same input → same output).
//  5. Evaluator runs AFTER generator (post-hoc) and bounds its own latency.
//  6. Findings cite a stable Dimension string for analytics aggregation.

func TestBDD_GeneratorEvaluatorSeparation(t *testing.T) {

	t.Run("Scenario_EvaluatorReceivesObservableOutputOnly", func(t *testing.T) {
		// Given the generator just finished a run,
		// When the evaluator is invoked,
		// Then it sees ONLY the observable RunOutput shape (final text,
		//      counts, costs) — NOT the system prompt, hooks, or
		//      generator's reasoning trace. Architectural separation is
		//      enforced by the type system: RunOutput has no field
		//      named SystemPrompt / Hooks / Reasoning / ChainOfThought.
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		out := RunOutput{FinalText: "I followed the rules and produced this answer."}
		rep, err := ev.Evaluate(context.Background(), out)
		assert.NoError(t, err)
		assert.Equal(t, "heuristic", rep.EvaluatorName,
			"evaluator must self-identify so analytics can attribute reports")

		// The evaluator score MUST NOT depend on whether the generator
		// thought it did well — it depends only on observable output.
		// We test this indirectly: same FinalText with different
		// (hypothetical) reasoning would give the same score, since
		// reasoning is not part of RunOutput at all.
		rep2, _ := ev.Evaluate(context.Background(), out)
		assert.Equal(t, rep.OverallScore, rep2.OverallScore,
			"deterministic eval: same input → same score")
		assert.Equal(t, rep.Decision, rep2.Decision,
			"deterministic eval: same input → same decision")
	})

	t.Run("Scenario_EvaluatorRefusesGeneratorPraiseSignals", func(t *testing.T) {
		// Given the generator might confidently end with "I did a great
		//       job!" (PDF Section 11 — agents praise own work),
		// When the evaluator scores the output,
		// Then the praise text is treated as ordinary content — it does
		//      NOT bias the score upward. Heuristic evaluator looks at
		//      length / tool errors / budget, never at sentiment.
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		praiseOut := RunOutput{
			FinalText:      "I did an excellent job and the answer is perfect!",
			ToolCallCount:  4,
			ToolErrorCount: 4, // ALL tool calls failed
		}
		rep, err := ev.Evaluate(context.Background(), praiseOut)
		assert.NoError(t, err)
		// All 4 tools failed — quality is bad regardless of self-praise.
		assert.NotEqual(t, EvalPass, rep.Decision,
			"praise must not override tool-failure signal")
	})

	t.Run("Scenario_DecisionEnumIsBounded", func(t *testing.T) {
		// Given downstream consumers (UI, dashboards, alerting) bind to
		//       the decision string,
		// When the EvaluationDecision constants are inspected,
		// Then exactly 3 valid values exist: pass / warn / fail.
		assert.Equal(t, EvaluationDecision("pass"), EvalPass)
		assert.Equal(t, EvaluationDecision("warn"), EvalWarn)
		assert.Equal(t, EvaluationDecision("fail"), EvalFail)
		assert.True(t, IsTerminalEvaluationDecision(EvalPass))
		assert.True(t, IsTerminalEvaluationDecision(EvalWarn))
		assert.True(t, IsTerminalEvaluationDecision(EvalFail))
		assert.False(t, IsTerminalEvaluationDecision(EvaluationDecision("unknown")),
			"unknown decisions must be rejected by the contract guard")
	})

	t.Run("Scenario_QualityReportIsJSONStableForSSEWire", func(t *testing.T) {
		// Given QualityReport flows through the SSE event stream,
		// When the report is JSON-marshalled,
		// Then the field names match the stable wire contract
		//      (camelCase, observability layer binds to these keys).
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		rep, _ := ev.Evaluate(context.Background(), RunOutput{
			FinalText:      "valid answer",
			ToolCallCount:  2,
			ToolErrorCount: 1,
		})
		raw, err := json.Marshal(rep)
		assert.NoError(t, err)
		s := string(raw)
		assert.Contains(t, s, `"evaluator"`)
		assert.Contains(t, s, `"overallScore"`)
		assert.Contains(t, s, `"dimensions"`)
		assert.Contains(t, s, `"findings"`)
		assert.Contains(t, s, `"decision"`)
		assert.Contains(t, s, `"evaluatedAt"`)
		assert.Contains(t, s, `"latencyMs"`)
	})

	t.Run("Scenario_HeuristicEvaluatorIsDeterministic", func(t *testing.T) {
		// Given the same RunOutput,
		// When evaluated multiple times,
		// Then OverallScore / Decision / Dimensions are byte-for-byte
		//      identical. PDF Section 11 — deterministic evaluator
		//      output feeds observability and circuit breakers.
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		out := RunOutput{
			FinalText:      "stable input produces stable output",
			ToolCallCount:  3,
			ToolErrorCount: 1,
			HitRetryLimit:  true,
		}
		rep1, _ := ev.Evaluate(context.Background(), out)
		rep2, _ := ev.Evaluate(context.Background(), out)
		assert.Equal(t, rep1.OverallScore, rep2.OverallScore)
		assert.Equal(t, rep1.Decision, rep2.Decision)
		assert.Equal(t, rep1.Dimensions, rep2.Dimensions)
		assert.Equal(t, len(rep1.Findings), len(rep2.Findings),
			"findings count must be deterministic")
	})

	t.Run("Scenario_FindingDimensionsAreStableStrings", func(t *testing.T) {
		// Given the analytics layer aggregates findings by dimension,
		// When findings are produced,
		// Then dimensions come from a closed set of stable strings:
		//      "length", "tool_failure_rate", "budget".
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		out := RunOutput{
			FinalText:      strings.Repeat("x", 200000), // huge — triggers length warn
			ToolCallCount:  2,
			ToolErrorCount: 2, // 100% failure
			HitBudgetCap:   true,
		}
		rep, _ := ev.Evaluate(context.Background(), out)
		validDims := map[string]bool{
			"length": true, "tool_failure_rate": true, "budget": true,
		}
		for _, f := range rep.Findings {
			assert.True(t, validDims[f.Dimension],
				"finding dimension %q must be in the stable set", f.Dimension)
		}
	})

	t.Run("Scenario_FindingSeverityIsBounded", func(t *testing.T) {
		// Given UI surfaces findings with severity icons,
		// When findings are produced,
		// Then severity ∈ {info, warn, error}.
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		out := RunOutput{
			FinalText:      "x",
			ToolCallCount:  10,
			ToolErrorCount: 9,
			HitBudgetCap:   true,
		}
		rep, _ := ev.Evaluate(context.Background(), out)
		validSev := map[string]bool{"info": true, "warn": true, "error": true}
		for _, f := range rep.Findings {
			assert.True(t, validSev[f.Severity],
				"finding severity %q must be in {info,warn,error}", f.Severity)
		}
	})

	t.Run("Scenario_LatencyMsIsRecordedSeparatelyFromGenerator", func(t *testing.T) {
		// Given the evaluator runs AFTER the generator,
		// When the report is produced,
		// Then it carries its OWN latency separate from the run's
		//      DurationMs. Allows attribution: how much of total user
		//      latency came from evaluation vs generation.
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		out := RunOutput{
			FinalText:  "answer",
			DurationMs: 5000, // generator took 5s
		}
		rep, _ := ev.Evaluate(context.Background(), out)
		// Evaluator latency should be much smaller than generator
		// duration — but they are SEPARATE fields.
		assert.GreaterOrEqual(t, rep.LatencyMs, int64(0),
			"evaluator latency tracked")
		// Note: generator's DurationMs is on RunOutput (input), not on
		// the report — confirming separation.
	})

	t.Run("Scenario_EvaluatorContextCancellationIsHonored", func(t *testing.T) {
		// Given a slow evaluator on a cancelled run,
		// When ctx is cancelled before Evaluate runs,
		// Then the evaluator returns an error and produces NO partial
		//      report — observability layer must not record half-done
		//      evaluations.
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := ev.Evaluate(ctx, RunOutput{FinalText: "any"})
		assert.Error(t, err)
	})

	t.Run("Scenario_EmptyRunOutputDoesNotPanic", func(t *testing.T) {
		// Given a degenerate edge case: zero-value RunOutput,
		// When the evaluator runs,
		// Then it returns a report (with low score / fail) — never
		//      panics. Operational robustness for malformed runs.
		ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
		assert.NotPanics(t, func() {
			rep, err := ev.Evaluate(context.Background(), RunOutput{})
			assert.NoError(t, err)
			assert.NotEmpty(t, rep.Decision, "decision must be set even on zero input")
		})
	})

	t.Run("Scenario_EvaluationDataIsCorrelatedToRunIDForTracing", func(t *testing.T) {
		// Given the evaluation event must correlate to the run it
		//       evaluated (PDF Section 11.4 — parent run trace must
		//       chain to evaluator span),
		// When an EvaluationData payload is constructed,
		// Then it carries (RunID, Report) so the tracing layer can
		//      pair them.
		data := EvaluationData{
			RunID: "run-abc-123",
			Report: QualityReport{
				EvaluatorName: "heuristic",
				OverallScore:  0.8,
				Decision:      EvalPass,
			},
		}
		raw, err := json.Marshal(data)
		assert.NoError(t, err)
		s := string(raw)
		assert.Contains(t, s, `"runId":"run-abc-123"`,
			"RunID is the correlation key — must be on wire")
		assert.Contains(t, s, `"report"`)
	})

	t.Run("Scenario_ConfigGuardsThresholdInversion", func(t *testing.T) {
		// Given misconfigured thresholds (warn < fail) would invert the
		//       decision policy — silent, dangerous,
		// When the default config is used,
		// Then warn > fail by construction. Refactors must keep this.
		cfg := DefaultHeuristicRunEvaluatorConfig()
		assert.Greater(t, cfg.WarnDecisionThreshold, cfg.FailDecisionThreshold,
			"warn threshold MUST be > fail threshold — inversion would silently misclassify")
	})
}
