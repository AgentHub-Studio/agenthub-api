package agentic_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestQueryGuard_InitialState(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	assert.Equal(t, agentic.QueryIdle, g.Status())
	assert.False(t, g.IsActive())
	assert.Equal(t, int64(0), g.Generation())
}

func TestQueryGuard_Reserve_FromIdle(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	ok := g.Reserve()
	assert.True(t, ok)
	assert.Equal(t, agentic.QueryDispatching, g.Status())
	assert.True(t, g.IsActive())
}

func TestQueryGuard_Reserve_WhileDispatching(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	g.Reserve()
	ok := g.Reserve()
	assert.False(t, ok, "should not reserve while dispatching")
}

func TestQueryGuard_Reserve_WhileRunning(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	g.TryStart()
	ok := g.Reserve()
	assert.False(t, ok, "should not reserve while running")
}

func TestQueryGuard_CancelReservation(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	g.Reserve()
	g.CancelReservation()
	assert.Equal(t, agentic.QueryIdle, g.Status())
	assert.False(t, g.IsActive())
}

func TestQueryGuard_CancelReservation_NotDispatching(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	g.CancelReservation() // no-op
	assert.Equal(t, agentic.QueryIdle, g.Status())
}

func TestQueryGuard_TryStart_FromIdle(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	gen, ok := g.TryStart()
	assert.True(t, ok)
	assert.Equal(t, int64(1), gen)
	assert.Equal(t, agentic.QueryRunning, g.Status())
}

func TestQueryGuard_TryStart_FromDispatching(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	g.Reserve()
	gen, ok := g.TryStart()
	assert.True(t, ok)
	assert.Equal(t, int64(1), gen)
	assert.Equal(t, agentic.QueryRunning, g.Status())
}

func TestQueryGuard_TryStart_WhileRunning(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	g.TryStart()
	gen, ok := g.TryStart()
	assert.False(t, ok, "should not start while running")
	assert.Equal(t, int64(0), gen)
}

func TestQueryGuard_End_MatchingGeneration(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	gen, _ := g.TryStart()
	ok := g.End(gen)
	assert.True(t, ok)
	assert.Equal(t, agentic.QueryIdle, g.Status())
}

func TestQueryGuard_End_StaleGeneration(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	gen1, _ := g.TryStart()
	g.ForceEnd()
	ok := g.End(gen1)
	assert.False(t, ok, "stale generation should return false")
}

func TestQueryGuard_End_NotRunning(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	ok := g.End(1)
	assert.False(t, ok, "should return false when not running")
}

func TestQueryGuard_ForceEnd_FromRunning(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	gen1, _ := g.TryStart()
	g.ForceEnd()
	assert.Equal(t, agentic.QueryIdle, g.Status())
	assert.Greater(t, g.Generation(), gen1)
}

func TestQueryGuard_ForceEnd_FromDispatching(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	g.Reserve()
	g.ForceEnd()
	assert.Equal(t, agentic.QueryIdle, g.Status())
}

func TestQueryGuard_ForceEnd_FromIdle(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	gen := g.Generation()
	g.ForceEnd()
	assert.Equal(t, agentic.QueryIdle, g.Status())
	assert.Equal(t, gen, g.Generation(), "should not increment when already idle")
}

func TestQueryGuard_GenerationIncrementsOnStart(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	gen1, _ := g.TryStart()
	g.End(gen1)
	gen2, _ := g.TryStart()
	assert.Equal(t, gen1+1, gen2)
}

func TestQueryGuard_FullLifecycle(t *testing.T) {
	var transitions []agentic.QueryStatus
	g := agentic.NewQueryGuard(func(s agentic.QueryStatus) {
		transitions = append(transitions, s)
	})

	// Reserve → Start → End
	ok := g.Reserve()
	require.True(t, ok)
	gen, ok := g.TryStart()
	require.True(t, ok)
	ok = g.End(gen)
	require.True(t, ok)

	assert.Equal(t, []agentic.QueryStatus{
		agentic.QueryDispatching,
		agentic.QueryRunning,
		agentic.QueryIdle,
	}, transitions)
}

func TestQueryGuard_OnChangeCallback(t *testing.T) {
	callCount := 0
	g := agentic.NewQueryGuard(func(_ agentic.QueryStatus) {
		callCount++
	})
	g.TryStart()
	g.ForceEnd()
	assert.Equal(t, 2, callCount)
}

func TestQueryGuard_ConcurrentReserve(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	successCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if g.Reserve() {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, successCount, "only one goroutine should succeed in reserving")
}

func TestQueryGuard_ConcurrentTryStart(t *testing.T) {
	g := agentic.NewQueryGuard(nil)
	successCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := g.TryStart(); ok {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, successCount, "only one goroutine should succeed in starting")
}
