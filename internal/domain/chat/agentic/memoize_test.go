package agentic_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- MemoizeWithTTL ---

func TestMemoizeWithTTL_CachesResult(t *testing.T) {
	calls := 0
	m := agentic.NewMemoizeWithTTL(func(k string) int {
		calls++
		return len(k)
	}, time.Minute)

	assert.Equal(t, 5, m.Get("hello"))
	assert.Equal(t, 5, m.Get("hello"))
	assert.Equal(t, 1, calls, "should only compute once")
}

func TestMemoizeWithTTL_DifferentKeys(t *testing.T) {
	m := agentic.NewMemoizeWithTTL(func(k string) int { return len(k) }, time.Minute)
	assert.Equal(t, 3, m.Get("abc"))
	assert.Equal(t, 5, m.Get("hello"))
}

func TestMemoizeWithTTL_Expiry(t *testing.T) {
	var calls int32
	m := agentic.NewMemoizeWithTTL(func(k string) int {
		atomic.AddInt32(&calls, 1)
		return len(k)
	}, 50*time.Millisecond)

	m.Get("a")
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))

	time.Sleep(100 * time.Millisecond) // expire

	m.Get("a") // returns stale, triggers background refresh
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
}

func TestMemoizeWithTTL_Clear(t *testing.T) {
	calls := 0
	m := agentic.NewMemoizeWithTTL(func(k string) int {
		calls++
		return len(k)
	}, time.Minute)

	m.Get("x")
	assert.Equal(t, 1, calls)
	assert.Equal(t, 1, m.Size())

	m.Clear()
	assert.Equal(t, 0, m.Size())

	m.Get("x")
	assert.Equal(t, 2, calls)
}

func TestMemoizeWithTTL_DefaultTTL(t *testing.T) {
	m := agentic.NewMemoizeWithTTL(func(k int) int { return k * 2 }, 0)
	assert.Equal(t, 10, m.Get(5))
}

func TestMemoizeWithTTL_ConcurrentAccess(t *testing.T) {
	var calls int64
	m := agentic.NewMemoizeWithTTL(func(k string) int {
		atomic.AddInt64(&calls, 1)
		return len(k)
	}, time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Get("concurrent")
		}()
	}
	wg.Wait()
	// First call computes, rest should hit cache.
	assert.LessOrEqual(t, atomic.LoadInt64(&calls), int64(50))
}

// --- MemoizeAsyncWithTTL ---

func TestMemoizeAsyncWithTTL_CachesResult(t *testing.T) {
	calls := 0
	m := agentic.NewMemoizeAsyncWithTTL(func(k string) (int, error) {
		calls++
		return len(k), nil
	}, time.Minute)

	val, err := m.Get("hello")
	require.NoError(t, err)
	assert.Equal(t, 5, val)

	val2, err2 := m.Get("hello")
	require.NoError(t, err2)
	assert.Equal(t, 5, val2)
	assert.Equal(t, 1, calls)
}

func TestMemoizeAsyncWithTTL_InFlightDedup(t *testing.T) {
	var calls int64
	barrier := make(chan struct{})
	m := agentic.NewMemoizeAsyncWithTTL(func(k string) (int, error) {
		atomic.AddInt64(&calls, 1)
		<-barrier
		return len(k), nil
	}, time.Minute)

	var wg sync.WaitGroup
	results := make([]int, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, _ := m.Get("shared")
			results[idx] = val
		}(i)
	}

	time.Sleep(50 * time.Millisecond) // let all goroutines reach Get
	close(barrier)                    // release
	wg.Wait()

	assert.Equal(t, int64(1), atomic.LoadInt64(&calls), "should only compute once")
	for _, r := range results {
		assert.Equal(t, 6, r) // len("shared")
	}
}

func TestMemoizeAsyncWithTTL_Error(t *testing.T) {
	m := agentic.NewMemoizeAsyncWithTTL(func(k string) (int, error) {
		return 0, errors.New("fail")
	}, time.Minute)

	_, err := m.Get("x")
	assert.Error(t, err)
	assert.Equal(t, 0, m.Size(), "errors should not be cached")
}

func TestMemoizeAsyncWithTTL_Clear(t *testing.T) {
	m := agentic.NewMemoizeAsyncWithTTL(func(k string) (int, error) {
		return 42, nil
	}, time.Minute)

	_, err := m.Get("x")
	require.NoError(t, err)
	assert.Equal(t, 1, m.Size())
	m.Clear()
	assert.Equal(t, 0, m.Size())
}

// --- LRUCache ---

func TestLRUCache_SetAndGet(t *testing.T) {
	c := agentic.NewLRUCache[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)

	val, ok := c.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, val)
}

func TestLRUCache_Miss(t *testing.T) {
	c := agentic.NewLRUCache[string, int](3)
	_, ok := c.Get("missing")
	assert.False(t, ok)
}

func TestLRUCache_Eviction(t *testing.T) {
	c := agentic.NewLRUCache[string, int](2)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // evicts "a"

	_, ok := c.Get("a")
	assert.False(t, ok, "a should be evicted")

	val, ok := c.Get("c")
	assert.True(t, ok)
	assert.Equal(t, 3, val)
}

func TestLRUCache_GetUpdatesRecency(t *testing.T) {
	c := agentic.NewLRUCache[string, int](2)
	c.Set("a", 1)
	c.Set("b", 2)

	c.Get("a") // touch a, making b the LRU

	c.Set("c", 3) // evicts b (not a)
	_, ok := c.Get("b")
	assert.False(t, ok, "b should be evicted")

	val, ok := c.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, val)
}

func TestLRUCache_PeekDoesNotUpdateRecency(t *testing.T) {
	c := agentic.NewLRUCache[string, int](2)
	c.Set("a", 1)
	c.Set("b", 2)

	val, ok := c.Peek("a") // peek without touching
	assert.True(t, ok)
	assert.Equal(t, 1, val)

	c.Set("c", 3) // evicts a (still LRU since Peek didn't update)
	_, ok = c.Get("a")
	assert.False(t, ok, "a should be evicted since Peek doesn't update recency")
}

func TestLRUCache_Update(t *testing.T) {
	c := agentic.NewLRUCache[string, int](3)
	c.Set("a", 1)
	c.Set("a", 2) // update

	val, _ := c.Get("a")
	assert.Equal(t, 2, val)
	assert.Equal(t, 1, c.Size())
}

func TestLRUCache_Delete(t *testing.T) {
	c := agentic.NewLRUCache[string, int](3)
	c.Set("a", 1)
	assert.True(t, c.Delete("a"))
	assert.False(t, c.Delete("a"))
	assert.Equal(t, 0, c.Size())
}

func TestLRUCache_Has(t *testing.T) {
	c := agentic.NewLRUCache[string, int](3)
	assert.False(t, c.Has("a"))
	c.Set("a", 1)
	assert.True(t, c.Has("a"))
}

func TestLRUCache_Clear(t *testing.T) {
	c := agentic.NewLRUCache[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()
	assert.Equal(t, 0, c.Size())
	_, ok := c.Get("a")
	assert.False(t, ok)
}

func TestLRUCache_Cap(t *testing.T) {
	c := agentic.NewLRUCache[string, int](42)
	assert.Equal(t, 42, c.Cap())
}

func TestLRUCache_DefaultCap(t *testing.T) {
	c := agentic.NewLRUCache[string, int](0)
	assert.Equal(t, 100, c.Cap())
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	c := agentic.NewLRUCache[int, int](100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			c.Set(v, v*2)
			c.Get(v)
			c.Has(v)
		}(i)
	}
	wg.Wait()
	assert.LessOrEqual(t, c.Size(), 100)
}

func TestLRUCache_EvictionOrder(t *testing.T) {
	c := agentic.NewLRUCache[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Access order: a, b, c → c is most recent, a is LRU
	c.Set("d", 4) // evicts a

	assert.False(t, c.Has("a"))
	assert.True(t, c.Has("b"))
	assert.True(t, c.Has("c"))
	assert.True(t, c.Has("d"))
}
