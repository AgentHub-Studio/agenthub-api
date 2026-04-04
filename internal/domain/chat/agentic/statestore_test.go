package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestStateStore_GetState(t *testing.T) {
	s := agentic.NewStateStore(42, nil)
	assert.Equal(t, 42, s.GetState())
}

func TestStateStore_SetState(t *testing.T) {
	s := agentic.NewStateStore(0, nil)
	s.SetState(func(prev int) int { return prev + 10 })
	assert.Equal(t, 10, s.GetState())
}

func TestStateStore_NoNotifyOnSameState(t *testing.T) {
	called := false
	s := agentic.NewStateStore(5, func(_, _ int) { called = true })
	s.SetState(func(prev int) int { return prev }) // no change
	assert.False(t, called)
}

func TestStateStore_OnChange(t *testing.T) {
	var oldVal, newVal int
	s := agentic.NewStateStore(1, func(old, new_ int) {
		oldVal = old
		newVal = new_
	})
	s.SetState(func(_ int) int { return 2 })
	assert.Equal(t, 1, oldVal)
	assert.Equal(t, 2, newVal)
}

func TestStateStore_Subscribe(t *testing.T) {
	calls := 0
	s := agentic.NewStateStore(0, nil)
	unsub := s.Subscribe(func() { calls++ })
	_ = unsub

	s.SetState(func(_ int) int { return 1 })
	assert.Equal(t, 1, calls)

	s.SetState(func(_ int) int { return 2 })
	assert.Equal(t, 2, calls)
}

func TestStateStore_Unsubscribe(t *testing.T) {
	calls := 0
	s := agentic.NewStateStore(0, nil)
	unsub := s.Subscribe(func() { calls++ })

	s.SetState(func(_ int) int { return 1 })
	assert.Equal(t, 1, calls)

	unsub()
	s.SetState(func(_ int) int { return 2 })
	assert.Equal(t, 1, calls) // no more calls
}

func TestStateStore_MultipleSubscribers(t *testing.T) {
	calls1, calls2 := 0, 0
	s := agentic.NewStateStore(0, nil)
	s.Subscribe(func() { calls1++ })
	s.Subscribe(func() { calls2++ })

	s.SetState(func(_ int) int { return 1 })
	assert.Equal(t, 1, calls1)
	assert.Equal(t, 1, calls2)
}

func TestStateStore_StringState(t *testing.T) {
	s := agentic.NewStateStore("init", nil)
	assert.Equal(t, "init", s.GetState())

	s.SetState(func(_ string) string { return "updated" })
	assert.Equal(t, "updated", s.GetState())
}
