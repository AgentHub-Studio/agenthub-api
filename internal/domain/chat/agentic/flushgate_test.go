package agentic_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewFlushGate ---

func TestNewFlushGate(t *testing.T) {
	g := agentic.NewFlushGate[string]()
	assert.Equal(t, agentic.GateInactive, g.State())
	assert.False(t, g.IsActive())
}

// --- Start ---

func TestFlushGate_Start(t *testing.T) {
	g := agentic.NewFlushGate[int]()
	g.Start()
	assert.True(t, g.IsActive())
	assert.Equal(t, agentic.GateActive, g.State())
}

// --- Send ---

func TestFlushGate_Send_Active(t *testing.T) {
	g := agentic.NewFlushGate[string]()
	g.Start()

	queued := g.Send("hello")
	assert.True(t, queued)
	assert.Equal(t, 1, g.QueueLen())
}

func TestFlushGate_Send_Inactive(t *testing.T) {
	g := agentic.NewFlushGate[string]()

	queued := g.Send("hello")
	assert.False(t, queued, "should pass through when inactive")
	assert.Equal(t, 0, g.QueueLen())
}

func TestFlushGate_Send_MultipleItems(t *testing.T) {
	g := agentic.NewFlushGate[int]()
	g.Start()

	for i := 0; i < 5; i++ {
		assert.True(t, g.Send(i))
	}
	assert.Equal(t, 5, g.QueueLen())
}

// --- End ---

func TestFlushGate_End(t *testing.T) {
	g := agentic.NewFlushGate[string]()
	g.Start()
	g.Send("a")
	g.Send("b")
	g.Send("c")

	items := g.End()
	assert.Equal(t, []string{"a", "b", "c"}, items)
	assert.Equal(t, agentic.GateInactive, g.State())
	assert.Equal(t, 0, g.QueueLen())
}

func TestFlushGate_End_Empty(t *testing.T) {
	g := agentic.NewFlushGate[int]()
	g.Start()

	items := g.End()
	assert.Nil(t, items)
	assert.Equal(t, agentic.GateInactive, g.State())
}

func TestFlushGate_End_WithoutStart(t *testing.T) {
	g := agentic.NewFlushGate[int]()
	items := g.End()
	assert.Nil(t, items)
}

// --- Deactivate ---

func TestFlushGate_Deactivate(t *testing.T) {
	g := agentic.NewFlushGate[string]()
	g.Start()
	g.Send("item")

	g.Deactivate()
	assert.Equal(t, agentic.GateDeactivated, g.State())
	assert.Equal(t, 0, g.QueueLen())
	assert.False(t, g.IsActive())
}

func TestFlushGate_Deactivate_DiscardItems(t *testing.T) {
	g := agentic.NewFlushGate[int]()
	g.Start()
	g.Send(1)
	g.Send(2)

	g.Deactivate()
	// Items are gone — no way to retrieve
	assert.Equal(t, 0, g.QueueLen())
}

// --- Send after End ---

func TestFlushGate_SendAfterEnd(t *testing.T) {
	g := agentic.NewFlushGate[string]()
	g.Start()
	g.Send("before")
	g.End()

	queued := g.Send("after")
	assert.False(t, queued, "should pass through after end")
}

// --- Restart ---

func TestFlushGate_Restart(t *testing.T) {
	g := agentic.NewFlushGate[int]()

	// First cycle
	g.Start()
	g.Send(1)
	items := g.End()
	assert.Equal(t, []int{1}, items)

	// Second cycle
	g.Start()
	g.Send(2)
	g.Send(3)
	items = g.End()
	assert.Equal(t, []int{2, 3}, items)
}

// --- Start clears previous queue ---

func TestFlushGate_Start_ClearsQueue(t *testing.T) {
	g := agentic.NewFlushGate[string]()
	g.Start()
	g.Send("old")

	// Re-start without End
	g.Start()
	assert.Equal(t, 0, g.QueueLen())
}

// --- Concurrent access ---

func TestFlushGate_ConcurrentAccess(t *testing.T) {
	g := agentic.NewFlushGate[int]()
	g.Start()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			g.Send(v)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, g.QueueLen())

	items := g.End()
	assert.Len(t, items, 100)
	assert.False(t, g.IsActive())
}
