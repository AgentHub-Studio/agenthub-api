package agentic_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewCleanupRegistry ---

func TestNewCleanupRegistry(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	assert.Equal(t, 0, r.Count())
	assert.False(t, r.IsClosed())
}

// --- Register ---

func TestCleanupRegistry_Register(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	r.Register(func(_ context.Context) error { return nil })
	r.Register(func(_ context.Context) error { return nil })
	assert.Equal(t, 2, r.Count())
}

func TestCleanupRegistry_Register_Unregister(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	unsub := r.Register(func(_ context.Context) error { return nil })
	assert.Equal(t, 1, r.Count())

	unsub()
	assert.Equal(t, 0, r.Count())
}

func TestCleanupRegistry_Register_AfterClosed(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	r.RunAll(context.Background())

	unsub := r.Register(func(_ context.Context) error { return nil })
	assert.Equal(t, 0, r.Count(), "should not register after close")
	unsub() // should not panic
}

// --- RunAll ---

func TestCleanupRegistry_RunAll_ExecutesAll(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	var count int32

	for i := 0; i < 5; i++ {
		r.Register(func(_ context.Context) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
	}

	errs := r.RunAll(context.Background())
	assert.Empty(t, errs)
	assert.Equal(t, int32(5), count)
	assert.True(t, r.IsClosed())
	assert.Equal(t, 0, r.Count())
}

func TestCleanupRegistry_RunAll_Parallel(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	var running int32
	var maxRunning int32

	for i := 0; i < 10; i++ {
		r.Register(func(_ context.Context) error {
			cur := atomic.AddInt32(&running, 1)
			for {
				old := atomic.LoadInt32(&maxRunning)
				if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&running, -1)
			return nil
		})
	}

	r.RunAll(context.Background())
	assert.Greater(t, atomic.LoadInt32(&maxRunning), int32(1), "should run handlers in parallel")
}

func TestCleanupRegistry_RunAll_CollectsErrors(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	r.Register(func(_ context.Context) error { return nil })
	r.Register(func(_ context.Context) error { return fmt.Errorf("cleanup failed") })
	r.Register(func(_ context.Context) error { return nil })

	errs := r.RunAll(context.Background())
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "cleanup failed")
}

func TestCleanupRegistry_RunAll_Empty(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	errs := r.RunAll(context.Background())
	assert.Nil(t, errs)
}

func TestCleanupRegistry_RunAll_ContextPassed(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("key"), "value")

	var received string
	r.Register(func(ctx context.Context) error {
		received = ctx.Value(ctxKey("key")).(string)
		return nil
	})

	r.RunAll(ctx)
	assert.Equal(t, "value", received)
}

// --- Reset ---

func TestCleanupRegistry_Reset(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	r.Register(func(_ context.Context) error { return nil })
	r.RunAll(context.Background())

	assert.True(t, r.IsClosed())

	r.Reset()
	assert.False(t, r.IsClosed())
	assert.Equal(t, 0, r.Count())

	// Can register again after reset.
	r.Register(func(_ context.Context) error { return nil })
	assert.Equal(t, 1, r.Count())
}

// --- Concurrent registration ---

func TestCleanupRegistry_ConcurrentRegister(t *testing.T) {
	r := agentic.NewCleanupRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Register(func(_ context.Context) error { return nil })
		}()
	}
	wg.Wait()

	assert.Equal(t, 50, r.Count())
}
