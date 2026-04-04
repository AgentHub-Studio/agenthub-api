package agentic_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- MessageSource ---

func TestMessageSource_Constants(t *testing.T) {
	assert.Equal(t, agentic.MessageSource("user"), agentic.SourceUser)
	assert.Equal(t, agentic.MessageSource("teammate"), agentic.SourceTeammate)
	assert.Equal(t, agentic.MessageSource("system"), agentic.SourceSystem)
	assert.Equal(t, agentic.MessageSource("tick"), agentic.SourceTick)
	assert.Equal(t, agentic.MessageSource("task"), agentic.SourceTask)
}

// --- NewMailMessage ---

func TestNewMailMessage(t *testing.T) {
	msg := agentic.NewMailMessage(agentic.SourceUser, "hello")
	assert.Equal(t, agentic.SourceUser, msg.Source)
	assert.Equal(t, "hello", msg.Content)
	assert.NotEmpty(t, msg.ID)
	assert.False(t, msg.Timestamp.IsZero())
}

func TestNewMailMessage_UniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		msg := agentic.NewMailMessage(agentic.SourceSystem, "test")
		assert.False(t, ids[msg.ID], "duplicate ID generated")
		ids[msg.ID] = true
	}
}

// --- Send / Poll ---

func TestMailbox_SendAndPoll(t *testing.T) {
	mb := agentic.NewMailbox()
	msg := agentic.NewMailMessage(agentic.SourceUser, "hi")
	mb.Send(msg)

	got, ok := mb.Poll()
	assert.True(t, ok)
	assert.Equal(t, "hi", got.Content)
}

func TestMailbox_PollEmpty(t *testing.T) {
	mb := agentic.NewMailbox()
	_, ok := mb.Poll()
	assert.False(t, ok)
}

func TestMailbox_FIFO(t *testing.T) {
	mb := agentic.NewMailbox()
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "first"))
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "second"))
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "third"))

	msg1, _ := mb.Poll()
	msg2, _ := mb.Poll()
	msg3, _ := mb.Poll()
	assert.Equal(t, "first", msg1.Content)
	assert.Equal(t, "second", msg2.Content)
	assert.Equal(t, "third", msg3.Content)
}

// --- PollAll ---

func TestMailbox_PollAll(t *testing.T) {
	mb := agentic.NewMailbox()
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "a"))
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "b"))

	msgs := mb.PollAll()
	assert.Len(t, msgs, 2)
	assert.Equal(t, "a", msgs[0].Content)
	assert.Equal(t, "b", msgs[1].Content)

	// Queue should be empty now.
	assert.Nil(t, mb.PollAll())
}

func TestMailbox_PollAllEmpty(t *testing.T) {
	mb := agentic.NewMailbox()
	assert.Nil(t, mb.PollAll())
}

// --- Pending ---

func TestMailbox_Pending(t *testing.T) {
	mb := agentic.NewMailbox()
	assert.Equal(t, 0, mb.Pending())

	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "a"))
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "b"))
	assert.Equal(t, 2, mb.Pending())

	mb.Poll()
	assert.Equal(t, 1, mb.Pending())
}

// --- Receive (blocking) ---

func TestMailbox_ReceiveImmediate(t *testing.T) {
	mb := agentic.NewMailbox()
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "ready"))

	done := make(chan struct{})
	msg, ok := mb.Receive(done)
	assert.True(t, ok)
	assert.Equal(t, "ready", msg.Content)
}

func TestMailbox_ReceiveBlocks(t *testing.T) {
	mb := agentic.NewMailbox()
	done := make(chan struct{})

	var received agentic.MailMessage
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		received, _ = mb.Receive(done)
	}()

	// Give goroutine time to block.
	time.Sleep(20 * time.Millisecond)
	mb.Send(agentic.NewMailMessage(agentic.SourceTeammate, "delayed"))

	wg.Wait()
	assert.Equal(t, "delayed", received.Content)
}

func TestMailbox_ReceiveDoneCancels(t *testing.T) {
	mb := agentic.NewMailbox()
	done := make(chan struct{})

	var ok bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, ok = mb.Receive(done)
	}()

	time.Sleep(20 * time.Millisecond)
	close(done)
	wg.Wait()
	assert.False(t, ok)
}

func TestMailbox_ReceiveWaiterDeferral(t *testing.T) {
	mb := agentic.NewMailbox()
	done := make(chan struct{})

	// Two waiters.
	var msg1, msg2 agentic.MailMessage
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		msg1, _ = mb.Receive(done)
	}()
	go func() {
		defer wg.Done()
		msg2, _ = mb.Receive(done)
	}()

	time.Sleep(30 * time.Millisecond)

	// Send two messages — both waiters should be served directly.
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "one"))
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "two"))
	wg.Wait()

	// Both should have received a message; nothing in queue.
	assert.NotEmpty(t, msg1.Content)
	assert.NotEmpty(t, msg2.Content)
	assert.Equal(t, 0, mb.Pending())
}

// --- Subscribe ---

func TestMailbox_Subscribe(t *testing.T) {
	mb := agentic.NewMailbox()

	var received []string
	mb.Subscribe(func(msg agentic.MailMessage) {
		received = append(received, msg.Content)
	})

	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "first"))
	mb.Send(agentic.NewMailMessage(agentic.SourceSystem, "second"))

	assert.Equal(t, []string{"first", "second"}, received)
}

func TestMailbox_SubscribeUnsubscribe(t *testing.T) {
	mb := agentic.NewMailbox()

	var count int
	unsub := mb.Subscribe(func(msg agentic.MailMessage) {
		count++
	})

	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "a"))
	assert.Equal(t, 1, count)

	unsub()

	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "b"))
	assert.Equal(t, 1, count, "should not increment after unsubscribe")
}

func TestMailbox_MultipleSubscribers(t *testing.T) {
	mb := agentic.NewMailbox()

	var count1, count2 int
	mb.Subscribe(func(msg agentic.MailMessage) { count1++ })
	mb.Subscribe(func(msg agentic.MailMessage) { count2++ })

	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "msg"))
	assert.Equal(t, 1, count1)
	assert.Equal(t, 1, count2)
}

// --- Close ---

func TestMailbox_Close(t *testing.T) {
	mb := agentic.NewMailbox()
	mb.Close()

	// Send after close should be ignored.
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "ignored"))
	assert.Equal(t, 0, mb.Pending())
}

func TestMailbox_CloseUnblocksWaiters(t *testing.T) {
	mb := agentic.NewMailbox()
	done := make(chan struct{})

	var ok bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, ok = mb.Receive(done)
	}()

	time.Sleep(20 * time.Millisecond)
	mb.Close()
	wg.Wait()
	assert.False(t, ok)
}

// --- FilterBySource ---

func TestMailbox_FilterBySource(t *testing.T) {
	mb := agentic.NewMailbox()
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "user msg"))
	mb.Send(agentic.NewMailMessage(agentic.SourceSystem, "sys msg"))
	mb.Send(agentic.NewMailMessage(agentic.SourceUser, "user msg 2"))

	userMsgs := mb.FilterBySource(agentic.SourceUser)
	assert.Len(t, userMsgs, 2)

	sysMsgs := mb.FilterBySource(agentic.SourceSystem)
	assert.Len(t, sysMsgs, 1)

	taskMsgs := mb.FilterBySource(agentic.SourceTask)
	assert.Len(t, taskMsgs, 0)
}

// --- Send auto-fills ---

func TestMailbox_SendAutoFillsTimestamp(t *testing.T) {
	mb := agentic.NewMailbox()
	mb.Send(agentic.MailMessage{Source: agentic.SourceUser, Content: "bare"})

	msg, ok := mb.Poll()
	require.True(t, ok)
	assert.False(t, msg.Timestamp.IsZero())
	assert.NotEmpty(t, msg.ID)
}

// --- Metadata ---

func TestMailMessage_Metadata(t *testing.T) {
	msg := agentic.MailMessage{
		Source:   agentic.SourceTask,
		Content:  "done",
		Metadata: map[string]any{"taskId": "t-123", "exitCode": 0},
	}
	assert.Equal(t, "t-123", msg.Metadata["taskId"])
	assert.Equal(t, 0, msg.Metadata["exitCode"])
}

// --- Concurrency ---

func TestMailbox_ConcurrentSendPoll(t *testing.T) {
	mb := agentic.NewMailbox()
	const n = 100

	var wg sync.WaitGroup
	// Send n messages concurrently.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mb.Send(agentic.NewMailMessage(agentic.SourceUser, "msg"))
		}()
	}
	wg.Wait()

	// Poll all.
	var polled int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := mb.Poll(); ok {
				atomic.AddInt32(&polled, 1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(n), polled)
}
