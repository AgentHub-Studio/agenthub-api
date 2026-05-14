package agentic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_LazyInstructionLoader(t *testing.T) {
	t.Run("Scenario_FirstReferenceTriggersLoadSubsequentHitCache", func(t *testing.T) {
		// Given the agent registers 100 instruction sources but only
		// references 5 in this run,
		// When the runtime calls Load() for those 5,
		// Then only 5 loaders fire — the other 95 stay cold (cold-start
		// cost saved per PDF §7.9).
		l := NewInMemoryLazyInstructionLoader()
		var calls int32
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt32(&calls, 1)
			return "ok", nil
		}
		for _, slug := range []string{"a", "b", "c", "d", "e", "unused-1", "unused-2", "unused-3"} {
			_ = l.Register(context.Background(), slug, "", time.Hour, loader)
		}
		// Reference only 5.
		for _, slug := range []string{"a", "b", "c", "d", "e"} {
			_, _ = l.Load(context.Background(), slug)
		}
		assert.Equal(t, int32(5), atomic.LoadInt32(&calls),
			"only referenced sources triggered load; unused stay cold")
	})

	t.Run("Scenario_TTLExpiryReloadsForFreshness", func(t *testing.T) {
		// Given an instruction source has TTL=1h to keep content fresh,
		// When 2h elapse and the source is re-referenced,
		// Then the loader re-fires (caller gets fresh content; stale
		// cache doesn't pin agent to old data).
		l := NewInMemoryLazyInstructionLoader()
		now := time.Now()
		l.SetClock(func() time.Time { return now })
		var calls int32
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt32(&calls, 1)
			return "fresh", nil
		}
		_ = l.Register(context.Background(), "x", "", time.Hour, loader)
		_, _ = l.Load(context.Background(), "x")
		now = now.Add(2 * time.Hour)
		_, _ = l.Load(context.Background(), "x")
		assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
	})

	t.Run("Scenario_LoaderErrorCachesFailureToAvoidThundering", func(t *testing.T) {
		// Given an upstream is down and the loader errors,
		// When agent re-references the instruction many times in this
		// turn (e.g. matched by 5 different rules),
		// Then loader is called ONCE — failure is cached so agent
		// doesn't hammer the down upstream (thundering herd guard).
		l := NewInMemoryLazyInstructionLoader()
		var calls int32
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt32(&calls, 1)
			return "", errors.New("upstream down")
		}
		_ = l.Register(context.Background(), "x", "", time.Hour, loader)
		for i := 0; i < 10; i++ {
			_, _ = l.Load(context.Background(), "x")
		}
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls),
			"cached failure prevents thundering herd")
	})

	t.Run("Scenario_AdminInvalidateClearsCacheToForceFreshLoad", func(t *testing.T) {
		// Given admin pushed updated content upstream,
		// When admin calls Invalidate(slug),
		// Then next Load() invokes loader fn — fresh content fetched,
		// no stale cache pinning to old version.
		l := NewInMemoryLazyInstructionLoader()
		var version int32
		loader := func(ctx context.Context) (string, error) {
			v := atomic.AddInt32(&version, 1)
			if v == 1 {
				return "v1", nil
			}
			return "v2", nil
		}
		_ = l.Register(context.Background(), "x", "", time.Hour, loader)
		first, _ := l.Load(context.Background(), "x")
		assert.Equal(t, "v1", first)

		_ = l.Invalidate(context.Background(), "x")
		second, _ := l.Load(context.Background(), "x")
		assert.Equal(t, "v2", second, "post-invalidate fetches fresh")
	})

	t.Run("Scenario_FailureRecoveryRequiresExplicitInvalidate", func(t *testing.T) {
		// Given a transient failure cached on first call,
		// When upstream recovers,
		// Then admin must explicitly Invalidate to retry — no automatic
		// retry on cached failure (avoids hidden retry-storms).
		l := NewInMemoryLazyInstructionLoader()
		var calls int32
		failThenOk := func(ctx context.Context) (string, error) {
			c := atomic.AddInt32(&calls, 1)
			if c == 1 {
				return "", errors.New("transient")
			}
			return "ok", nil
		}
		_ = l.Register(context.Background(), "x", "", time.Hour, failThenOk)
		_, err := l.Load(context.Background(), "x")
		require.Error(t, err)

		// Without invalidate, repeats fail.
		_, err = l.Load(context.Background(), "x")
		require.Error(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "no auto-retry")

		// Explicit invalidate enables retry.
		_ = l.Invalidate(context.Background(), "x")
		got, err := l.Load(context.Background(), "x")
		require.NoError(t, err)
		assert.Equal(t, "ok", got)
	})

	t.Run("Scenario_StatsExposeCachePerformanceForObs009", func(t *testing.T) {
		// Given OBS-009 quality reports include cache effectiveness,
		// When admin queries Stats(),
		// Then it reports loaded/expired/failed counts + cumulative
		// load count for the cache-hit-rate metric.
		l := NewInMemoryLazyInstructionLoader()
		_ = l.Register(context.Background(), "ok", "", time.Hour, staticLoader("c"))
		_ = l.Register(context.Background(), "broken", "", time.Hour, failingLoader("err"))
		_, _ = l.Load(context.Background(), "ok")
		_, _ = l.Load(context.Background(), "ok")  // cache hit
		_, _ = l.Load(context.Background(), "broken")
		stats, _ := l.Stats(context.Background())
		assert.Equal(t, 2, stats.TotalRegistered)
		assert.Equal(t, 1, stats.TotalLoaded)
		assert.Equal(t, 1, stats.TotalFailed)
	})

	t.Run("Scenario_FindReturnsDescriptorWithoutTriggeringLoad", func(t *testing.T) {
		// Given admin browses registered instructions in the UI,
		// When Find(slug) is called,
		// Then loader is NOT invoked — UI can show metadata without
		// hammering loaders for every list-row.
		l := NewInMemoryLazyInstructionLoader()
		var calls int32
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt32(&calls, 1)
			return "", nil
		}
		_ = l.Register(context.Background(), "x", "d", time.Hour, loader)
		_, err := l.Find(context.Background(), "x")
		require.NoError(t, err)
		assert.Equal(t, int32(0), atomic.LoadInt32(&calls), "Find pure metadata read")
	})

	t.Run("Scenario_ZeroTTLNeverExpiresForStablePlatformSources", func(t *testing.T) {
		// Given platform-stable sources (e.g. immutable rule definitions),
		// When admin registers with TTL=0,
		// Then content never expires — loaded once, cached forever
		// until explicit Invalidate. Saves repeated reload cost.
		l := NewInMemoryLazyInstructionLoader()
		var calls int32
		loader := func(ctx context.Context) (string, error) {
			atomic.AddInt32(&calls, 1)
			return "stable", nil
		}
		now := time.Now()
		l.SetClock(func() time.Time { return now })
		_ = l.Register(context.Background(), "x", "d", 0, loader) // TTL=0
		_, _ = l.Load(context.Background(), "x")
		now = now.Add(1000 * time.Hour)
		_, _ = l.Load(context.Background(), "x")
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
	})

	t.Run("Scenario_LoaderInheritsCallerContextForTracing", func(t *testing.T) {
		// Given OBS-002 distributed tracing propagates trace-id via context,
		// When agent calls Load(ctx),
		// Then the loader fn receives the SAME ctx — its upstream calls
		// inherit the trace-id automatically.
		l := NewInMemoryLazyInstructionLoader()
		type ctxKey string
		const key ctxKey = "trace"
		var seen string
		loader := func(ctx context.Context) (string, error) {
			if v, ok := ctx.Value(key).(string); ok {
				seen = v
			}
			return "ok", nil
		}
		_ = l.Register(context.Background(), "x", "d", time.Hour, loader)
		ctxWith := context.WithValue(context.Background(), key, "trace-abc")
		_, _ = l.Load(ctxWith, "x")
		assert.Equal(t, "trace-abc", seen)
	})

	t.Run("Scenario_DuplicateRegisterRejectedToPreventLoaderOverwrite", func(t *testing.T) {
		// Given the slug is the permanent identifier,
		// When admin Re-Registers same slug (intends update),
		// Then rejection prevents accidental loader overwrite — admin
		// must Invalidate explicitly (lifecycle clarity).
		l := NewInMemoryLazyInstructionLoader()
		_ = l.Register(context.Background(), "x", "d", time.Hour, staticLoader("a"))
		err := l.Register(context.Background(), "x", "d", time.Hour, staticLoader("b"))
		assert.True(t, errors.Is(err, ErrLazyInstructionDuplicate))
	})
}
