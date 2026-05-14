package agentic

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluator_DefaultConfig_IsBounded(t *testing.T) {
	cfg := DefaultHeuristicRunEvaluatorConfig()
	assert.Greater(t, cfg.MinFinalTextLen, 0)
	assert.Greater(t, cfg.MaxFinalTextLen, cfg.MinFinalTextLen,
		"max must be > min")
	assert.GreaterOrEqual(t, cfg.MaxToolFailureRate, 0.0)
	assert.LessOrEqual(t, cfg.MaxToolFailureRate, 1.0)
	assert.Greater(t, cfg.WarnDecisionThreshold, cfg.FailDecisionThreshold,
		"warn threshold must be > fail threshold")
	assert.LessOrEqual(t, cfg.WarnDecisionThreshold, 1.0)
	assert.GreaterOrEqual(t, cfg.FailDecisionThreshold, 0.0)
}

func TestEvaluator_HappyPath_ReturnsPass(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	out := RunOutput{
		FinalText:      "Here is a clean answer with reasonable length.",
		TurnCount:      2,
		ToolCallCount:  0,
		ToolErrorCount: 0,
	}
	rep, err := ev.Evaluate(context.Background(), out)
	assert.NoError(t, err)
	assert.Equal(t, EvalPass, rep.Decision)
	assert.Equal(t, "heuristic", rep.EvaluatorName)
	assert.Equal(t, 1.0, rep.OverallScore)
	assert.Empty(t, rep.Findings)
}

func TestEvaluator_TooShortFinalText_Fails(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	out := RunOutput{FinalText: "ok"} // 2 chars < 5 default
	rep, err := ev.Evaluate(context.Background(), out)
	assert.NoError(t, err)
	assert.NotEmpty(t, rep.Findings)
	assert.Equal(t, "length", rep.Findings[0].Dimension)
	assert.Equal(t, "error", rep.Findings[0].Severity)
	assert.Less(t, rep.OverallScore, 1.0)
}

func TestEvaluator_TooLongFinalText_Warns(t *testing.T) {
	ev := NewHeuristicRunEvaluator(HeuristicRunEvaluatorConfig{
		MinFinalTextLen:       1,
		MaxFinalTextLen:       100,
		MaxToolFailureRate:    0.5,
		FailDecisionThreshold: 0.4,
		WarnDecisionThreshold: 0.7,
	})
	long := strings.Repeat("x", 200)
	out := RunOutput{FinalText: long}
	rep, err := ev.Evaluate(context.Background(), out)
	assert.NoError(t, err)
	hasLengthWarn := false
	for _, f := range rep.Findings {
		if f.Dimension == "length" && f.Severity == "warn" {
			hasLengthWarn = true
		}
	}
	assert.True(t, hasLengthWarn, "long text must produce length warn finding")
}

func TestEvaluator_HighToolFailureRate_DownscoresAndFails(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	out := RunOutput{
		FinalText:      "answer that is long enough to pass length check",
		ToolCallCount:  4,
		ToolErrorCount: 3, // 75% failure rate >= 50% threshold
	}
	rep, err := ev.Evaluate(context.Background(), out)
	assert.NoError(t, err)
	assert.Less(t, rep.Dimensions["tool_failure_rate"], 0.5)
	hasErr := false
	for _, f := range rep.Findings {
		if f.Dimension == "tool_failure_rate" && f.Severity == "error" {
			hasErr = true
		}
	}
	assert.True(t, hasErr, "high failure rate must produce error finding")
}

func TestEvaluator_RetryLimitHit_AddsBudgetFinding(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	out := RunOutput{
		FinalText:     "answer that is long enough to pass length check",
		HitRetryLimit: true,
	}
	rep, err := ev.Evaluate(context.Background(), out)
	assert.NoError(t, err)
	assert.Less(t, rep.Dimensions["budget"], 1.0)
	hasBudget := false
	for _, f := range rep.Findings {
		if f.Dimension == "budget" {
			hasBudget = true
		}
	}
	assert.True(t, hasBudget, "retry-limit-hit must produce budget finding")
}

func TestEvaluator_BudgetCapHit_DowngradesMoreThanRetryLimit(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	out := RunOutput{
		FinalText:    "answer that is long enough to pass length check",
		HitBudgetCap: true,
	}
	rep, err := ev.Evaluate(context.Background(), out)
	assert.NoError(t, err)
	// budget cap is more severe than retry limit alone.
	assert.LessOrEqual(t, rep.Dimensions["budget"], 0.4)
}

func TestEvaluator_DecisionRespectsThresholds(t *testing.T) {
	cfg := HeuristicRunEvaluatorConfig{
		MinFinalTextLen:       5,
		MaxFinalTextLen:       100000,
		MaxToolFailureRate:    0.5,
		FailDecisionThreshold: 0.4,
		WarnDecisionThreshold: 0.7,
	}
	ev := NewHeuristicRunEvaluator(cfg)

	// Force fail by combining bad signals on every dimension:
	// length=0 (text shorter than min) + tool_failure=0.1 + budget=0.3
	// → avg = (0 + 0.1 + 0.3)/3 = 0.133 < 0.4 → fail.
	failOut := RunOutput{
		FinalText:      "x",
		ToolCallCount:  10,
		ToolErrorCount: 9,
		HitBudgetCap:   true,
	}
	rep, err := ev.Evaluate(context.Background(), failOut)
	assert.NoError(t, err)
	assert.Equal(t, EvalFail, rep.Decision)
}

func TestEvaluator_DecisionEnumIsBounded(t *testing.T) {
	assert.True(t, IsTerminalEvaluationDecision(EvalPass))
	assert.True(t, IsTerminalEvaluationDecision(EvalWarn))
	assert.True(t, IsTerminalEvaluationDecision(EvalFail))
	assert.False(t, IsTerminalEvaluationDecision(EvaluationDecision("unknown")))
	assert.False(t, IsTerminalEvaluationDecision(""))
}

func TestEvaluator_ContextCancelled_ReturnsError(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ev.Evaluate(ctx, RunOutput{FinalText: "anything"})
	assert.Error(t, err)
}

func TestEvaluator_OverallScoreIsAverageOfDimensions(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	rep, err := ev.Evaluate(context.Background(), RunOutput{
		FinalText: "long enough text to pass the length check fine",
	})
	assert.NoError(t, err)
	sum := 0.0
	for _, v := range rep.Dimensions {
		sum += v
	}
	expected := sum / float64(len(rep.Dimensions))
	assert.InDelta(t, expected, rep.OverallScore, 0.0001)
}

func TestEvaluator_LatencyAndTimestampPopulated(t *testing.T) {
	ev := NewHeuristicRunEvaluator(DefaultHeuristicRunEvaluatorConfig())
	rep, err := ev.Evaluate(context.Background(), RunOutput{FinalText: "hello world"})
	assert.NoError(t, err)
	assert.False(t, rep.EvaluatedAt.IsZero(), "EvaluatedAt must be populated")
	assert.GreaterOrEqual(t, rep.LatencyMs, int64(0))
}
