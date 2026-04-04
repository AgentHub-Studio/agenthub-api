package agentic_test

import (
	"encoding/json"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestMailbox_SendAndReadUnread(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()

	msg := mb.Send(sessionID, "agent-a", "agent-b", "found schema info")
	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, "agent-a", msg.From)
	assert.Equal(t, "agent-b", msg.To)
	assert.Equal(t, "found schema info", msg.Content)
	assert.False(t, msg.Read)
	assert.Equal(t, 1, mb.Len(sessionID))

	// agent-b reads the message.
	msgs := mb.ReadUnread(sessionID, "agent-b")
	require.Len(t, msgs, 1)
	assert.Equal(t, "found schema info", msgs[0].Content)
	assert.True(t, msgs[0].Read)

	// Reading again returns nothing (already read).
	msgs = mb.ReadUnread(sessionID, "agent-b")
	assert.Empty(t, msgs)
}

func TestMailbox_Broadcast(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()

	mb.Send(sessionID, "agent-a", "*", "broadcast message")

	// First recipient gets the broadcast.
	msgs := mb.ReadUnread(sessionID, "agent-b")
	require.Len(t, msgs, 1)
	assert.Equal(t, "broadcast message", msgs[0].Content)

	// Once read, another agent won't see it (already marked read).
	msgs = mb.ReadUnread(sessionID, "agent-c")
	assert.Empty(t, msgs)
}

func TestMailbox_MultipleMessages(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()

	mb.Send(sessionID, "a", "b", "msg1")
	mb.Send(sessionID, "c", "b", "msg2")
	mb.Send(sessionID, "a", "d", "msg3") // not for b

	msgs := mb.ReadUnread(sessionID, "b")
	require.Len(t, msgs, 2)
	assert.Equal(t, "msg1", msgs[0].Content)
	assert.Equal(t, "msg2", msgs[1].Content)
}

func TestMailbox_PendingCount(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()

	assert.Equal(t, 0, mb.PendingCount(sessionID, "b"))

	mb.Send(sessionID, "a", "b", "msg1")
	mb.Send(sessionID, "a", "b", "msg2")
	mb.Send(sessionID, "a", "c", "not-for-b")

	assert.Equal(t, 2, mb.PendingCount(sessionID, "b"))

	mb.ReadUnread(sessionID, "b")
	assert.Equal(t, 0, mb.PendingCount(sessionID, "b"))
}

func TestMailbox_Cleanup(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()

	mb.Send(sessionID, "a", "b", "msg1")
	assert.Equal(t, 1, mb.Len(sessionID))

	mb.Cleanup(sessionID)
	assert.Equal(t, 0, mb.Len(sessionID))

	// ReadUnread after cleanup returns nothing.
	msgs := mb.ReadUnread(sessionID, "b")
	assert.Empty(t, msgs)
}

func TestMailbox_IsolatedSessions(t *testing.T) {
	mb := agentic.NewMailbox()
	session1 := uuid.New()
	session2 := uuid.New()

	mb.Send(session1, "a", "b", "session 1 msg")
	mb.Send(session2, "a", "b", "session 2 msg")

	msgs := mb.ReadUnread(session1, "b")
	require.Len(t, msgs, 1)
	assert.Equal(t, "session 1 msg", msgs[0].Content)

	msgs = mb.ReadUnread(session2, "b")
	require.Len(t, msgs, 1)
	assert.Equal(t, "session 2 msg", msgs[0].Content)
}

func TestDrainMailbox_NoPending(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()

	result := agentic.DrainMailbox(mb, sessionID, "agent-b")
	assert.Empty(t, result)
}

func TestDrainMailbox_NilMailbox(t *testing.T) {
	result := agentic.DrainMailbox(nil, uuid.New(), "agent-b")
	assert.Empty(t, result)
}

func TestDrainMailbox_WithMessages(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()

	mb.Send(sessionID, "agent-a", "agent-b", "found the schema")
	mb.Send(sessionID, "agent-c", "agent-b", "query results ready")

	result := agentic.DrainMailbox(mb, sessionID, "agent-b")
	assert.Contains(t, result, "[From agent-a]: found the schema")
	assert.Contains(t, result, "[From agent-c]: query results ready")
	assert.Contains(t, result, "--- Messages from other agents ---")
}

func TestHandleSendMessage_Success(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()
	ch := make(chan agentic.RunEvent, 10)

	input, _ := json.Marshal(map[string]string{
		"to":      "agent-b",
		"message": "hello from a",
	})

	result := agentic.HandleSendMessage(mb, sessionID, "agent-a", input, ch)
	assert.Nil(t, result.Error)
	assert.NotNil(t, result.Output)

	var output agentic.SendMessageOutput
	require.NoError(t, json.Unmarshal(result.Output, &output))
	assert.Equal(t, "delivered", output.Status)
	assert.Equal(t, "agent-b", output.To)
	assert.NotEmpty(t, output.MessageID)

	// Check SSE event was emitted.
	select {
	case ev := <-ch:
		assert.Equal(t, agentic.EventAgentMessage, ev.Type)
		var data agentic.AgentMessageData
		require.NoError(t, json.Unmarshal(ev.Data, &data))
		assert.Equal(t, "agent-a", data.From)
		assert.Equal(t, "agent-b", data.To)
		assert.Equal(t, "hello from a", data.Content)
	default:
		t.Fatal("expected agent_message event")
	}

	// Verify message is in mailbox.
	msgs := mb.ReadUnread(sessionID, "agent-b")
	require.Len(t, msgs, 1)
	assert.Equal(t, "hello from a", msgs[0].Content)
}

func TestHandleSendMessage_EmptyMessage(t *testing.T) {
	mb := agentic.NewMailbox()
	ch := make(chan agentic.RunEvent, 10)

	input, _ := json.Marshal(map[string]string{
		"to":      "agent-b",
		"message": "",
	})

	result := agentic.HandleSendMessage(mb, uuid.New(), "agent-a", input, ch)
	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "requires a 'message'")
}

func TestHandleSendMessage_EmptyTo(t *testing.T) {
	mb := agentic.NewMailbox()
	ch := make(chan agentic.RunEvent, 10)

	input, _ := json.Marshal(map[string]string{
		"to":      "",
		"message": "hello",
	})

	result := agentic.HandleSendMessage(mb, uuid.New(), "agent-a", input, ch)
	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "requires a 'to'")
}

func TestHandleSendMessage_InvalidJSON(t *testing.T) {
	mb := agentic.NewMailbox()
	ch := make(chan agentic.RunEvent, 10)

	result := agentic.HandleSendMessage(mb, uuid.New(), "agent-a", json.RawMessage("not-json"), ch)
	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid send_message input")
}

func TestIsSendMessageToolCall(t *testing.T) {
	assert.True(t, agentic.IsSendMessageToolCall("send_message"))
	assert.False(t, agentic.IsSendMessageToolCall("agent"))
	assert.False(t, agentic.IsSendMessageToolCall("other_tool"))
}

func TestSendMessageTool_InToolSchema_SubAgent(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(
		&mockSkillLister{},
		nil,
		nil,
	).WithDepthLimits(1, 3) // depth 1 = sub-agent
	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	found := false
	for _, tool := range tools {
		if tool.Name == "send_message" {
			found = true
			assert.Contains(t, tool.Description, "Send a message")
		}
	}
	assert.True(t, found, "send_message should be available for sub-agents (depth > 0)")
}

func TestSendMessageTool_NotInRootAgent(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(
		&mockSkillLister{},
		nil,
		nil,
	).WithDepthLimits(0, 3) // depth 0 = root agent
	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	for _, tool := range tools {
		assert.NotEqual(t, "send_message", tool.Name, "send_message should not be available for root agent")
	}
}

func TestMailbox_ConcurrentAccess(t *testing.T) {
	mb := agentic.NewMailbox()
	sessionID := uuid.New()
	done := make(chan struct{})

	// Writer goroutine.
	go func() {
		for i := 0; i < 100; i++ {
			mb.Send(sessionID, "writer", "reader", "msg")
		}
		close(done)
	}()

	// Reader goroutine.
	for i := 0; i < 50; i++ {
		mb.ReadUnread(sessionID, "reader")
		mb.PendingCount(sessionID, "reader")
	}

	<-done
	assert.Equal(t, 100, mb.Len(sessionID))
}
