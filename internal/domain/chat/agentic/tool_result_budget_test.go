package agentic

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolResultBudget_ActionEnumIsBounded(t *testing.T) {
	for _, a := range AllToolResultBudgetActions() {
		assert.True(t, IsValidToolResultBudgetAction(a))
	}
	assert.False(t, IsValidToolResultBudgetAction(ToolResultBudgetAction("rewrite")))
	assert.Equal(t, 4, len(AllToolResultBudgetActions()))
}

func TestToolResultBudget_DefaultConfigHasSensibleDefaults(t *testing.T) {
	cfg := DefaultToolResultBudgetConfig()
	assert.Equal(t, 4000, cfg.PerCallMaxTokens)
	assert.Equal(t, 12000, cfg.PerTurnMaxTokens)
	assert.Equal(t, 50000, cfg.PerRunMaxTokens)
	assert.Equal(t, 8000, cfg.ForceSummarizeOverTokens)
}

func TestEnforce_KeepsSmallResultVerbatim(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	body := strings.Repeat("x", 100) // 25 tokens
	d, err := e.Enforce("ping", body)
	require.NoError(t, err)
	assert.Equal(t, ToolResultBudgetKeep, d.Action)
	assert.Equal(t, body, d.FinalBody)
	assert.False(t, d.IsBudgetReason())
}

func TestEnforce_TruncatesAbovePerCallCap(t *testing.T) {
	cfg := ToolResultBudgetConfig{
		PerCallMaxTokens:         100,
		PerTurnMaxTokens:         100000,
		PerRunMaxTokens:          1000000,
		ForceSummarizeOverTokens: 0,
	}
	e := NewToolResultBudgetEnforcer(cfg)
	body := strings.Repeat("x", 1000) // 250 tokens > cap 100
	d, err := e.Enforce("query", body)
	require.NoError(t, err)
	assert.Equal(t, ToolResultBudgetTruncate, d.Action)
	assert.LessOrEqual(t, d.FinalTokens, 101)
	assert.True(t, d.OriginalTokens > d.FinalTokens)
	assert.True(t, d.IsBudgetReason())
}

func TestEnforce_SummarizesAboveForceSummarizeThreshold(t *testing.T) {
	cfg := ToolResultBudgetConfig{
		PerCallMaxTokens:         1000000,
		PerTurnMaxTokens:         1000000,
		PerRunMaxTokens:          1000000,
		ForceSummarizeOverTokens: 100,
	}
	e := NewToolResultBudgetEnforcer(cfg)
	body := strings.Repeat("x", 1000) // 250 > 100
	d, err := e.Enforce("query", body)
	require.NoError(t, err)
	assert.Equal(t, ToolResultBudgetSummarize, d.Action)
	assert.Contains(t, d.FinalBody, "summarized")
	assert.Contains(t, d.FinalBody, "query")
}

func TestEnforce_SummarizeTakesPrecedenceOverTruncate(t *testing.T) {
	cfg := ToolResultBudgetConfig{
		PerCallMaxTokens:         100,
		ForceSummarizeOverTokens: 50,
	}
	e := NewToolResultBudgetEnforcer(cfg)
	body := strings.Repeat("x", 1000)
	d, _ := e.Enforce("q", body)
	assert.Equal(t, ToolResultBudgetSummarize, d.Action,
		"force-summarize must trigger before per-call truncate")
}

func TestEnforce_DropsWhenPerRunBudgetExceeded(t *testing.T) {
	cfg := ToolResultBudgetConfig{
		PerCallMaxTokens: 100,
		PerRunMaxTokens:  150,
	}
	e := NewToolResultBudgetEnforcer(cfg)
	// First call uses 100 tokens.
	_, _ = e.Enforce("q", strings.Repeat("x", 400))
	// Second call would push run cumulative to 200 > 150.
	d, err := e.Enforce("q", strings.Repeat("y", 400))
	require.NoError(t, err)
	assert.Equal(t, ToolResultBudgetDrop, d.Action)
	assert.Contains(t, d.FinalBody, "dropped")
}

func TestEnforce_DropsWhenPerTurnBudgetExceeded(t *testing.T) {
	cfg := ToolResultBudgetConfig{
		PerCallMaxTokens: 200,
		PerTurnMaxTokens: 150,
		PerRunMaxTokens:  100000,
	}
	e := NewToolResultBudgetEnforcer(cfg)
	_, _ = e.Enforce("q", strings.Repeat("x", 400)) // 100 tokens
	d, _ := e.Enforce("q", strings.Repeat("y", 400)) // would push to 200 > 150
	assert.Equal(t, ToolResultBudgetDrop, d.Action)
	assert.Contains(t, d.Reason, "turn cumulative")
}

func TestEnforce_RejectsEmptyBody(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	_, err := e.Enforce("q", "")
	assert.True(t, errors.Is(err, ErrToolResultBudgetEmptyBody))
}

func TestEnforce_TracksCumulativeAcrossCalls(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	for i := 0; i < 3; i++ {
		_, _ = e.Enforce("q", strings.Repeat("x", 400)) // 100 tokens each
	}
	assert.Equal(t, 300, e.PerTurnUsed())
	assert.Equal(t, 300, e.PerRunUsed())
}

func TestEnforce_EndTurnResetsPerTurnButNotPerRun(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	_, _ = e.Enforce("q", strings.Repeat("x", 400))
	assert.Equal(t, 100, e.PerTurnUsed())
	assert.Equal(t, 100, e.PerRunUsed())
	e.EndTurn()
	assert.Equal(t, 0, e.PerTurnUsed())
	assert.Equal(t, 100, e.PerRunUsed(), "per-run survives turn boundary")
}

func TestEnforce_EndRunResetsBoth(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	_, _ = e.Enforce("q", strings.Repeat("x", 400))
	e.EndRun()
	assert.Equal(t, 0, e.PerTurnUsed())
	assert.Equal(t, 0, e.PerRunUsed())
}

func TestEnforce_ZeroBudgetMeansUnlimited(t *testing.T) {
	cfg := ToolResultBudgetConfig{} // all zeros
	e := NewToolResultBudgetEnforcer(cfg)
	huge := strings.Repeat("x", 100000) // 25k tokens
	d, err := e.Enforce("q", huge)
	require.NoError(t, err)
	assert.Equal(t, ToolResultBudgetKeep, d.Action,
		"zero limits = unlimited")
}

func TestEnforce_DropPreemptsTruncateWhenRunBudgetExceeded(t *testing.T) {
	cfg := ToolResultBudgetConfig{
		PerCallMaxTokens: 1000,
		PerRunMaxTokens:  50,
	}
	e := NewToolResultBudgetEnforcer(cfg)
	// First call: 100 tokens already > PerRun cap → dropped immediately.
	d, _ := e.Enforce("q", strings.Repeat("x", 400))
	assert.Equal(t, ToolResultBudgetDrop, d.Action)
}

func TestEnforce_DecisionRecordsOriginalAndFinalTokens(t *testing.T) {
	cfg := ToolResultBudgetConfig{PerCallMaxTokens: 100}
	e := NewToolResultBudgetEnforcer(cfg)
	d, _ := e.Enforce("q", strings.Repeat("x", 1000))
	assert.Equal(t, 250, d.OriginalTokens)
	assert.LessOrEqual(t, d.FinalTokens, 101)
}

func TestEnforce_DecisionRecordsCumulativeAfter(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	d, _ := e.Enforce("q", strings.Repeat("x", 400))
	assert.Equal(t, 100, d.PerTurnUsedAfter)
	assert.Equal(t, 100, d.PerRunUsedAfter)
}

func TestConfigSummary_StringFormat(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	summary := e.ConfigSummary()
	assert.Contains(t, summary, "per_call=4000")
	assert.Contains(t, summary, "per_turn=12000")
	assert.Contains(t, summary, "per_run=50000")
	assert.Contains(t, summary, "force_summarize_over=8000")
}

func TestEnforceMany_BatchesDecisions(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	items := []struct {
		ToolName string
		Body     string
	}{
		{"a", "small"},
		{"b", "also small"},
	}
	got, err := e.EnforceMany(items)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, d := range got {
		assert.Equal(t, ToolResultBudgetKeep, d.Action)
	}
}

func TestEnforceMany_PropagatesEmptyBodyError(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	items := []struct {
		ToolName string
		Body     string
	}{
		{"good", "ok"},
		{"bad", ""},
	}
	_, err := e.EnforceMany(items)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}

func TestEnforce_ConcurrentEnforceIsSafe(t *testing.T) {
	e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = e.Enforce("q", "small")
		}()
	}
	wg.Wait()
	// Each enforce uses ~2 tokens ("small" = 5 chars / 4 ~= 2 tokens).
	// PerRunUsed should equal 50 * 2 = 100 (but 5/4 rounded up to 2).
	assert.GreaterOrEqual(t, e.PerRunUsed(), 50)
}

func TestEnforce_IsBudgetReasonSet(t *testing.T) {
	cfg := ToolResultBudgetConfig{PerCallMaxTokens: 50}
	e := NewToolResultBudgetEnforcer(cfg)
	d, _ := e.Enforce("q", strings.Repeat("x", 1000))
	assert.True(t, d.IsBudgetReason())

	d2, _ := e.Enforce("q2", "small")
	assert.False(t, d2.IsBudgetReason())
}
