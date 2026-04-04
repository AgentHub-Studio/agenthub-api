package agentic_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- QueuePriority ---

func TestQueuePriority_String(t *testing.T) {
	assert.Equal(t, "now", agentic.PriorityNow.String())
	assert.Equal(t, "next", agentic.PriorityNext.String())
	assert.Equal(t, "later", agentic.PriorityLater.String())
}

func TestQueuePriority_Order(t *testing.T) {
	assert.Less(t, int(agentic.PriorityNow), int(agentic.PriorityNext))
	assert.Less(t, int(agentic.PriorityNext), int(agentic.PriorityLater))
}

// --- NewCommandQueue ---

func TestNewCommandQueue_Empty(t *testing.T) {
	q := agentic.NewCommandQueue()
	assert.True(t, q.IsEmpty())
	assert.Equal(t, 0, q.Len())
}

// --- Enqueue / Dequeue ---

func TestCommandQueue_EnqueueDequeue_FIFO(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "a", Priority: agentic.PriorityNext, Content: "first"})
	q.Enqueue(agentic.QueuedCommand{ID: "b", Priority: agentic.PriorityNext, Content: "second"})

	cmd, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "a", cmd.ID)

	cmd, ok = q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "b", cmd.ID)

	_, ok = q.Dequeue()
	assert.False(t, ok)
}

func TestCommandQueue_Dequeue_PriorityOrder(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "later", Priority: agentic.PriorityLater})
	q.Enqueue(agentic.QueuedCommand{ID: "now", Priority: agentic.PriorityNow})
	q.Enqueue(agentic.QueuedCommand{ID: "next", Priority: agentic.PriorityNext})

	cmd, _ := q.Dequeue()
	assert.Equal(t, "now", cmd.ID)

	cmd, _ = q.Dequeue()
	assert.Equal(t, "next", cmd.ID)

	cmd, _ = q.Dequeue()
	assert.Equal(t, "later", cmd.ID)
}

func TestCommandQueue_Dequeue_PriorityThenFIFO(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "now-1", Priority: agentic.PriorityNow})
	q.Enqueue(agentic.QueuedCommand{ID: "now-2", Priority: agentic.PriorityNow})
	q.Enqueue(agentic.QueuedCommand{ID: "later-1", Priority: agentic.PriorityLater})

	// First two should be now-1 then now-2 (FIFO within priority).
	cmd, _ := q.Dequeue()
	assert.Equal(t, "now-1", cmd.ID)

	cmd, _ = q.Dequeue()
	assert.Equal(t, "now-2", cmd.ID)

	cmd, _ = q.Dequeue()
	assert.Equal(t, "later-1", cmd.ID)
}

// --- DequeueFiltered ---

func TestCommandQueue_DequeueFiltered(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "a", Priority: agentic.PriorityNow, Source: "user"})
	q.Enqueue(agentic.QueuedCommand{ID: "b", Priority: agentic.PriorityNow, Source: "system"})

	cmd, ok := q.DequeueFiltered(func(c agentic.QueuedCommand) bool {
		return c.Source == "system"
	})
	require.True(t, ok)
	assert.Equal(t, "b", cmd.ID)
	assert.Equal(t, 1, q.Len())
}

func TestCommandQueue_DequeueFiltered_NoMatch(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "a", Source: "user"})

	_, ok := q.DequeueFiltered(func(c agentic.QueuedCommand) bool {
		return c.Source == "nope"
	})
	assert.False(t, ok)
	assert.Equal(t, 1, q.Len())
}

// --- DequeueAll ---

func TestCommandQueue_DequeueAll(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "later", Priority: agentic.PriorityLater})
	q.Enqueue(agentic.QueuedCommand{ID: "now", Priority: agentic.PriorityNow})

	all := q.DequeueAll()
	require.Len(t, all, 2)
	assert.Equal(t, "now", all[0].ID)
	assert.Equal(t, "later", all[1].ID)
	assert.True(t, q.IsEmpty())
}

func TestCommandQueue_DequeueAll_Empty(t *testing.T) {
	q := agentic.NewCommandQueue()
	assert.Nil(t, q.DequeueAll())
}

// --- DequeueAllMatching ---

func TestCommandQueue_DequeueAllMatching(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "a", Source: "user"})
	q.Enqueue(agentic.QueuedCommand{ID: "b", Source: "system"})
	q.Enqueue(agentic.QueuedCommand{ID: "c", Source: "user"})

	removed := q.DequeueAllMatching(func(c agentic.QueuedCommand) bool {
		return c.Source == "user"
	})
	assert.Len(t, removed, 2)
	assert.Equal(t, 1, q.Len())
}

// --- Peek ---

func TestCommandQueue_Peek(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "later", Priority: agentic.PriorityLater})
	q.Enqueue(agentic.QueuedCommand{ID: "now", Priority: agentic.PriorityNow})

	cmd, ok := q.Peek()
	require.True(t, ok)
	assert.Equal(t, "now", cmd.ID)
	assert.Equal(t, 2, q.Len(), "peek should not remove")
}

func TestCommandQueue_Peek_Empty(t *testing.T) {
	q := agentic.NewCommandQueue()
	_, ok := q.Peek()
	assert.False(t, ok)
}

func TestCommandQueue_PeekFiltered(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "a", Priority: agentic.PriorityNow, Source: "x"})
	q.Enqueue(agentic.QueuedCommand{ID: "b", Priority: agentic.PriorityLater, Source: "y"})

	cmd, ok := q.PeekFiltered(func(c agentic.QueuedCommand) bool {
		return c.Source == "y"
	})
	require.True(t, ok)
	assert.Equal(t, "b", cmd.ID)
}

// --- GetByMaxPriority ---

func TestCommandQueue_GetByMaxPriority(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "now", Priority: agentic.PriorityNow})
	q.Enqueue(agentic.QueuedCommand{ID: "next", Priority: agentic.PriorityNext})
	q.Enqueue(agentic.QueuedCommand{ID: "later", Priority: agentic.PriorityLater})

	urgent := q.GetByMaxPriority(agentic.PriorityNext)
	assert.Len(t, urgent, 2, "should include now + next")
}

// --- Snapshot ---

func TestCommandQueue_Snapshot_SortedCopy(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "later", Priority: agentic.PriorityLater})
	q.Enqueue(agentic.QueuedCommand{ID: "now", Priority: agentic.PriorityNow})

	snap := q.Snapshot()
	require.Len(t, snap, 2)
	assert.Equal(t, "now", snap[0].ID)
	assert.Equal(t, "later", snap[1].ID)

	// Verify it's a copy.
	assert.Equal(t, 2, q.Len())
}

// --- Clear ---

func TestCommandQueue_Clear(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "a"})
	q.Enqueue(agentic.QueuedCommand{ID: "b"})
	q.Clear()
	assert.True(t, q.IsEmpty())
}

// --- Subscribe ---

func TestCommandQueue_Subscribe_NotifiedOnEnqueue(t *testing.T) {
	q := agentic.NewCommandQueue()
	var count int
	q.Subscribe(func() { count++ })

	q.Enqueue(agentic.QueuedCommand{ID: "a"})
	assert.Equal(t, 1, count)

	q.Enqueue(agentic.QueuedCommand{ID: "b"})
	assert.Equal(t, 2, count)
}

func TestCommandQueue_Subscribe_Unsubscribe(t *testing.T) {
	q := agentic.NewCommandQueue()
	var count int
	unsub := q.Subscribe(func() { count++ })

	q.Enqueue(agentic.QueuedCommand{ID: "a"})
	assert.Equal(t, 1, count)

	unsub()
	q.Enqueue(agentic.QueuedCommand{ID: "b"})
	assert.Equal(t, 1, count, "should not fire after unsubscribe")
}

func TestCommandQueue_Subscribe_NotifiedOnDequeue(t *testing.T) {
	q := agentic.NewCommandQueue()
	q.Enqueue(agentic.QueuedCommand{ID: "a"})

	var count int
	q.Subscribe(func() { count++ })

	q.Dequeue()
	assert.Equal(t, 1, count)
}

// --- Concurrent access ---

func TestCommandQueue_ConcurrentAccess(t *testing.T) {
	q := agentic.NewCommandQueue()
	var wg sync.WaitGroup
	var dequeued int64

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q.Enqueue(agentic.QueuedCommand{
				ID:       fmt.Sprintf("cmd-%d", i),
				Priority: agentic.QueuePriority(i % 3),
			})
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, q.Len())

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := q.Dequeue(); ok {
				atomic.AddInt64(&dequeued, 1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(100), dequeued)
	assert.True(t, q.IsEmpty())
}

// --- Command fields ---

func TestQueuedCommand_Fields(t *testing.T) {
	cmd := agentic.QueuedCommand{
		ID:       "test-1",
		Priority: agentic.PriorityNow,
		Content:  "do something",
		Source:   "user",
		Metadata: map[string]interface{}{"key": "val"},
		Editable: true,
		Visible:  true,
	}
	assert.Equal(t, "test-1", cmd.ID)
	assert.True(t, cmd.Editable)
	assert.True(t, cmd.Visible)
}
