package agentic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func staticLoader(content string) LazyInstructionLoadFn {
	return func(ctx context.Context) (string, error) {
		return content, nil
	}
}

func failingLoader(msg string) LazyInstructionLoadFn {
	return func(ctx context.Context) (string, error) {
		return "", errors.New(msg)
	}
}

func TestLazyInstruction_StatusEnumIsBounded(t *testing.T) {
	for _, s := range AllLazyInstructionStatuses() {
		assert.True(t, IsValidLazyInstructionStatus(s))
	}
	assert.False(t, IsValidLazyInstructionStatus(LazyInstructionStatus("hibernating")))
	assert.Equal(t, 4, len(AllLazyInstructionStatuses()))
}

func TestLazyInstruction_IsExpired_ZeroTTLNeverExpires(t *testing.T) {
	i := LazyInstruction{
		Status: LazyInstructionStatusLoaded,
		LoadedAt: time.Now().Add(-100 * time.Hour),
		TTL: 0,
	}
	assert.False(t, i.IsExpired(time.Now()))
}

func TestLazyInstruction_IsExpired_WhenLoadedAndPastTTL(t *testing.T) {
	now := time.Now()
	i := LazyInstruction{
		Status: LazyInstructionStatusLoaded,
		LoadedAt: now.Add(-2 * time.Hour),
		TTL: time.Hour,
	}
	assert.True(t, i.IsExpired(now))
}

func TestLazyInstruction_IsExpired_NotWhenStillRegistered(t *testing.T) {
	i := LazyInstruction{Status: LazyInstructionStatusRegistered, TTL: time.Hour}
	assert.False(t, i.IsExpired(time.Now()))
}

func TestLazy_Register_CreatesDescriptor(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	err := l.Register(context.Background(), "rule-x", "desc", time.Hour, staticLoader("content"))
	require.NoError(t, err)
	instr, err := l.Find(context.Background(), "rule-x")
	require.NoError(t, err)
	assert.Equal(t, LazyInstructionStatusRegistered, instr.Status)
	assert.Equal(t, "", instr.Content, "no load yet")
}

func TestLazy_Register_RejectsEmptySlug(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	err := l.Register(context.Background(), " ", "desc", time.Hour, staticLoader("c"))
	assert.True(t, errors.Is(err, ErrLazyInstructionSlugEmpty))
}

func TestLazy_Register_RejectsNilLoader(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	err := l.Register(context.Background(), "x", "desc", time.Hour, nil)
	assert.True(t, errors.Is(err, ErrLazyInstructionLoaderNil))
}

func TestLazy_Register_RejectsDuplicate(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, staticLoader("c")))
	err := l.Register(context.Background(), "x", "d", time.Hour, staticLoader("c"))
	assert.True(t, errors.Is(err, ErrLazyInstructionDuplicate))
}

func TestLazy_Load_InvokesLoaderOnFirstCall(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	var calls int32
	loader := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "loaded content", nil
	}
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, loader))
	got, err := l.Load(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, "loaded content", got)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestLazy_Load_HitsCacheOnSecondCall(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	var calls int32
	loader := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "x", nil
	}
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, loader))
	_, _ = l.Load(context.Background(), "x")
	_, _ = l.Load(context.Background(), "x")
	_, _ = l.Load(context.Background(), "x")
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "cache hits avoid re-loading")
}

func TestLazy_Load_ReinvokesAfterTTLExpiry(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	now := time.Now()
	l.SetClock(func() time.Time { return now })
	var calls int32
	loader := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return fmt.Sprintf("call-%d", atomic.LoadInt32(&calls)), nil
	}
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, loader))
	_, _ = l.Load(context.Background(), "x")
	now = now.Add(2 * time.Hour) // past TTL
	_, _ = l.Load(context.Background(), "x")
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "expired triggers reload")
}

func TestLazy_Load_UnknownReturnsNotFound(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	_, err := l.Load(context.Background(), "missing")
	assert.True(t, errors.Is(err, ErrLazyInstructionNotFound))
}

func TestLazy_Load_LoaderErrorCachesFailure(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	var calls int32
	loader := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", errors.New("upstream down")
	}
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, loader))
	_, err1 := l.Load(context.Background(), "x")
	assert.True(t, errors.Is(err1, ErrLazyInstructionLastFailed))

	// Second call returns cached failure WITHOUT calling loader again.
	_, err2 := l.Load(context.Background(), "x")
	assert.True(t, errors.Is(err2, ErrLazyInstructionLastFailed))
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "failure cached; loader not re-called")
}

func TestLazy_Invalidate_AllowsRetryAfterFailure(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	var calls int32
	failThenSucceed := func(ctx context.Context) (string, error) {
		c := atomic.AddInt32(&calls, 1)
		if c == 1 {
			return "", errors.New("transient")
		}
		return "ok", nil
	}
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, failThenSucceed))

	_, err := l.Load(context.Background(), "x")
	assert.Error(t, err)

	require.NoError(t, l.Invalidate(context.Background(), "x"))
	got, err := l.Load(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, "ok", got)
}

func TestLazy_Invalidate_UnknownReturnsNotFound(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	err := l.Invalidate(context.Background(), "missing")
	assert.True(t, errors.Is(err, ErrLazyInstructionNotFound))
}

func TestLazy_InvalidateAll_ClearsAllCaches(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	require.NoError(t, l.Register(context.Background(), "a", "d", time.Hour, staticLoader("ax")))
	require.NoError(t, l.Register(context.Background(), "b", "d", time.Hour, staticLoader("bx")))
	_, _ = l.Load(context.Background(), "a")
	_, _ = l.Load(context.Background(), "b")
	require.NoError(t, l.InvalidateAll(context.Background()))

	all, _ := l.List(context.Background())
	for _, instr := range all {
		assert.Equal(t, LazyInstructionStatusRegistered, instr.Status)
		assert.Empty(t, instr.Content)
	}
}

func TestLazy_Find_ReturnsDescriptorWithoutLoading(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	var calls int32
	loader := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "x", nil
	}
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, loader))
	instr, err := l.Find(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, LazyInstructionStatusRegistered, instr.Status)
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls), "Find does not invoke loader")
}

func TestLazy_List_OrderedBySlug(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	for _, slug := range []string{"zeta", "alpha", "mid"} {
		require.NoError(t, l.Register(context.Background(), slug, "d", time.Hour, staticLoader("c")))
	}
	got, err := l.List(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.Less(t, got[i-1].Slug, got[i].Slug)
	}
}

func TestLazy_ListByStatus_FiltersStrictly(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	require.NoError(t, l.Register(context.Background(), "loaded", "d", time.Hour, staticLoader("x")))
	require.NoError(t, l.Register(context.Background(), "registered", "d", time.Hour, staticLoader("x")))
	_, _ = l.Load(context.Background(), "loaded")

	loaded, _ := l.ListByStatus(context.Background(), LazyInstructionStatusLoaded)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "loaded", loaded[0].Slug)

	reg, _ := l.ListByStatus(context.Background(), LazyInstructionStatusRegistered)
	assert.Len(t, reg, 1)
	assert.Equal(t, "registered", reg[0].Slug)
}

func TestLazy_Stats_SummarizesPerformance(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	require.NoError(t, l.Register(context.Background(), "a", "d", time.Hour, staticLoader("ax")))
	require.NoError(t, l.Register(context.Background(), "b", "d", time.Hour, failingLoader("oops")))
	_, _ = l.Load(context.Background(), "a")
	_, _ = l.Load(context.Background(), "a") // cache hit
	_, _ = l.Load(context.Background(), "b")

	stats, err := l.Stats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalRegistered)
	assert.Equal(t, 1, stats.TotalLoaded)
	assert.Equal(t, 1, stats.TotalFailed)
	assert.Equal(t, 2, stats.CumulativeLoads, "1 successful + 1 failed = 2 invocations")
}

func TestLazy_LoadCount_IncrementsOnInvocation(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	now := time.Now()
	l.SetClock(func() time.Time { return now })
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, staticLoader("c")))
	_, _ = l.Load(context.Background(), "x")
	_, _ = l.Load(context.Background(), "x") // hit cache; no increment
	now = now.Add(2 * time.Hour)
	_, _ = l.Load(context.Background(), "x") // expired; increment

	instr, _ := l.Find(context.Background(), "x")
	assert.Equal(t, 2, instr.LoadCount, "1 first-load + 1 reload after expiry = 2")
}

func TestLazy_LoaderReceivesOriginalContext(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	type ctxKey string
	const key ctxKey = "trace-id"
	var seen string
	loader := func(ctx context.Context) (string, error) {
		if v, ok := ctx.Value(key).(string); ok {
			seen = v
		}
		return "ok", nil
	}
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, loader))
	ctxWith := context.WithValue(context.Background(), key, "trace-123")
	_, _ = l.Load(ctxWith, "x")
	assert.Equal(t, "trace-123", seen, "loader receives caller's context")
}

func TestLazy_ConcurrentLoadIsSafe(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, staticLoader("c")))
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = l.Load(context.Background(), "x")
		}()
	}
	wg.Wait()
}

func TestLazy_ContextCancelled(t *testing.T) {
	l := NewInMemoryLazyInstructionLoader()
	require.NoError(t, l.Register(context.Background(), "x", "d", time.Hour, staticLoader("c")))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := l.Register(ctx, "y", "d", time.Hour, staticLoader("c"))
	assert.Error(t, err)
	_, err = l.Load(ctx, "x")
	assert.Error(t, err)
	_, err = l.Find(ctx, "x")
	assert.Error(t, err)
	err = l.Invalidate(ctx, "x")
	assert.Error(t, err)
	err = l.InvalidateAll(ctx)
	assert.Error(t, err)
	_, err = l.List(ctx)
	assert.Error(t, err)
	_, err = l.ListByStatus(ctx, LazyInstructionStatusLoaded)
	assert.Error(t, err)
	_, err = l.Stats(ctx)
	assert.Error(t, err)
}
