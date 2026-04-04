package agentic_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewMinDisplayThrottle ---

func TestNewMinDisplayThrottle(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("init", 100*time.Millisecond, nil)
	assert.Equal(t, "init", m.Current())
}

// --- Set immediate ---

func TestMinDisplayThrottle_Set_Immediate(t *testing.T) {
	var mu sync.Mutex
	var updates []string
	m := agentic.NewMinDisplayThrottle("a", 50*time.Millisecond, func(v string) {
		mu.Lock()
		updates = append(updates, v)
		mu.Unlock()
	})
	defer m.Close()

	m.Set("b")
	time.Sleep(10 * time.Millisecond) // let goroutine fire
	assert.Equal(t, "b", m.Current())
	mu.Lock()
	assert.Equal(t, []string{"b"}, updates)
	mu.Unlock()
}

// --- Set deferred (within min duration) ---

func TestMinDisplayThrottle_Set_Deferred(t *testing.T) {
	var mu sync.Mutex
	var updates []string
	m := agentic.NewMinDisplayThrottle("a", 80*time.Millisecond, func(v string) {
		mu.Lock()
		updates = append(updates, v)
		mu.Unlock()
	})
	defer m.Close()

	m.Set("b") // immediate
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, "b", m.Current())

	m.Set("c") // should be deferred — less than 80ms since "b"
	assert.Equal(t, "b", m.Current())

	p, ok := m.Pending()
	assert.True(t, ok)
	assert.Equal(t, "c", p)

	// Wait for deferred apply
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, "c", m.Current())

	mu.Lock()
	assert.Equal(t, []string{"b", "c"}, updates)
	mu.Unlock()
}

// --- Set same value clears pending ---

func TestMinDisplayThrottle_Set_SameValue(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("a", 100*time.Millisecond, nil)
	defer m.Close()

	m.Set("b") // immediate
	time.Sleep(5 * time.Millisecond)
	m.Set("c") // deferred

	_, ok := m.Pending()
	assert.True(t, ok)

	m.Set("b") // same as current — clears pending
	_, ok = m.Pending()
	assert.False(t, ok)
}

// --- Pending ---

func TestMinDisplayThrottle_Pending_None(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("a", 50*time.Millisecond, nil)
	defer m.Close()

	_, ok := m.Pending()
	assert.False(t, ok)
}

// --- Flush ---

func TestMinDisplayThrottle_Flush(t *testing.T) {
	var mu sync.Mutex
	var updates []string
	m := agentic.NewMinDisplayThrottle("a", 200*time.Millisecond, func(v string) {
		mu.Lock()
		updates = append(updates, v)
		mu.Unlock()
	})
	defer m.Close()

	m.Set("b") // immediate
	time.Sleep(10 * time.Millisecond)
	m.Set("c") // deferred
	assert.Equal(t, "b", m.Current())

	m.Flush()
	time.Sleep(10 * time.Millisecond) // let goroutine fire
	assert.Equal(t, "c", m.Current())

	_, ok := m.Pending()
	assert.False(t, ok)
}

func TestMinDisplayThrottle_Flush_NoPending(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("a", 50*time.Millisecond, nil)
	defer m.Close()
	m.Flush() // should not panic
	assert.Equal(t, "a", m.Current())
}

// --- Close ---

func TestMinDisplayThrottle_Close(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("a", 200*time.Millisecond, nil)
	m.Set("b")
	time.Sleep(5 * time.Millisecond)
	m.Set("c") // deferred
	m.Close()

	// Should not apply pending after close
	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, "b", m.Current())
}

func TestMinDisplayThrottle_Close_SetAfterClose(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("a", 50*time.Millisecond, nil)
	m.Close()
	m.Set("b") // should be no-op
	assert.Equal(t, "a", m.Current())
}

// --- Rapid updates keep last ---

func TestMinDisplayThrottle_RapidUpdates(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("a", 100*time.Millisecond, nil)
	defer m.Close()

	m.Set("b") // immediate
	time.Sleep(5 * time.Millisecond)

	// Rapid updates while within min duration — only last should apply
	m.Set("c")
	m.Set("d")
	m.Set("e")

	p, ok := m.Pending()
	assert.True(t, ok)
	assert.Equal(t, "e", p)

	time.Sleep(120 * time.Millisecond)
	assert.Equal(t, "e", m.Current())
}

// --- Nil callback ---

func TestMinDisplayThrottle_NilCallback(t *testing.T) {
	m := agentic.NewMinDisplayThrottle("a", 50*time.Millisecond, nil)
	defer m.Close()
	m.Set("b") // should not panic with nil onUpdate
	assert.Equal(t, "b", m.Current())
}
