package agentic_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- SequentialFunc ---

func TestSequentialFunc_SerializesConcurrentCalls(t *testing.T) {
	var running int32
	var maxRunning int32

	fn := agentic.SequentialFunc(func(_ context.Context, v int) error {
		cur := atomic.AddInt32(&running, 1)
		for {
			old := atomic.LoadInt32(&maxRunning)
			if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return nil
	})

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			errs <- fn(context.Background(), v)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&maxRunning), "should never run more than 1 at a time")
}

func TestSequentialFunc_PreservesErrors(t *testing.T) {
	expected := errors.New("fail")
	fn := agentic.SequentialFunc(func(_ context.Context, _ int) error {
		return expected
	})

	err := fn(context.Background(), 42)
	assert.Equal(t, expected, err)
}

func TestSequentialFunc_Success(t *testing.T) {
	fn := agentic.SequentialFunc(func(_ context.Context, v string) error {
		return nil
	})
	assert.NoError(t, fn(context.Background(), "ok"))
}

// --- SequentialFuncResult ---

func TestSequentialFuncResult_ReturnsValue(t *testing.T) {
	fn := agentic.SequentialFuncResult(func(_ context.Context, v int) (string, error) {
		return "result", nil
	})

	result, err := fn(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "result", result)
}

func TestSequentialFuncResult_ReturnsError(t *testing.T) {
	expected := errors.New("fail")
	fn := agentic.SequentialFuncResult(func(_ context.Context, v int) (string, error) {
		return "", expected
	})

	_, err := fn(context.Background(), 1)
	assert.Equal(t, expected, err)
}

func TestSequentialFuncResult_Serializes(t *testing.T) {
	var running int32
	var maxRunning int32

	fn := agentic.SequentialFuncResult(func(_ context.Context, _ int) (int, error) {
		cur := atomic.AddInt32(&running, 1)
		for {
			old := atomic.LoadInt32(&maxRunning)
			if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return 0, nil
	})

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			_, err := fn(context.Background(), v)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&maxRunning))
}

// --- SequentialQueue ---

func TestSequentialQueue_EnqueueWait(t *testing.T) {
	sq := agentic.NewSequentialQueue()
	var order []int
	var mu sync.Mutex

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			errs <- sq.EnqueueWait(context.Background(), func(_ context.Context) error {
				time.Sleep(5 * time.Millisecond)
				mu.Lock()
				order = append(order, v)
				mu.Unlock()
				return nil
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, order, 5, "all items should execute")
}

func TestSequentialQueue_EnqueueWait_Error(t *testing.T) {
	sq := agentic.NewSequentialQueue()
	expected := errors.New("fail")

	err := sq.EnqueueWait(context.Background(), func(_ context.Context) error {
		return expected
	})
	assert.Equal(t, expected, err)
}

func TestSequentialQueue_Enqueue_Async(t *testing.T) {
	sq := agentic.NewSequentialQueue()
	errC := sq.Enqueue(context.Background(), func(_ context.Context) error {
		return nil
	})

	err := <-errC
	assert.NoError(t, err)
}

func TestSequentialQueue_SerializesExecution(t *testing.T) {
	sq := agentic.NewSequentialQueue()
	var running int32
	var maxRunning int32

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- sq.EnqueueWait(context.Background(), func(_ context.Context) error {
				cur := atomic.AddInt32(&running, 1)
				for {
					old := atomic.LoadInt32(&maxRunning)
					if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt32(&running, -1)
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&maxRunning))
}

func TestSequentialQueue_ContextCancellation(t *testing.T) {
	sq := agentic.NewSequentialQueue()
	ctx, cancel := context.WithCancel(context.Background())

	// Enqueue a slow task.
	sq.Enqueue(context.Background(), func(_ context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// Cancel context before second task runs.
	cancel()
	err := sq.EnqueueWait(ctx, func(_ context.Context) error {
		return nil
	})

	assert.Error(t, err)
}

func TestSequentialQueue_Pending(t *testing.T) {
	sq := agentic.NewSequentialQueue()

	// Enqueue a blocking task.
	blocker := make(chan struct{})
	sq.Enqueue(context.Background(), func(_ context.Context) error {
		<-blocker
		return nil
	})

	time.Sleep(10 * time.Millisecond) // let processLoop start

	// Enqueue more while blocked.
	sq.Enqueue(context.Background(), func(_ context.Context) error { return nil })
	sq.Enqueue(context.Background(), func(_ context.Context) error { return nil })

	pending := sq.Pending()
	require.GreaterOrEqual(t, pending, 1, "should have pending items")

	close(blocker)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, sq.Pending())
}
