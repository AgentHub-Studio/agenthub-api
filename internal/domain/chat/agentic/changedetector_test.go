package agentic_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestNewChangeDetector_DefaultDuration(t *testing.T) {
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{}, func(paths []string) {})
	assert.NotNil(t, d)
}

func TestNewChangeDetector_CustomDuration(t *testing.T) {
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 50 * time.Millisecond,
	}, func(paths []string) {})
	assert.NotNil(t, d)
}

func TestChangeDetector_Notify_SinglePath(t *testing.T) {
	var mu sync.Mutex
	var received []string

	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		mu.Lock()
		received = paths
		mu.Unlock()
	})
	defer d.Dispose()

	d.Notify("/foo/bar.go")

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	assert.Equal(t, "/foo/bar.go", received[0])
}

func TestChangeDetector_Notify_Deduplicates(t *testing.T) {
	var mu sync.Mutex
	var received []string

	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		mu.Lock()
		received = paths
		mu.Unlock()
	})
	defer d.Dispose()

	d.Notify("/foo/bar.go")
	d.Notify("/foo/bar.go")
	d.Notify("/foo/bar.go")

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1, "duplicate paths should be deduplicated")
}

func TestChangeDetector_Notify_MultiplePaths(t *testing.T) {
	var mu sync.Mutex
	var received []string

	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		mu.Lock()
		received = paths
		mu.Unlock()
	})
	defer d.Dispose()

	d.Notify("/a.go")
	d.Notify("/b.go")
	d.Notify("/c.go")

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 3)
}

func TestChangeDetector_NotifyBatch(t *testing.T) {
	var mu sync.Mutex
	var received []string

	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		mu.Lock()
		received = paths
		mu.Unlock()
	})
	defer d.Dispose()

	d.NotifyBatch([]string{"/a.go", "/b.go", "/c.go"})

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 3)
}

func TestChangeDetector_NotifyBatch_Deduplicates(t *testing.T) {
	var mu sync.Mutex
	var received []string

	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		mu.Lock()
		received = paths
		mu.Unlock()
	})
	defer d.Dispose()

	d.NotifyBatch([]string{"/a.go", "/b.go"})
	d.Notify("/a.go") // duplicate

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 2)
}

func TestChangeDetector_Flush(t *testing.T) {
	var mu sync.Mutex
	var received []string

	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 5 * time.Second, // long debounce
	}, func(paths []string) {
		mu.Lock()
		received = paths
		mu.Unlock()
	})
	defer d.Dispose()

	d.Notify("/x.go")
	d.Flush() // immediate

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	assert.Equal(t, "/x.go", received[0])
}

func TestChangeDetector_Flush_Empty(t *testing.T) {
	called := false
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		called = true
	})
	defer d.Dispose()

	d.Flush() // nothing pending
	assert.False(t, called)
}

func TestChangeDetector_PendingCount(t *testing.T) {
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 5 * time.Second,
	}, func(paths []string) {})
	defer d.Dispose()

	assert.Equal(t, 0, d.PendingCount())

	d.Notify("/a.go")
	d.Notify("/b.go")
	assert.Equal(t, 2, d.PendingCount())

	d.Flush()
	assert.Equal(t, 0, d.PendingCount())
}

func TestChangeDetector_TotalChanges(t *testing.T) {
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 5 * time.Second,
	}, func(paths []string) {})
	defer d.Dispose()

	assert.Equal(t, 0, d.TotalChanges())

	d.Notify("/a.go")
	d.Notify("/a.go") // duplicate path but still counted
	d.Notify("/b.go")
	assert.Equal(t, 3, d.TotalChanges())
}

func TestChangeDetector_Dispose_StopsProcessing(t *testing.T) {
	called := false
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		called = true
	})

	d.Notify("/a.go")
	d.Dispose()

	time.Sleep(80 * time.Millisecond)
	assert.False(t, called, "handler should not fire after dispose")
}

func TestChangeDetector_Dispose_IgnoresNewNotifications(t *testing.T) {
	called := false
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, func(paths []string) {
		called = true
	})

	d.Dispose()
	d.Notify("/a.go")
	d.NotifyBatch([]string{"/b.go"})

	time.Sleep(80 * time.Millisecond)
	assert.False(t, called)
	assert.Equal(t, 0, d.PendingCount())
}

func TestChangeDetector_Debounce_ResetsTimer(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 50 * time.Millisecond,
	}, func(paths []string) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})
	defer d.Dispose()

	// Fire notifications with gaps shorter than debounce
	d.Notify("/a.go")
	time.Sleep(20 * time.Millisecond)
	d.Notify("/b.go")
	time.Sleep(20 * time.Millisecond)
	d.Notify("/c.go")

	// Wait for debounce to fire
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, callCount, "debounce should batch into single call")
}

func TestChangeDetector_NilHandler(t *testing.T) {
	d := agentic.NewChangeDetector(agentic.ChangeDetectorConfig{
		DebounceDuration: 30 * time.Millisecond,
	}, nil)
	defer d.Dispose()

	d.Notify("/a.go")
	d.Flush() // should not panic
}
