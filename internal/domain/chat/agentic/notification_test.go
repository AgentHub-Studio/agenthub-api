package agentic_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Priority constants ---

func TestNotificationPriority_Order(t *testing.T) {
	assert.Less(t, int(agentic.NotifyImmediate), int(agentic.NotifyHigh))
	assert.Less(t, int(agentic.NotifyHigh), int(agentic.NotifyMedium))
	assert.Less(t, int(agentic.NotifyMedium), int(agentic.NotifyLow))
}

// --- Notification ---

func TestNotification_Timeout_Default(t *testing.T) {
	n := agentic.Notification{Key: "k", Priority: agentic.NotifyMedium}
	assert.Equal(t, 8*time.Second, n.Timeout())
}

func TestNotification_Timeout_Custom(t *testing.T) {
	n := agentic.Notification{Key: "k", TimeoutMs: 2000}
	assert.Equal(t, 2*time.Second, n.Timeout())
}

// --- NotificationQueue basic ---

func TestNotificationQueue_AddAndCurrent(t *testing.T) {
	var shown []string
	q := agentic.NewNotificationQueue(
		func(n agentic.Notification) { shown = append(shown, n.Key) },
		nil,
	)

	q.Add(agentic.Notification{Key: "a", Text: "hello", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	cur := q.Current()
	require.NotNil(t, cur)
	assert.Equal(t, "a", cur.Key)
	assert.Equal(t, "hello", cur.Text)
	assert.Equal(t, []string{"a"}, shown)
}

func TestNotificationQueue_Empty(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	assert.Nil(t, q.Current())
	assert.Equal(t, 0, q.QueueLen())
}

func TestNotificationQueue_SecondQueued(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "a", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "b", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	assert.Equal(t, "a", q.Current().Key)
	assert.Equal(t, 1, q.QueueLen())
}

// --- Priority ordering ---

func TestNotificationQueue_PriorityOrdering(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "first", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "low", Priority: agentic.NotifyLow, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "high", Priority: agentic.NotifyHigh, TimeoutMs: 5000})

	// "first" is current; remove it to see priority order.
	q.Remove("first")
	assert.Equal(t, "high", q.Current().Key, "high priority should come before low")
}

// --- Immediate priority ---

func TestNotificationQueue_ImmediatePreempts(t *testing.T) {
	var shown []string
	q := agentic.NewNotificationQueue(
		func(n agentic.Notification) { shown = append(shown, n.Key) },
		nil,
	)

	q.Add(agentic.Notification{Key: "normal", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	assert.Equal(t, "normal", q.Current().Key)

	q.Add(agentic.Notification{Key: "urgent", Priority: agentic.NotifyImmediate, TimeoutMs: 5000})
	assert.Equal(t, "urgent", q.Current().Key)
	assert.Equal(t, []string{"normal", "urgent"}, shown)
}

func TestNotificationQueue_ImmediateRequeuesNormal(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "normal", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "urgent", Priority: agentic.NotifyImmediate, TimeoutMs: 5000})

	// "normal" should be re-queued.
	assert.Equal(t, 1, q.QueueLen())
	q.Remove("urgent")
	assert.Equal(t, "normal", q.Current().Key)
}

// --- Remove ---

func TestNotificationQueue_RemoveCurrent(t *testing.T) {
	var cleared []string
	q := agentic.NewNotificationQueue(nil, func(key string) { cleared = append(cleared, key) })
	q.Add(agentic.Notification{Key: "a", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "b", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	q.Remove("a")
	assert.Equal(t, "b", q.Current().Key)
	assert.Contains(t, cleared, "a")
}

func TestNotificationQueue_RemoveQueued(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "a", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "b", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	q.Remove("b")
	assert.Equal(t, 0, q.QueueLen())
}

func TestNotificationQueue_RemoveNonexistent(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Remove("nonexistent") // should not panic
}

// --- Invalidation ---

func TestNotificationQueue_InvalidatesQueued(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "a", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "old1", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "old2", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	q.Add(agentic.Notification{
		Key:         "new",
		Priority:    agentic.NotifyMedium,
		Invalidates: []string{"old1", "old2"},
		TimeoutMs:   5000,
	})

	assert.Equal(t, "a", q.Current().Key)
	// old1 and old2 should be removed; only "new" remains in queue.
	assert.Equal(t, 1, q.QueueLen())
}

func TestNotificationQueue_InvalidatesCurrent(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "current", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "queued", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	q.Add(agentic.Notification{
		Key:         "replacement",
		Priority:    agentic.NotifyMedium,
		Invalidates: []string{"current"},
		TimeoutMs:   5000,
	})

	// "current" invalidated; "queued" or "replacement" should become current.
	cur := q.Current()
	require.NotNil(t, cur)
	assert.NotEqual(t, "current", cur.Key)
}

// --- Fold ---

func TestNotificationQueue_FoldMergesCurrent(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.RegisterFold("counter", func(acc, inc agentic.Notification) agentic.Notification {
		count, _ := acc.Metadata["count"].(int)
		acc.Metadata["count"] = count + 1
		return acc
	})

	q.Add(agentic.Notification{
		Key:      "counter",
		Text:     "count: 1",
		Priority: agentic.NotifyMedium,
		Metadata: map[string]any{"count": 1},
		TimeoutMs: 5000,
	})

	q.Add(agentic.Notification{
		Key:      "counter",
		Text:     "count: 2",
		Priority: agentic.NotifyMedium,
		Metadata: map[string]any{},
		TimeoutMs: 5000,
	})

	cur := q.Current()
	require.NotNil(t, cur)
	assert.Equal(t, 2, cur.Metadata["count"])
	assert.Equal(t, 0, q.QueueLen(), "folded, no duplicate in queue")
}

func TestNotificationQueue_FoldMergesQueued(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.RegisterFold("counter", func(acc, inc agentic.Notification) agentic.Notification {
		count, _ := acc.Metadata["count"].(int)
		acc.Metadata["count"] = count + 1
		return acc
	})

	// Fill current with something else.
	q.Add(agentic.Notification{Key: "other", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	q.Add(agentic.Notification{
		Key:      "counter",
		Priority: agentic.NotifyMedium,
		Metadata: map[string]any{"count": 1},
		TimeoutMs: 5000,
	})
	q.Add(agentic.Notification{
		Key:      "counter",
		Priority: agentic.NotifyMedium,
		Metadata: map[string]any{},
		TimeoutMs: 5000,
	})

	assert.Equal(t, 1, q.QueueLen(), "folded into one queued item")
}

// --- Duplicate key prevention ---

func TestNotificationQueue_NoDuplicateKeys(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "block", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "dup", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "dup", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	assert.Equal(t, 1, q.QueueLen(), "duplicate should be ignored")
}

// --- Clear ---

func TestNotificationQueue_Clear(t *testing.T) {
	q := agentic.NewNotificationQueue(nil, nil)
	q.Add(agentic.Notification{Key: "a", Priority: agentic.NotifyMedium, TimeoutMs: 5000})
	q.Add(agentic.Notification{Key: "b", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	q.Clear()
	assert.Nil(t, q.Current())
	assert.Equal(t, 0, q.QueueLen())
}

// --- Timer auto-advance ---

func TestNotificationQueue_TimerAdvances(t *testing.T) {
	var mu sync.Mutex
	var shown []string

	q := agentic.NewNotificationQueue(
		func(n agentic.Notification) {
			mu.Lock()
			shown = append(shown, n.Key)
			mu.Unlock()
		},
		nil,
	)

	q.Add(agentic.Notification{Key: "first", Priority: agentic.NotifyMedium, TimeoutMs: 50})
	q.Add(agentic.Notification{Key: "second", Priority: agentic.NotifyMedium, TimeoutMs: 5000})

	time.Sleep(150 * time.Millisecond) // wait for first to expire

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"first", "second"}, shown)
}

// --- Metadata ---

func TestNotification_Metadata(t *testing.T) {
	n := agentic.Notification{
		Key:      "k",
		Priority: agentic.NotifyMedium,
		Metadata: map[string]any{"type": "build", "count": 5},
	}
	assert.Equal(t, "build", n.Metadata["type"])
	assert.Equal(t, 5, n.Metadata["count"])
}
