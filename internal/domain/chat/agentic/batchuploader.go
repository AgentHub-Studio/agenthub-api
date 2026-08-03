package agentic

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Serial batch event uploader with backpressure.
//
// Inspired by Claude Code's SerialBatchEventUploader.ts — batches events
// for efficient upload with serial ordering (one request in-flight at a time),
// backpressure when the queue is full, and exponential backoff with jitter.

// BatchUploaderConfig configures the batch uploader.
type BatchUploaderConfig struct {
	// MaxBatchSize is the maximum number of items per batch.
	MaxBatchSize int
	// MaxBatchBytes is the maximum serialized byte size per batch.
	MaxBatchBytes int
	// MaxQueueSize is the maximum queue depth before backpressure.
	MaxQueueSize int
	// MaxConsecutiveFailures is the failure budget before dropping a batch.
	MaxConsecutiveFailures int
	// BaseRetryDelay is the initial retry delay.
	BaseRetryDelay time.Duration
	// MaxRetryDelay caps the exponential backoff.
	MaxRetryDelay time.Duration
}

// DefaultBatchUploaderConfig returns sensible defaults.
func DefaultBatchUploaderConfig() BatchUploaderConfig {
	return BatchUploaderConfig{
		MaxBatchSize:           100,
		MaxBatchBytes:          1024 * 1024, // 1MB
		MaxQueueSize:           10000,
		MaxConsecutiveFailures: 5,
		BaseRetryDelay:         time.Second,
		MaxRetryDelay:          30 * time.Second,
	}
}

// BatchUploadFunc is called to upload a batch. Returns a suggested retry-after
// duration (zero means no server hint) and any error.
type BatchUploadFunc func(batch []json.RawMessage) (retryAfter time.Duration, err error)

// BatchUploader buffers events and uploads them in serial batches.
type BatchUploader struct {
	mu                  sync.Mutex
	config              BatchUploaderConfig
	queue               []json.RawMessage
	uploadFn            BatchUploadFunc
	consecutiveFailures int
	droppedBatches      int
	totalUploaded       int
	closed              bool
	backpressure        *sync.Cond
}

// NewBatchUploader creates a batch uploader.
func NewBatchUploader(config BatchUploaderConfig, fn BatchUploadFunc) *BatchUploader {
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 100
	}
	if config.MaxBatchBytes <= 0 {
		config.MaxBatchBytes = 1024 * 1024
	}
	if config.MaxQueueSize <= 0 {
		config.MaxQueueSize = 10000
	}
	if config.MaxConsecutiveFailures <= 0 {
		config.MaxConsecutiveFailures = 5
	}
	if config.BaseRetryDelay <= 0 {
		config.BaseRetryDelay = time.Second
	}
	if config.MaxRetryDelay <= 0 {
		config.MaxRetryDelay = 30 * time.Second
	}

	bu := &BatchUploader{
		config:   config,
		uploadFn: fn,
	}
	bu.backpressure = sync.NewCond(&bu.mu)
	return bu
}

// Enqueue adds an item to the upload queue.
// Blocks if the queue is at MaxQueueSize (backpressure).
// Returns false if the uploader is closed or the item can't be serialized.
func (u *BatchUploader) Enqueue(item interface{}) bool {
	data, err := json.Marshal(item)
	if err != nil {
		return false // drop un-serializable items
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.closed {
		return false
	}

	// Backpressure: wait until queue has space.
	for len(u.queue) >= u.config.MaxQueueSize && !u.closed {
		u.backpressure.Wait()
	}

	if u.closed {
		return false
	}

	u.queue = append(u.queue, data)
	return true
}

// EnqueueRaw adds pre-serialized data to the queue.
func (u *BatchUploader) EnqueueRaw(data json.RawMessage) bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.closed {
		return false
	}

	for len(u.queue) >= u.config.MaxQueueSize && !u.closed {
		u.backpressure.Wait()
	}

	if u.closed {
		return false
	}

	u.queue = append(u.queue, data)
	return true
}

// TakeBatch extracts the next batch respecting both count and byte limits.
func (u *BatchUploader) TakeBatch() []json.RawMessage {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.queue) == 0 {
		return nil
	}

	var batch []json.RawMessage
	totalBytes := 0

	end := len(u.queue)
	if end > u.config.MaxBatchSize {
		end = u.config.MaxBatchSize
	}

	for i := 0; i < end; i++ {
		itemSize := len(u.queue[i])
		if totalBytes+itemSize > u.config.MaxBatchBytes && len(batch) > 0 {
			break
		}
		batch = append(batch, u.queue[i])
		totalBytes += itemSize
	}

	u.queue = u.queue[len(batch):]

	// Release backpressure.
	u.backpressure.Broadcast()

	return batch
}

// UploadOnce takes a batch and attempts to upload it.
// Implements retry with exponential backoff and jitter.
// Returns the number of items uploaded, or drops the batch after max failures.
func (u *BatchUploader) UploadOnce() (uploaded int, dropped bool) {
	batch := u.TakeBatch()
	if len(batch) == 0 {
		return 0, false
	}

	for attempt := 0; attempt <= u.config.MaxConsecutiveFailures; attempt++ {
		retryAfter, err := u.uploadFn(batch)
		if err == nil {
			u.mu.Lock()
			u.consecutiveFailures = 0
			u.totalUploaded += len(batch)
			u.mu.Unlock()
			return len(batch), false
		}

		u.mu.Lock()
		u.consecutiveFailures++
		failures := u.consecutiveFailures
		u.mu.Unlock()

		if failures >= u.config.MaxConsecutiveFailures {
			u.mu.Lock()
			u.droppedBatches++
			u.consecutiveFailures = 0
			u.mu.Unlock()
			return 0, true
		}

		delay := u.retryDelay(attempt, retryAfter)
		time.Sleep(delay)
	}

	return 0, false
}

// Flush blocks until all currently queued items are uploaded.
func (u *BatchUploader) Flush() {
	for {
		u.mu.Lock()
		empty := len(u.queue) == 0
		u.mu.Unlock()

		if empty {
			return
		}

		u.UploadOnce()
	}
}

// Close prevents new enqueues and releases any blocked enqueuers.
func (u *BatchUploader) Close() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.closed = true
	u.backpressure.Broadcast()
}

// QueueLen returns the current queue depth.
func (u *BatchUploader) QueueLen() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.queue)
}

// DroppedBatches returns the number of batches dropped due to failures.
func (u *BatchUploader) DroppedBatches() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.droppedBatches
}

// TotalUploaded returns the total number of items successfully uploaded.
func (u *BatchUploader) TotalUploaded() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.totalUploaded
}

// IsClosed returns whether the uploader is closed.
func (u *BatchUploader) IsClosed() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.closed
}

// retryDelay calculates backoff with jitter, respecting server hints.
func (u *BatchUploader) retryDelay(attempt int, serverHint time.Duration) time.Duration {
	base := float64(u.config.BaseRetryDelay) * math.Pow(2, float64(attempt))
	jitter := base * 0.1 * rand.Float64()
	delay := time.Duration(base + jitter)

	if delay > u.config.MaxRetryDelay {
		delay = u.config.MaxRetryDelay
	}

	if serverHint > 0 && serverHint > delay {
		delay = serverHint
	}

	return delay
}

// Stats returns upload statistics.
func (u *BatchUploader) Stats() BatchUploaderStats {
	u.mu.Lock()
	defer u.mu.Unlock()
	return BatchUploaderStats{
		QueueLen:       len(u.queue),
		TotalUploaded:  u.totalUploaded,
		DroppedBatches: u.droppedBatches,
		Closed:         u.closed,
	}
}

// BatchUploaderStats holds upload telemetry.
type BatchUploaderStats struct {
	QueueLen       int  `json:"queueLen"`
	TotalUploaded  int  `json:"totalUploaded"`
	DroppedBatches int  `json:"droppedBatches"`
	Closed         bool `json:"closed"`
}

// Summary returns a human-readable stats summary.
func (s BatchUploaderStats) Summary() string {
	return fmt.Sprintf("queue=%d uploaded=%d dropped=%d closed=%v",
		s.QueueLen, s.TotalUploaded, s.DroppedBatches, s.Closed)
}
