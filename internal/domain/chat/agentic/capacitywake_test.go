package agentic_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewCapacityWake ---

func TestNewCapacityWake(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())
	assert.NotNil(t, w)
	assert.False(t, w.IsDone())
	assert.Equal(t, 0, w.WakeCount())
}

// --- Wake ---

func TestCapacityWake_Wake(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())
	w.Wake()
	assert.Equal(t, 1, w.WakeCount())
}

func TestCapacityWake_Wake_Multiple(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())
	w.Wake()
	w.Wake()
	w.Wake()
	assert.Equal(t, 3, w.WakeCount())
}

// --- Sleep woken by Wake ---

func TestCapacityWake_Sleep_WokenByWake(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())

	go func() {
		time.Sleep(20 * time.Millisecond)
		w.Wake()
	}()

	woken := w.Sleep()
	assert.True(t, woken, "should be woken by Wake()")
}

// --- Sleep woken by context cancel ---

func TestCapacityWake_Sleep_WokenByCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := agentic.NewCapacityWake(ctx)

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	woken := w.Sleep()
	assert.False(t, woken, "should return false on shutdown")
}

// --- IsDone ---

func TestCapacityWake_IsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := agentic.NewCapacityWake(ctx)

	assert.False(t, w.IsDone())
	cancel()
	assert.True(t, w.IsDone())
}

// --- WaitCtx ---

func TestCapacityWake_WaitCtx_WokenByWake(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())

	waitCtx, waitCancel := w.WaitCtx()
	defer waitCancel()

	go func() {
		time.Sleep(20 * time.Millisecond)
		w.Wake()
	}()

	select {
	case <-waitCtx.Done():
		// Expected — woken by Wake()
	case <-time.After(2 * time.Second):
		t.Fatal("WaitCtx did not wake up")
	}
}

func TestCapacityWake_WaitCtx_WokenByShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := agentic.NewCapacityWake(ctx)

	waitCtx, waitCancel := w.WaitCtx()
	defer waitCancel()

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	select {
	case <-waitCtx.Done():
		// Expected — woken by shutdown
	case <-time.After(2 * time.Second):
		t.Fatal("WaitCtx did not respond to shutdown")
	}
}

func TestCapacityWake_WaitCtx_CallerCancel(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())

	_, waitCancel := w.WaitCtx()
	waitCancel() // caller cancels immediately

	// Should not leak goroutines or panic
	time.Sleep(10 * time.Millisecond)
}

// --- Concurrent wake ---

func TestCapacityWake_ConcurrentWake(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Wake()
		}()
	}
	wg.Wait()

	assert.Equal(t, 20, w.WakeCount())
}

// --- Sleep then Wake pattern ---

func TestCapacityWake_RepeatedSleepWake(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())

	for i := 0; i < 3; i++ {
		go func() {
			time.Sleep(10 * time.Millisecond)
			w.Wake()
		}()

		woken := w.Sleep()
		assert.True(t, woken)
	}

	assert.Equal(t, 3, w.WakeCount())
}

// --- Wake before Sleep ---

func TestCapacityWake_WakeBeforeSleep(t *testing.T) {
	w := agentic.NewCapacityWake(context.Background())

	w.Wake() // wake before sleep

	done := make(chan bool, 1)
	go func() {
		woken := w.Sleep()
		done <- woken
	}()

	select {
	case woken := <-done:
		assert.True(t, woken)
	case <-time.After(time.Second):
		t.Fatal("Sleep should return immediately when pre-woken")
	}
}
