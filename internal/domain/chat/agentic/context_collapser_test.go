package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func msg(role, kind, content string) TranscriptMessage {
	return TranscriptMessage{
		ID:        uuid.New(),
		Role:      role,
		Kind:      kind,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func errMsg(role, kind, content string) TranscriptMessage {
	m := msg(role, kind, content)
	m.IsError = true
	return m
}

func TestCollapseStrategy_EnumIsBounded(t *testing.T) {
	for _, s := range AllCollapseStrategies() {
		assert.True(t, IsValidCollapseStrategy(s))
	}
	assert.False(t, IsValidCollapseStrategy(CollapseStrategy("nuclear")))
	assert.Equal(t, 3, len(AllCollapseStrategies()))
}

func TestProject_RejectsInvalidStrategy(t *testing.T) {
	c := NewContextCollapser()
	_, err := c.Project(context.Background(), CollapseInput{
		Messages: []TranscriptMessage{},
		Strategy: "bogus",
	})
	assert.True(t, errors.Is(err, ErrCollapserInvalidStrategy))
}

func TestProject_RejectsNilMessages(t *testing.T) {
	c := NewContextCollapser()
	_, err := c.Project(context.Background(), CollapseInput{
		Strategy: CollapseStrategyMinimal,
	})
	assert.True(t, errors.Is(err, ErrCollapserNilInput))
}

func TestProject_MinimalKeepsEverything(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{
		msg("user", "text", "hello"),
		msg("assistant", "tool_use", "calling tool"),
		msg("system", "tool_result", "result body"),
		msg("assistant", "text", "response"),
	}
	view, err := c.Project(context.Background(), CollapseInput{
		Messages: msgs,
		Strategy: CollapseStrategyMinimal,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, view.MessageCount())
	assert.Equal(t, 0, view.DroppedCount)
}

func TestProject_BalancedDropsToolUseKeepsResults(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{
		msg("user", "text", "hello"),
		msg("assistant", "tool_use", "calling tool"),
		msg("system", "tool_result", "result body"),
		msg("assistant", "text", "response"),
	}
	view, _ := c.Project(context.Background(), CollapseInput{
		Messages: msgs,
		Strategy: CollapseStrategyBalanced,
	})
	// tool_use dropped; tool_result kept.
	assert.Equal(t, 3, view.MessageCount())
	for _, m := range view.Messages {
		assert.NotEqual(t, "tool_use", m.Kind)
	}
}

func TestProject_BalancedKeepsErrorToolUse(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{
		errMsg("assistant", "tool_use", "failed call"),
	}
	view, _ := c.Project(context.Background(), CollapseInput{
		Messages: msgs,
		Strategy: CollapseStrategyBalanced,
	})
	assert.Equal(t, 1, view.MessageCount(), "errors always preserved")
}

func TestProject_AggressiveKeepsOnlyAssistantTextErrorsAndSummaries(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{
		msg("user", "text", "user input"),
		msg("assistant", "tool_use", "tool call"),
		msg("system", "tool_result", "result"),
		errMsg("assistant", "tool_result", "error"),
		msg("assistant", "text", "summary response"),
		msg("system", "compact_summary", "compaction marker"),
	}
	view, _ := c.Project(context.Background(), CollapseInput{
		Messages: msgs,
		Strategy: CollapseStrategyAggressive,
	})
	// Kept: error tool_result + assistant text + compact_summary = 3.
	assert.Equal(t, 3, view.MessageCount())
}

func TestProject_BoundaryCutDropsPreAnchor(t *testing.T) {
	c := NewContextCollapser()
	anchorID := uuid.New()
	msgs := []TranscriptMessage{
		msg("user", "text", "pre1"),
		msg("user", "text", "pre2"),
		{ID: anchorID, Role: "user", Kind: "text", Content: "anchor"},
		msg("assistant", "text", "post"),
	}
	boundary := &CompactBoundary{
		ID:               uuid.New(),
		SessionID:        uuid.New(),
		SummaryMessageID: uuid.New(),
		HeadUUID:         uuid.New(),
		AnchorUUID:       anchorID,
		TailUUID:         uuid.New(),
		SummarizedCount:  2,
	}
	view, _ := c.Project(context.Background(), CollapseInput{
		Messages:       msgs,
		Strategy:       CollapseStrategyMinimal,
		LatestBoundary: boundary,
	})
	assert.Equal(t, 2, view.MessageCount(), "anchor + post = 2")
	assert.Equal(t, 2, view.DroppedReasons["pre_boundary_anchor"])
}

func TestProject_BoundaryAnchorNotInMessagesKeepsAll(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{
		msg("user", "text", "msg1"),
		msg("user", "text", "msg2"),
	}
	boundary := &CompactBoundary{AnchorUUID: uuid.New()} // not in messages
	view, _ := c.Project(context.Background(), CollapseInput{
		Messages:       msgs,
		Strategy:       CollapseStrategyMinimal,
		LatestBoundary: boundary,
	})
	assert.Equal(t, 2, view.MessageCount(), "no anchor → no cut")
}

func TestProject_DeterministicForSameInput(t *testing.T) {
	c := NewContextCollapser()
	now := time.Now()
	c.SetClock(func() time.Time { return now })
	msgs := []TranscriptMessage{
		msg("user", "text", "x"),
		msg("assistant", "tool_use", "y"),
	}
	in := CollapseInput{Messages: msgs, Strategy: CollapseStrategyBalanced}
	v1, _ := c.Project(context.Background(), in)
	v2, _ := c.Project(context.Background(), in)
	assert.Equal(t, v1.MessageCount(), v2.MessageCount())
	assert.Equal(t, v1.DroppedCount, v2.DroppedCount)
}

func TestProject_EmptyMessagesReturnsEmptyView(t *testing.T) {
	c := NewContextCollapser()
	view, err := c.Project(context.Background(), CollapseInput{
		Messages: []TranscriptMessage{},
		Strategy: CollapseStrategyMinimal,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, view.MessageCount())
	assert.Equal(t, 0, view.OriginalCount)
}

func TestProject_DoesNotMutateInput(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{
		msg("user", "text", "x"),
		msg("assistant", "tool_use", "y"),
	}
	originalLen := len(msgs)
	_, _ = c.Project(context.Background(), CollapseInput{
		Messages: msgs,
		Strategy: CollapseStrategyBalanced,
	})
	assert.Equal(t, originalLen, len(msgs), "input not mutated")
}

func TestCollapsedView_Render(t *testing.T) {
	view := CollapsedView{
		Messages: []TranscriptMessage{
			{Role: "user", Kind: "text", Content: "hello"},
			{Role: "assistant", Kind: "text", Content: "hi"},
		},
	}
	rendered := view.Render()
	assert.Contains(t, rendered, "[user/text]")
	assert.Contains(t, rendered, "[assistant/text]")
	assert.Contains(t, rendered, "hello")
	assert.Contains(t, rendered, "hi")
}

func TestCollapsedView_RenderErrorMarker(t *testing.T) {
	view := CollapsedView{
		Messages: []TranscriptMessage{
			{Role: "assistant", Kind: "tool_result", Content: "err", IsError: true},
		},
	}
	rendered := view.Render()
	assert.Contains(t, rendered, "/error")
}

func TestCollapsedView_Compression(t *testing.T) {
	view := CollapsedView{OriginalCount: 10, DroppedCount: 7}
	assert.InDelta(t, 0.7, view.Compression(), 0.001)
}

func TestCollapsedView_CompressionEmpty(t *testing.T) {
	view := CollapsedView{OriginalCount: 0}
	assert.Equal(t, 0.0, view.Compression())
}

func TestCollapsedView_TraceForAudit(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{
		msg("user", "text", "x"),
		msg("assistant", "tool_use", "y"),
		msg("assistant", "tool_use", "z"),
	}
	view, _ := c.Project(context.Background(), CollapseInput{
		Messages: msgs,
		Strategy: CollapseStrategyBalanced,
	})
	trace := view.Trace()
	assert.Equal(t, 3, trace.OriginalCount)
	assert.Equal(t, 1, trace.FinalCount)
	assert.InDelta(t, 2.0/3.0, trace.Compression, 0.001)
	assert.Equal(t, CollapseStrategyBalanced, trace.Strategy)
	assert.Equal(t, 2, trace.DroppedReasons["tool_use_redacted"])
}

func TestCollapsedView_SortedDroppedReasons(t *testing.T) {
	view := CollapsedView{
		DroppedReasons: map[string]int{
			"a_reason": 1,
			"b_reason": 5,
			"c_reason": 3,
		},
	}
	sorted := view.SortedDroppedReasons()
	require.Len(t, sorted, 3)
	assert.Equal(t, "b_reason", sorted[0].Reason)
	assert.Equal(t, 5, sorted[0].Count)
	assert.Equal(t, "c_reason", sorted[1].Reason)
}

func TestProject_ConcurrentSafe(t *testing.T) {
	c := NewContextCollapser()
	msgs := []TranscriptMessage{msg("user", "text", "x")}
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.Project(context.Background(), CollapseInput{
				Messages: msgs,
				Strategy: CollapseStrategyBalanced,
			})
		}()
	}
	wg.Wait()
}

func TestProject_ContextCancelled(t *testing.T) {
	c := NewContextCollapser()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Project(ctx, CollapseInput{
		Messages: []TranscriptMessage{msg("user", "text", "x")},
		Strategy: CollapseStrategyMinimal,
	})
	assert.Error(t, err)
}
