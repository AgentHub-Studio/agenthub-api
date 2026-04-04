package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func gm(id, typ, assistantID string) agentic.GroupableMessage {
	return agentic.GroupableMessage{ID: id, Type: typ, AssistantID: assistantID}
}

// --- GroupMessagesByAPIRound ---

func TestGroupMessagesByAPIRound_Empty(t *testing.T) {
	groups := agentic.GroupMessagesByAPIRound(nil)
	assert.Nil(t, groups)
}

func TestGroupMessagesByAPIRound_SingleMessage(t *testing.T) {
	msgs := []agentic.GroupableMessage{
		gm("m1", "user", ""),
	}
	groups := agentic.GroupMessagesByAPIRound(msgs)
	require.Len(t, groups, 1)
	assert.Len(t, groups[0], 1)
}

func TestGroupMessagesByAPIRound_SingleRound(t *testing.T) {
	msgs := []agentic.GroupableMessage{
		gm("m1", "user", ""),
		gm("m2", "assistant", "a1"),
		gm("m3", "tool_use", "a1"),
		gm("m4", "tool_result", ""),
		gm("m5", "assistant", "a1"), // same assistant ID
	}
	groups := agentic.GroupMessagesByAPIRound(msgs)
	require.Len(t, groups, 1)
	assert.Len(t, groups[0], 5)
}

func TestGroupMessagesByAPIRound_TwoRounds(t *testing.T) {
	msgs := []agentic.GroupableMessage{
		gm("m1", "user", ""),
		gm("m2", "assistant", "a1"),
		gm("m3", "tool_result", ""),
		gm("m4", "assistant", "a2"), // new round
		gm("m5", "tool_result", ""),
	}
	groups := agentic.GroupMessagesByAPIRound(msgs)
	require.Len(t, groups, 2)
	assert.Len(t, groups[0], 3) // user + assistant(a1) + tool_result
	assert.Len(t, groups[1], 2) // assistant(a2) + tool_result
}

func TestGroupMessagesByAPIRound_MultipleToolCalls(t *testing.T) {
	msgs := []agentic.GroupableMessage{
		gm("m1", "user", ""),
		gm("m2", "assistant", "a1"),
		gm("m3", "tool_use", "a1"),
		gm("m4", "tool_result", ""),
		gm("m5", "assistant", "a1"), // same ID — streaming chunk
		gm("m6", "tool_use", "a1"),
		gm("m7", "tool_result", ""),
	}
	groups := agentic.GroupMessagesByAPIRound(msgs)
	require.Len(t, groups, 1)
	assert.Len(t, groups[0], 7, "all tool calls with same assistant ID stay in one group")
}

func TestGroupMessagesByAPIRound_ThreeRounds(t *testing.T) {
	msgs := []agentic.GroupableMessage{
		gm("m1", "user", ""),
		gm("m2", "assistant", "a1"),
		gm("m3", "assistant", "a2"),
		gm("m4", "assistant", "a3"),
	}
	groups := agentic.GroupMessagesByAPIRound(msgs)
	require.Len(t, groups, 3)
	assert.Len(t, groups[0], 2) // user + assistant(a1)
	assert.Len(t, groups[1], 1) // assistant(a2)
	assert.Len(t, groups[2], 1) // assistant(a3)
}

func TestGroupMessagesByAPIRound_NonAssistantOnly(t *testing.T) {
	msgs := []agentic.GroupableMessage{
		gm("m1", "user", ""),
		gm("m2", "system", ""),
		gm("m3", "user", ""),
	}
	groups := agentic.GroupMessagesByAPIRound(msgs)
	require.Len(t, groups, 1)
	assert.Len(t, groups[0], 3, "no assistant messages means one group")
}

// --- SummarizeRounds ---

func TestSummarizeRounds(t *testing.T) {
	msgs := []agentic.GroupableMessage{
		gm("m1", "user", ""),
		gm("m2", "assistant", "a1"),
		gm("m3", "tool_use", "a1"),
		gm("m4", "tool_result", ""),
		gm("m5", "assistant", "a2"),
	}
	groups := agentic.GroupMessagesByAPIRound(msgs)
	summaries := agentic.SummarizeRounds(groups)

	require.Len(t, summaries, 2)
	assert.Equal(t, "a1", summaries[0].AssistantID)
	assert.Equal(t, 4, summaries[0].Messages)
	assert.True(t, summaries[0].HasToolUse)
	assert.True(t, summaries[0].HasText)

	assert.Equal(t, "a2", summaries[1].AssistantID)
	assert.Equal(t, 1, summaries[1].Messages)
	assert.False(t, summaries[1].HasToolUse)
}

func TestSummarizeRounds_Empty(t *testing.T) {
	summaries := agentic.SummarizeRounds(nil)
	assert.Empty(t, summaries)
}
