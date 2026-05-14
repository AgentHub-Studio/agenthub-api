package agentic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ContextCollapser(t *testing.T) {
	t.Run("Scenario_SameTranscriptProjectsDifferentlyForDifferentConsumers", func(t *testing.T) {
		// Given a raw transcript with tool calls + text turns,
		// And the agent renderer wants verbose (minimal), the analytics
		// reporter wants summary (balanced), and the cost dashboard
		// wants only outcomes (aggressive),
		// When the same transcript is projected three ways,
		// Then THREE DIFFERENT collapsed views emerge from ONE source —
		// raw data preserved; projection is read-time (PDF §7.11).
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("user", "text", "x"),
			msg("assistant", "tool_use", "y"),
			msg("system", "tool_result", "z"),
			msg("assistant", "text", "summary"),
		}
		min, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyMinimal})
		bal, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyBalanced})
		agg, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyAggressive})

		assert.Greater(t, min.MessageCount(), bal.MessageCount())
		assert.Greater(t, bal.MessageCount(), agg.MessageCount())
	})

	t.Run("Scenario_RawTranscriptIsNeverMutatedByProjection", func(t *testing.T) {
		// Given the runtime calls Project on a transcript that other
		// components still hold a reference to,
		// When projection runs,
		// Then input slice is NOT mutated — replay debugging and audit
		// can re-read the original later.
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("user", "text", "x"),
			msg("assistant", "tool_use", "y"),
		}
		_, _ = c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyAggressive})
		// Original still has both messages.
		assert.Equal(t, 2, len(msgs))
		assert.Equal(t, "tool_use", msgs[1].Kind)
	})

	t.Run("Scenario_BoundaryCutAppliedAtReadTimeNotWriteTime", func(t *testing.T) {
		// Given a session had a compact boundary recorded,
		// When the renderer requests projection,
		// Then it cuts pre-boundary messages — but the database still
		// holds the raw history (write-time data preserved; read-time
		// view is computed).
		c := NewContextCollapser()
		anchorID := uuid.New()
		msgs := []TranscriptMessage{
			msg("user", "text", "old1"),
			msg("user", "text", "old2"),
			{ID: anchorID, Role: "assistant", Kind: "text", Content: "anchor"},
			msg("assistant", "text", "fresh"),
		}
		view, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs,
			Strategy: CollapseStrategyMinimal,
			LatestBoundary: &CompactBoundary{
				AnchorUUID:      anchorID,
				HeadUUID:        uuid.New(),
				TailUUID:        uuid.New(),
				SessionID:       uuid.New(),
				SummaryMessageID: uuid.New(),
				SummarizedCount: 2,
			},
		})
		assert.Equal(t, 2, view.MessageCount(), "anchor + fresh = 2")
		assert.Equal(t, 4, view.OriginalCount, "original transcript still 4")
	})

	t.Run("Scenario_ErrorsAlwaysSurviveEvenAggressiveCollapse", func(t *testing.T) {
		// Given debugging needs to see what failed,
		// When the most aggressive strategy is applied,
		// Then error messages are STILL kept — silently dropping errors
		// would defeat post-mortem investigation.
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("user", "text", "x"),
			msg("assistant", "tool_use", "ok call"),
			errMsg("system", "tool_result", "boom"),
		}
		view, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyAggressive})
		hasError := false
		for _, m := range view.Messages {
			if m.IsError {
				hasError = true
			}
		}
		assert.True(t, hasError, "error preserved even with aggressive collapse")
	})

	t.Run("Scenario_BalancedDropsRedundantToolCallsButKeepsResults", func(t *testing.T) {
		// Given tool_use messages are redundant with their results
		// (LLM already saw the call; we don't need to replay it),
		// When balanced strategy runs,
		// Then tool_use is dropped but tool_result kept — half the
		// noise gone but context preserved.
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("assistant", "tool_use", "calling X"),
			msg("system", "tool_result", "X returned 42"),
		}
		view, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyBalanced})
		require.Equal(t, 1, view.MessageCount())
		assert.Equal(t, "tool_result", view.Messages[0].Kind)
	})

	t.Run("Scenario_AggressivePreservesCompactSummariesForResume", func(t *testing.T) {
		// Given PDF §9.2 resume reconstruction depends on compact_summary
		// markers being present,
		// When the most aggressive collapse runs,
		// Then compact_summary messages are still kept — resume integrity
		// preserved even under extreme collapse.
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("user", "text", "x"),
			msg("system", "compact_summary", "summary of turns 1-10"),
		}
		view, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyAggressive})
		hasSummary := false
		for _, m := range view.Messages {
			if m.Kind == "compact_summary" {
				hasSummary = true
			}
		}
		assert.True(t, hasSummary)
	})

	t.Run("Scenario_TraceProvidesGOV001AuditDataForCompressionDecisions", func(t *testing.T) {
		// Given GOV-001 audits what the LLM ultimately received,
		// When projection drops messages,
		// Then trace.DroppedReasons enumerates each drop type + count
		// — auditor can verify the projection was reasonable.
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("user", "text", "x"),
			msg("assistant", "tool_use", "a"),
			msg("assistant", "tool_use", "b"),
			msg("assistant", "tool_use", "c"),
		}
		view, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyBalanced})
		trace := view.Trace()
		assert.Equal(t, 4, trace.OriginalCount)
		assert.Equal(t, 1, trace.FinalCount)
		assert.Equal(t, 3, trace.DroppedReasons["tool_use_redacted"])
	})

	t.Run("Scenario_DeterministicProjectionForReplayDebugging", func(t *testing.T) {
		// Given replay debugging requires reproducible outputs,
		// When the same projection runs twice on the same input,
		// Then output is byte-identical — same messages + same counts
		// + same drop reasons.
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("user", "text", "x"),
			msg("assistant", "tool_use", "y"),
		}
		in := CollapseInput{Messages: msgs, Strategy: CollapseStrategyAggressive}
		v1, _ := c.Project(context.Background(), in)
		v2, _ := c.Project(context.Background(), in)
		assert.Equal(t, v1.Trace(), v2.Trace())
	})

	t.Run("Scenario_BoundaryAnchorMissingFromMessagesIsGracefullyHandled", func(t *testing.T) {
		// Given the latest boundary points to an anchor not in this
		// projection's message slice (e.g. partial transcript replay),
		// When projection runs,
		// Then NO cut applied (graceful degradation; better to over-show
		// than silently drop everything).
		c := NewContextCollapser()
		msgs := []TranscriptMessage{msg("user", "text", "only")}
		view, _ := c.Project(context.Background(), CollapseInput{
			Messages:       msgs,
			Strategy:       CollapseStrategyMinimal,
			LatestBoundary: &CompactBoundary{AnchorUUID: uuid.New()},
		})
		assert.Equal(t, 1, view.MessageCount())
	})

	t.Run("Scenario_CompressionRatioReportsAggregateImpact", func(t *testing.T) {
		// Given OBS-009 quality reports include compression effectiveness,
		// When the collapser projects,
		// Then Compression() returns dropped/original ratio — easy KPI
		// for dashboards.
		c := NewContextCollapser()
		msgs := []TranscriptMessage{
			msg("user", "text", "x"),
			msg("assistant", "tool_use", "a"),
			msg("assistant", "tool_use", "b"),
			msg("assistant", "tool_use", "c"),
		}
		view, _ := c.Project(context.Background(), CollapseInput{
			Messages: msgs, Strategy: CollapseStrategyBalanced})
		// 3 of 4 dropped = 0.75.
		assert.InDelta(t, 0.75, view.Compression(), 0.001)
	})
}
