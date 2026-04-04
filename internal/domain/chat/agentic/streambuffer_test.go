package agentic_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DefaultStreamBufferConfig ---

func TestDefaultStreamBufferConfig(t *testing.T) {
	cfg := agentic.DefaultStreamBufferConfig()
	assert.Equal(t, 100*time.Millisecond, cfg.Window)
	assert.Equal(t, 50, cfg.MaxSize)
}

// --- NewStreamBuffer ---

func TestNewStreamBuffer(t *testing.T) {
	b := agentic.NewStreamBuffer(agentic.DefaultStreamBufferConfig(), func(batch []string) {})
	assert.NotNil(t, b)
	assert.False(t, b.IsClosed())
	assert.Equal(t, 0, b.Len())
}

// --- Add ---

func TestStreamBuffer_Add(t *testing.T) {
	b := agentic.NewStreamBuffer(agentic.DefaultStreamBufferConfig(), func(batch []int) {})
	b.Add(1)
	b.Add(2)
	assert.Equal(t, 2, b.Len())
}

// --- Timer flush ---

func TestStreamBuffer_TimerFlush(t *testing.T) {
	var flushed []int
	var mu sync.Mutex
	cfg := agentic.StreamBufferConfig{Window: 50 * time.Millisecond, MaxSize: 100}

	b := agentic.NewStreamBuffer(cfg, func(batch []int) {
		mu.Lock()
		flushed = append(flushed, batch...)
		mu.Unlock()
	})

	b.Add(1)
	b.Add(2)
	b.Add(3)

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2, 3}, flushed)
	assert.Equal(t, 1, b.FlushCount())
}

// --- MaxSize flush ---

func TestStreamBuffer_MaxSizeFlush(t *testing.T) {
	var flushed []int
	var mu sync.Mutex
	cfg := agentic.StreamBufferConfig{Window: 5 * time.Second, MaxSize: 3}

	b := agentic.NewStreamBuffer(cfg, func(batch []int) {
		mu.Lock()
		flushed = append(flushed, batch...)
		mu.Unlock()
	})

	b.Add(1)
	b.Add(2)
	b.Add(3) // should trigger flush at MaxSize

	// Give a tiny bit of time for the flush
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2, 3}, flushed)
}

// --- Manual Flush ---

func TestStreamBuffer_ManualFlush(t *testing.T) {
	var flushed []string
	var mu sync.Mutex
	cfg := agentic.StreamBufferConfig{Window: 5 * time.Second, MaxSize: 100}

	b := agentic.NewStreamBuffer(cfg, func(batch []string) {
		mu.Lock()
		flushed = append(flushed, batch...)
		mu.Unlock()
	})

	b.Add("a")
	b.Add("b")
	b.Flush()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"a", "b"}, flushed)
}

func TestStreamBuffer_FlushEmpty(t *testing.T) {
	flushCount := 0
	cfg := agentic.DefaultStreamBufferConfig()
	b := agentic.NewStreamBuffer(cfg, func(batch []int) {
		flushCount++
	})

	b.Flush()
	assert.Equal(t, 0, flushCount, "empty flush should not call flushFn")
}

// --- Close ---

func TestStreamBuffer_Close(t *testing.T) {
	var flushed []int
	var mu sync.Mutex
	cfg := agentic.StreamBufferConfig{Window: 5 * time.Second, MaxSize: 100}

	b := agentic.NewStreamBuffer(cfg, func(batch []int) {
		mu.Lock()
		flushed = append(flushed, batch...)
		mu.Unlock()
	})

	b.Add(1)
	b.Add(2)
	b.Close()

	assert.True(t, b.IsClosed())

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2}, flushed)
}

func TestStreamBuffer_AddAfterClose(t *testing.T) {
	b := agentic.NewStreamBuffer(agentic.DefaultStreamBufferConfig(), func(batch []int) {})
	b.Close()
	b.Add(1) // should not panic
	assert.Equal(t, 0, b.Len())
}

// --- FlushCount ---

func TestStreamBuffer_FlushCount(t *testing.T) {
	cfg := agentic.StreamBufferConfig{Window: 5 * time.Second, MaxSize: 2}
	b := agentic.NewStreamBuffer(cfg, func(batch []int) {})

	b.Add(1)
	b.Add(2) // triggers flush
	time.Sleep(10 * time.Millisecond)

	b.Add(3)
	b.Flush()

	assert.Equal(t, 2, b.FlushCount())
}

// --- Concurrent adds ---

func TestStreamBuffer_ConcurrentAdd(t *testing.T) {
	var total int
	var mu sync.Mutex
	cfg := agentic.StreamBufferConfig{Window: 50 * time.Millisecond, MaxSize: 1000}

	b := agentic.NewStreamBuffer(cfg, func(batch []int) {
		mu.Lock()
		total += len(batch)
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			b.Add(v)
		}(i)
	}
	wg.Wait()

	b.Close()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 50, total)
}
