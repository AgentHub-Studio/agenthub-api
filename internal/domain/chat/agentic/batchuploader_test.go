package agentic_test

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DefaultBatchUploaderConfig ---

func TestDefaultBatchUploaderConfig(t *testing.T) {
	cfg := agentic.DefaultBatchUploaderConfig()
	assert.Equal(t, 100, cfg.MaxBatchSize)
	assert.Equal(t, 1024*1024, cfg.MaxBatchBytes)
	assert.Equal(t, 10000, cfg.MaxQueueSize)
	assert.Equal(t, 5, cfg.MaxConsecutiveFailures)
	assert.Equal(t, time.Second, cfg.BaseRetryDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxRetryDelay)
}

// --- Enqueue ---

func TestBatchUploader_Enqueue(t *testing.T) {
	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), nil)
	ok := u.Enqueue(map[string]string{"key": "value"})
	assert.True(t, ok)
	assert.Equal(t, 1, u.QueueLen())
}

func TestBatchUploader_Enqueue_AfterClose(t *testing.T) {
	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), nil)
	u.Close()
	ok := u.Enqueue("item")
	assert.False(t, ok)
}

// --- EnqueueRaw ---

func TestBatchUploader_EnqueueRaw(t *testing.T) {
	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), nil)
	ok := u.EnqueueRaw(json.RawMessage(`{"raw":true}`))
	assert.True(t, ok)
	assert.Equal(t, 1, u.QueueLen())
}

func TestBatchUploader_EnqueueRaw_AfterClose(t *testing.T) {
	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), nil)
	u.Close()
	ok := u.EnqueueRaw(json.RawMessage(`{}`))
	assert.False(t, ok)
}

// --- TakeBatch ---

func TestBatchUploader_TakeBatch_Empty(t *testing.T) {
	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), nil)
	batch := u.TakeBatch()
	assert.Nil(t, batch)
}

func TestBatchUploader_TakeBatch_RespectsMaxBatchSize(t *testing.T) {
	cfg := agentic.DefaultBatchUploaderConfig()
	cfg.MaxBatchSize = 3
	u := agentic.NewBatchUploader(cfg, nil)

	for i := 0; i < 10; i++ {
		u.Enqueue(i)
	}

	batch := u.TakeBatch()
	assert.Len(t, batch, 3)
	assert.Equal(t, 7, u.QueueLen())
}

func TestBatchUploader_TakeBatch_RespectsMaxBatchBytes(t *testing.T) {
	cfg := agentic.DefaultBatchUploaderConfig()
	cfg.MaxBatchBytes = 10 // very small
	cfg.MaxBatchSize = 100
	u := agentic.NewBatchUploader(cfg, nil)

	// Each item serializes to ~1-3 bytes
	u.Enqueue(1)
	u.Enqueue(2)
	u.Enqueue(3)
	u.Enqueue(4)
	u.Enqueue(5)

	batch := u.TakeBatch()
	// At least 1 item (first item always included even if over byte limit)
	assert.GreaterOrEqual(t, len(batch), 1)
	assert.Less(t, len(batch), 6)
}

// --- UploadOnce ---

func TestBatchUploader_UploadOnce_Success(t *testing.T) {
	var uploaded []json.RawMessage
	fn := func(batch []json.RawMessage) (time.Duration, error) {
		uploaded = batch
		return 0, nil
	}

	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), fn)
	u.Enqueue("hello")
	u.Enqueue("world")

	n, dropped := u.UploadOnce()
	assert.Equal(t, 2, n)
	assert.False(t, dropped)
	assert.Len(t, uploaded, 2)
	assert.Equal(t, 2, u.TotalUploaded())
}

func TestBatchUploader_UploadOnce_EmptyQueue(t *testing.T) {
	fn := func(batch []json.RawMessage) (time.Duration, error) {
		return 0, nil
	}

	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), fn)
	n, dropped := u.UploadOnce()
	assert.Equal(t, 0, n)
	assert.False(t, dropped)
}

func TestBatchUploader_UploadOnce_DropAfterMaxFailures(t *testing.T) {
	cfg := agentic.DefaultBatchUploaderConfig()
	cfg.MaxConsecutiveFailures = 2
	cfg.BaseRetryDelay = time.Millisecond
	cfg.MaxRetryDelay = time.Millisecond

	var attempts int32
	fn := func(batch []json.RawMessage) (time.Duration, error) {
		atomic.AddInt32(&attempts, 1)
		return 0, assert.AnError
	}

	u := agentic.NewBatchUploader(cfg, fn)
	u.Enqueue("item")

	n, dropped := u.UploadOnce()
	assert.Equal(t, 0, n)
	assert.True(t, dropped)
	assert.Equal(t, 1, u.DroppedBatches())
}

// --- Flush ---

func TestBatchUploader_Flush(t *testing.T) {
	var total int32
	fn := func(batch []json.RawMessage) (time.Duration, error) {
		atomic.AddInt32(&total, int32(len(batch)))
		return 0, nil
	}

	cfg := agentic.DefaultBatchUploaderConfig()
	cfg.MaxBatchSize = 2
	u := agentic.NewBatchUploader(cfg, fn)

	for i := 0; i < 5; i++ {
		u.Enqueue(i)
	}

	u.Flush()
	assert.Equal(t, int32(5), atomic.LoadInt32(&total))
	assert.Equal(t, 0, u.QueueLen())
}

// --- Close ---

func TestBatchUploader_Close(t *testing.T) {
	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), nil)
	assert.False(t, u.IsClosed())
	u.Close()
	assert.True(t, u.IsClosed())
}

// --- Stats ---

func TestBatchUploader_Stats(t *testing.T) {
	fn := func(batch []json.RawMessage) (time.Duration, error) {
		return 0, nil
	}

	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), fn)
	u.Enqueue("a")
	u.Enqueue("b")
	u.UploadOnce()

	stats := u.Stats()
	assert.Equal(t, 0, stats.QueueLen)
	assert.Equal(t, 2, stats.TotalUploaded)
	assert.Equal(t, 0, stats.DroppedBatches)
	assert.False(t, stats.Closed)
}

func TestBatchUploaderStats_Summary(t *testing.T) {
	stats := agentic.BatchUploaderStats{
		QueueLen:       5,
		TotalUploaded:  100,
		DroppedBatches: 2,
		Closed:         false,
	}
	s := stats.Summary()
	assert.Contains(t, s, "queue=5")
	assert.Contains(t, s, "uploaded=100")
	assert.Contains(t, s, "dropped=2")
	assert.Contains(t, s, "closed=false")
}

// --- Backpressure ---

func TestBatchUploader_Backpressure(t *testing.T) {
	cfg := agentic.DefaultBatchUploaderConfig()
	cfg.MaxQueueSize = 5

	fn := func(batch []json.RawMessage) (time.Duration, error) {
		return 0, nil
	}

	u := agentic.NewBatchUploader(cfg, fn)

	// Fill queue
	for i := 0; i < 5; i++ {
		ok := u.Enqueue(i)
		require.True(t, ok)
	}

	// Next enqueue should block until we drain
	done := make(chan bool, 1)
	go func() {
		ok := u.Enqueue(99)
		done <- ok
	}()

	// Drain some items
	time.Sleep(10 * time.Millisecond)
	u.UploadOnce()

	select {
	case ok := <-done:
		assert.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("enqueue blocked forever")
	}
}

// --- Concurrent enqueue ---

func TestBatchUploader_ConcurrentEnqueue(t *testing.T) {
	fn := func(batch []json.RawMessage) (time.Duration, error) {
		return 0, nil
	}

	u := agentic.NewBatchUploader(agentic.DefaultBatchUploaderConfig(), fn)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			u.Enqueue(v)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 50, u.QueueLen())

	u.Flush()
	assert.Equal(t, 50, u.TotalUploaded())
}

// --- Config defaults applied ---

func TestNewBatchUploader_DefaultsApplied(t *testing.T) {
	// Zero config should get defaults
	u := agentic.NewBatchUploader(agentic.BatchUploaderConfig{}, nil)
	assert.NotNil(t, u)
	// Should not panic on enqueue
	u.Enqueue("test")
	assert.Equal(t, 1, u.QueueLen())
}
