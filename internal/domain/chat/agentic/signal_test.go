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

// --- Signal ---

func TestSignal_EmitWithNoListeners(t *testing.T) {
	s := agentic.NewSignal[string]()
	s.Emit("hello") // should not panic
}

func TestSignal_SubscribeAndEmit(t *testing.T) {
	s := agentic.NewSignal[int]()
	var received int
	s.Subscribe(func(v int) { received = v })
	s.Emit(42)
	assert.Equal(t, 42, received)
}

func TestSignal_MultipleListeners(t *testing.T) {
	s := agentic.NewSignal[string]()
	var results []string
	var mu sync.Mutex

	s.Subscribe(func(v string) {
		mu.Lock()
		results = append(results, "a:"+v)
		mu.Unlock()
	})
	s.Subscribe(func(v string) {
		mu.Lock()
		results = append(results, "b:"+v)
		mu.Unlock()
	})

	s.Emit("test")
	assert.Len(t, results, 2)
}

func TestSignal_Unsubscribe(t *testing.T) {
	s := agentic.NewSignal[int]()
	callCount := 0
	unsub := s.Subscribe(func(_ int) { callCount++ })

	s.Emit(1)
	assert.Equal(t, 1, callCount)

	unsub()
	s.Emit(2)
	assert.Equal(t, 1, callCount, "should not receive after unsubscribe")
}

func TestSignal_UnsubscribeIdempotent(t *testing.T) {
	s := agentic.NewSignal[int]()
	unsub := s.Subscribe(func(_ int) {})
	unsub()
	unsub() // should not panic
}

func TestSignal_Clear(t *testing.T) {
	s := agentic.NewSignal[int]()
	s.Subscribe(func(_ int) {})
	s.Subscribe(func(_ int) {})
	assert.Equal(t, 2, s.Len())

	s.Clear()
	assert.Equal(t, 0, s.Len())
}

func TestSignal_Len(t *testing.T) {
	s := agentic.NewSignal[int]()
	assert.Equal(t, 0, s.Len())
	unsub1 := s.Subscribe(func(_ int) {})
	assert.Equal(t, 1, s.Len())
	s.Subscribe(func(_ int) {})
	assert.Equal(t, 2, s.Len())
	unsub1()
	assert.Equal(t, 1, s.Len())
}

func TestSignal_ConcurrentEmit(t *testing.T) {
	s := agentic.NewSignal[int]()
	var count int64
	s.Subscribe(func(_ int) { atomic.AddInt64(&count, 1) })

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			s.Emit(v)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, int64(100), count)
}

func TestSignal_ConcurrentSubscribeAndEmit(t *testing.T) {
	s := agentic.NewSignal[int]()
	var wg sync.WaitGroup

	// Concurrent subscribes.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unsub := s.Subscribe(func(_ int) {})
			time.Sleep(time.Millisecond)
			unsub()
		}()
	}

	// Concurrent emits.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			s.Emit(v)
		}(i)
	}

	wg.Wait() // should not race or panic
}

// --- BufferedWriter ---

func TestBufferedWriter_ImmediateMode(t *testing.T) {
	var received []string
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:       func(s string) { received = append(received, s) },
		ImmediateMode: true,
	})

	w.Write("hello")
	w.Write("world")
	assert.Equal(t, []string{"hello", "world"}, received)
}

func TestBufferedWriter_FlushOnDispose(t *testing.T) {
	var received string
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:       func(s string) { received = s },
		FlushInterval: 10 * time.Second, // long interval, shouldn't fire
	})

	w.Write("a")
	w.Write("b")
	assert.Empty(t, received, "should not flush yet")

	w.Dispose()
	assert.Equal(t, "ab", received)
}

func TestBufferedWriter_FlushOnSizeLimit(t *testing.T) {
	var received string
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:        func(s string) { received = s },
		MaxBufferSize:  3,
		FlushInterval:  10 * time.Second,
	})

	w.Write("x")
	w.Write("y")
	assert.Empty(t, received, "should buffer")

	w.Write("z") // triggers flush at size=3
	assert.Equal(t, "xyz", received)
}

func TestBufferedWriter_FlushOnByteLimit(t *testing.T) {
	var received string
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:         func(s string) { received = s },
		MaxBufferBytes:  10,
		FlushInterval:   10 * time.Second,
	})

	w.Write("12345")
	assert.Empty(t, received)

	w.Write("67890") // 10 bytes total → flush
	assert.Equal(t, "1234567890", received)
}

func TestBufferedWriter_ManualFlush(t *testing.T) {
	var received string
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:       func(s string) { received = s },
		FlushInterval: 10 * time.Second,
	})

	w.Write("data")
	assert.Empty(t, received)

	w.Flush()
	assert.Equal(t, "data", received)
}

func TestBufferedWriter_TimerFlush(t *testing.T) {
	var received string
	var mu sync.Mutex
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn: func(s string) {
			mu.Lock()
			received = s
			mu.Unlock()
		},
		FlushInterval: 50 * time.Millisecond,
	})

	w.Write("timed")

	// Wait for timer to fire.
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	result := received
	mu.Unlock()
	assert.Equal(t, "timed", result)
	w.Dispose()
}

func TestBufferedWriter_Pending(t *testing.T) {
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:       func(_ string) {},
		FlushInterval: 10 * time.Second,
	})

	assert.Equal(t, 0, w.Pending())
	w.Write("a")
	w.Write("b")
	assert.Equal(t, 2, w.Pending())
	w.Flush()
	assert.Equal(t, 0, w.Pending())
	w.Dispose()
}

func TestBufferedWriter_DisposeStopsTimer(t *testing.T) {
	callCount := 0
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:       func(_ string) { callCount++ },
		FlushInterval: 50 * time.Millisecond,
	})

	w.Write("data")
	w.Dispose()
	require.Equal(t, 1, callCount, "dispose should flush once")

	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, 1, callCount, "timer should not fire after dispose")
}

func TestBufferedWriter_WriteAfterDispose(t *testing.T) {
	callCount := 0
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:       func(_ string) { callCount++ },
		FlushInterval: 10 * time.Second,
	})

	w.Dispose()
	w.Write("ignored")
	assert.Equal(t, 0, callCount, "write after dispose should be ignored")
}

func TestBufferedWriter_FlushEmpty(t *testing.T) {
	callCount := 0
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn:       func(_ string) { callCount++ },
		FlushInterval: 10 * time.Second,
	})

	w.Flush() // empty flush
	assert.Equal(t, 0, callCount, "should not call WriteFn for empty buffer")
	w.Dispose()
}

func TestBufferedWriter_ConcurrentWrites(t *testing.T) {
	var mu sync.Mutex
	var flushes []string
	w := agentic.NewBufferedWriter(agentic.BufferedWriterConfig{
		WriteFn: func(s string) {
			mu.Lock()
			flushes = append(flushes, s)
			mu.Unlock()
		},
		MaxBufferSize: 10,
		FlushInterval: 10 * time.Second,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Write("x")
		}()
	}
	wg.Wait()
	w.Dispose()

	mu.Lock()
	total := 0
	for _, s := range flushes {
		total += len(s)
	}
	mu.Unlock()
	assert.Equal(t, 50, total, "all 50 writes should be flushed")
}
