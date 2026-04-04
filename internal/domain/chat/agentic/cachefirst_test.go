package agentic_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewCacheFirstStore ---

func TestNewCacheFirstStore(t *testing.T) {
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		return "val", nil
	})
	assert.NotNil(t, s)
	assert.False(t, s.IsFresh())
}

func TestNewCacheFirstStore_DefaultTTL(t *testing.T) {
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{}, func() (string, error) {
		return "val", nil
	})
	assert.NotNil(t, s)
}

// --- Get with no cache ---

func TestCacheFirst_Get_NoCacheTriggersFetch(t *testing.T) {
	var fetched atomic.Int32
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		fetched.Add(1)
		return "remote", nil
	})

	result := s.Get()
	assert.False(t, result.Success, "no cache should return failure")

	// Background fetch should complete
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), fetched.Load())

	result = s.Get()
	assert.True(t, result.Success)
	assert.Equal(t, "remote", result.Data)
}

// --- Get with fresh cache ---

func TestCacheFirst_Get_FreshCache(t *testing.T) {
	var fetched atomic.Int32
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		fetched.Add(1)
		return "remote", nil
	})
	s.Set("cached")

	result := s.Get()
	assert.True(t, result.Success)
	assert.Equal(t, "cached", result.Data)
	assert.Equal(t, int32(0), fetched.Load(), "should not fetch when fresh")
}

// --- Get with stale cache ---

func TestCacheFirst_Get_StaleCache(t *testing.T) {
	var fetched atomic.Int32
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: 1 * time.Millisecond}, func() (string, error) {
		fetched.Add(1)
		return "refreshed", nil
	})
	s.Set("old")
	time.Sleep(5 * time.Millisecond)

	result := s.Get()
	assert.True(t, result.Success, "stale cache still returns success")
	assert.Equal(t, "old", result.Data, "returns stale value immediately")

	// Background refresh
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), fetched.Load())

	result = s.Get()
	assert.Equal(t, "refreshed", result.Data)
}

// --- GetSync ---

func TestCacheFirst_GetSync_Success(t *testing.T) {
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		return "sync_val", nil
	})

	result := s.GetSync()
	assert.True(t, result.Success)
	assert.Equal(t, "sync_val", result.Data)
	assert.True(t, s.IsFresh())
}

func TestCacheFirst_GetSync_Error_NoCached(t *testing.T) {
	var errCount atomic.Int32
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{
		TTL: time.Minute,
		OnError: func(err error) {
			errCount.Add(1)
		},
	}, func() (string, error) {
		return "", errors.New("fail")
	})

	result := s.GetSync()
	assert.False(t, result.Success)
	assert.Equal(t, int32(1), errCount.Load())
}

func TestCacheFirst_GetSync_Error_ReturnsCached(t *testing.T) {
	first := true
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		if first {
			first = false
			return "good", nil
		}
		return "", errors.New("fail")
	})

	r1 := s.GetSync()
	require.True(t, r1.Success)

	r2 := s.GetSync()
	assert.True(t, r2.Success, "should return cached on error")
	assert.Equal(t, "good", r2.Data)
}

// --- Invalidate ---

func TestCacheFirst_Invalidate(t *testing.T) {
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		return "v", nil
	})
	s.Set("cached")
	assert.True(t, s.IsFresh())

	s.Invalidate()
	assert.False(t, s.IsFresh())

	result := s.Get()
	assert.False(t, result.Success)
}

// --- Set ---

func TestCacheFirst_Set(t *testing.T) {
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		return "", nil
	})
	s.Set("manual")
	result := s.Get()
	assert.True(t, result.Success)
	assert.Equal(t, "manual", result.Data)
}

// --- IsFresh ---

func TestCacheFirst_IsFresh_Expired(t *testing.T) {
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: 1 * time.Millisecond}, func() (string, error) {
		return "", nil
	})
	s.Set("v")
	time.Sleep(5 * time.Millisecond)
	assert.False(t, s.IsFresh())
}

// --- Background fetch dedup ---

func TestCacheFirst_BackgroundFetchDedup(t *testing.T) {
	var fetched atomic.Int32
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (string, error) {
		fetched.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "v", nil
	})

	// Trigger multiple Gets before first fetch completes
	s.Get()
	s.Get()
	s.Get()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), fetched.Load(), "should only fetch once")
}

// --- Generic types ---

func TestCacheFirst_IntType(t *testing.T) {
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (int, error) {
		return 42, nil
	})
	r := s.GetSync()
	assert.True(t, r.Success)
	assert.Equal(t, 42, r.Data)
}

func TestCacheFirst_StructType(t *testing.T) {
	type cfg struct {
		Name  string
		Count int
	}
	s := agentic.NewCacheFirstStore(agentic.CacheFirstConfig{TTL: time.Minute}, func() (cfg, error) {
		return cfg{Name: "test", Count: 5}, nil
	})
	r := s.GetSync()
	assert.True(t, r.Success)
	assert.Equal(t, "test", r.Data.Name)
	assert.Equal(t, 5, r.Data.Count)
}
