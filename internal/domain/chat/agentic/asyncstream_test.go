package agentic_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewStream ---

func TestNewStream(t *testing.T) {
	s := agentic.NewStream[string](10)
	assert.False(t, s.IsClosed())
}

// --- Enqueue / Collect ---

func TestStream_EnqueueCollect(t *testing.T) {
	s := agentic.NewStream[int](10)

	go func() {
		s.Enqueue(1)
		s.Enqueue(2)
		s.Enqueue(3)
		s.Done()
	}()

	values, err := s.Collect()
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, values)
}

// --- Done ---

func TestStream_Done(t *testing.T) {
	s := agentic.NewStream[string](5)
	s.Done()
	assert.True(t, s.IsClosed())
}

func TestStream_Done_Idempotent(t *testing.T) {
	s := agentic.NewStream[string](5)
	s.Done()
	s.Done() // should not panic
}

// --- Error ---

func TestStream_Error(t *testing.T) {
	s := agentic.NewStream[int](5)

	go func() {
		s.Enqueue(1)
		s.Error(fmt.Errorf("broken"))
	}()

	values, err := s.Collect()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
	assert.Equal(t, []int{1}, values)
}

// --- Enqueue after close ---

func TestStream_EnqueueAfterClose(t *testing.T) {
	s := agentic.NewStream[int](5)
	s.Done()
	ok := s.Enqueue(42)
	assert.False(t, ok, "enqueue after close should return false")
}

// --- Chan single-use ---

func TestStream_Chan_SingleUse(t *testing.T) {
	s := agentic.NewStream[int](5)
	_ = s.Chan()

	assert.Panics(t, func() {
		_ = s.Chan()
	})
}

// --- Backpressure ---

func TestStream_Backpressure(t *testing.T) {
	s := agentic.NewStream[int](2)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			s.Enqueue(i)
		}
		s.Done()
	}()

	values, err := s.Collect()
	require.NoError(t, err)
	assert.Len(t, values, 10)
	wg.Wait()
}

// --- Transform ---

func TestTransform(t *testing.T) {
	input := agentic.NewStream[int](5)

	go func() {
		input.Enqueue(1)
		input.Enqueue(2)
		input.Enqueue(3)
		input.Done()
	}()

	output := agentic.Transform(input, func(v int) (string, error) {
		return fmt.Sprintf("v%d", v), nil
	})

	values, err := output.Collect()
	require.NoError(t, err)
	assert.Equal(t, []string{"v1", "v2", "v3"}, values)
}

func TestTransform_Error(t *testing.T) {
	input := agentic.NewStream[int](5)

	go func() {
		input.Enqueue(1)
		input.Enqueue(2)
		input.Done()
	}()

	output := agentic.Transform(input, func(v int) (string, error) {
		if v == 2 {
			return "", fmt.Errorf("bad value")
		}
		return fmt.Sprintf("v%d", v), nil
	})

	values, err := output.Collect()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad value")
	assert.Equal(t, []string{"v1"}, values)
}

func TestTransform_InputError(t *testing.T) {
	input := agentic.NewStream[int](5)

	go func() {
		input.Enqueue(1)
		input.Error(fmt.Errorf("input error"))
	}()

	output := agentic.Transform(input, func(v int) (string, error) {
		return fmt.Sprintf("v%d", v), nil
	})

	_, err := output.Collect()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input error")
}

// --- Concurrent producer ---

func TestStream_ConcurrentProducer(t *testing.T) {
	s := agentic.NewStream[int](100)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			s.Enqueue(v)
		}(i)
	}

	go func() {
		wg.Wait()
		s.Done()
	}()

	values, err := s.Collect()
	require.NoError(t, err)
	assert.Len(t, values, 10)
}
