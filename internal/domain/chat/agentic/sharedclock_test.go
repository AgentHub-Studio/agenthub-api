package agentic_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewSharedClock ---

func TestNewSharedClock(t *testing.T) {
	c := agentic.NewSharedClock(50 * time.Millisecond)
	defer c.Close()
	assert.NotNil(t, c)
	assert.False(t, c.IsRunning())
	assert.Equal(t, 0, c.SubscriberCount())
}

// --- Subscribe starts clock ---

func TestSharedClock_Subscribe_KeepAlive(t *testing.T) {
	c := agentic.NewSharedClock(20 * time.Millisecond)
	defer c.Close()

	var count atomic.Int32
	unsub := c.Subscribe(func() {
		count.Add(1)
	}, true)

	assert.True(t, c.IsRunning())
	assert.Equal(t, 1, c.SubscriberCount())

	time.Sleep(80 * time.Millisecond)
	assert.GreaterOrEqual(t, int(count.Load()), 2)

	unsub()
	assert.Equal(t, 0, c.SubscriberCount())
	assert.False(t, c.IsRunning())
}

func TestSharedClock_Subscribe_NoKeepAlive(t *testing.T) {
	c := agentic.NewSharedClock(20 * time.Millisecond)
	defer c.Close()

	unsub := c.Subscribe(func() {}, false)
	defer unsub()

	// Non-keepalive subscriber should NOT start the clock
	assert.False(t, c.IsRunning())
	assert.Equal(t, 1, c.SubscriberCount())
}

// --- Multiple subscribers ---

func TestSharedClock_MultipleSubscribers(t *testing.T) {
	c := agentic.NewSharedClock(20 * time.Millisecond)
	defer c.Close()

	var c1, c2 atomic.Int32
	unsub1 := c.Subscribe(func() { c1.Add(1) }, true)
	unsub2 := c.Subscribe(func() { c2.Add(1) }, false)

	time.Sleep(60 * time.Millisecond)

	// Both get ticks since clock is running (due to sub1 keepAlive)
	assert.GreaterOrEqual(t, int(c1.Load()), 1)
	assert.GreaterOrEqual(t, int(c2.Load()), 1)

	unsub1()
	// Clock should stop — only non-keepalive subscriber remains
	time.Sleep(10 * time.Millisecond)
	assert.False(t, c.IsRunning())
	unsub2()
}

// --- Unsubscribe ---

func TestSharedClock_Unsubscribe(t *testing.T) {
	c := agentic.NewSharedClock(20 * time.Millisecond)
	defer c.Close()

	unsub := c.Subscribe(func() {}, true)
	assert.True(t, c.IsRunning())

	unsub()
	time.Sleep(10 * time.Millisecond)
	assert.False(t, c.IsRunning())
	assert.Equal(t, 0, c.SubscriberCount())

	// Double unsubscribe should not panic
	unsub()
}

// --- Now synchronized ---

func TestSharedClock_Now(t *testing.T) {
	c := agentic.NewSharedClock(20 * time.Millisecond)
	defer c.Close()

	t0 := c.Now()
	time.Sleep(30 * time.Millisecond)
	t1 := c.Now()
	assert.Greater(t, t1, t0)
}

func TestSharedClock_Now_Synchronized(t *testing.T) {
	c := agentic.NewSharedClock(50 * time.Millisecond)
	defer c.Close()

	var mu sync.Mutex
	var readings []int64

	unsub := c.Subscribe(func() {
		// Multiple reads within same tick should return same value
		v1 := c.Now()
		v2 := c.Now()
		mu.Lock()
		readings = append(readings, v1, v2)
		mu.Unlock()
	}, true)
	defer unsub()

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	// Each pair should be equal (same tick)
	for i := 0; i+1 < len(readings); i += 2 {
		assert.Equal(t, readings[i], readings[i+1], "readings within same tick should be equal")
	}
}

// --- SetTickInterval ---

func TestSharedClock_SetTickInterval(t *testing.T) {
	c := agentic.NewSharedClock(100 * time.Millisecond)
	defer c.Close()

	assert.Equal(t, 100*time.Millisecond, c.TickInterval())

	c.SetTickInterval(20 * time.Millisecond)
	assert.Equal(t, 20*time.Millisecond, c.TickInterval())
}

func TestSharedClock_SetTickInterval_WhileRunning(t *testing.T) {
	c := agentic.NewSharedClock(200 * time.Millisecond)
	defer c.Close()

	var count atomic.Int32
	unsub := c.Subscribe(func() { count.Add(1) }, true)
	defer unsub()

	assert.True(t, c.IsRunning())

	c.SetTickInterval(20 * time.Millisecond)
	time.Sleep(80 * time.Millisecond)

	// Should get more ticks with faster interval
	assert.GreaterOrEqual(t, int(count.Load()), 2)
}

func TestSharedClock_SetTickInterval_SameNoOp(t *testing.T) {
	c := agentic.NewSharedClock(50 * time.Millisecond)
	defer c.Close()
	c.SetTickInterval(50 * time.Millisecond) // same — no-op
	assert.Equal(t, 50*time.Millisecond, c.TickInterval())
}

// --- Close ---

func TestSharedClock_Close(t *testing.T) {
	c := agentic.NewSharedClock(20 * time.Millisecond)
	unsub := c.Subscribe(func() {}, true)
	_ = unsub

	c.Close()
	assert.False(t, c.IsRunning())
	assert.Equal(t, 0, c.SubscriberCount())
}

func TestSharedClock_Close_SubscribeAfterClose(t *testing.T) {
	c := agentic.NewSharedClock(20 * time.Millisecond)
	c.Close()

	unsub := c.Subscribe(func() {}, true)
	unsub() // should not panic

	assert.False(t, c.IsRunning())
	assert.Equal(t, 0, c.SubscriberCount())
}

// --- Concurrent subscribe/unsubscribe ---

func TestSharedClock_ConcurrentAccess(t *testing.T) {
	c := agentic.NewSharedClock(10 * time.Millisecond)
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unsub := c.Subscribe(func() {}, i%2 == 0)
			time.Sleep(30 * time.Millisecond)
			unsub()
		}()
	}
	wg.Wait()

	assert.Equal(t, 0, c.SubscriberCount())
}
