package agentic_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DefaultFeatureFlagConfig ---

func TestDefaultFeatureFlagConfig(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	assert.Equal(t, 5*time.Minute, cfg.CacheTTL)
	assert.Equal(t, 2*time.Second, cfg.FetchTimeout)
}

// --- NewFeatureFlagManager ---

func TestNewFeatureFlagManager(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	assert.NotNil(t, m)
	assert.Equal(t, 0, m.CacheSize())
}

func TestNewFeatureFlagManager_ZeroTTL(t *testing.T) {
	cfg := agentic.FeatureFlagConfig{CacheTTL: 0}
	m := agentic.NewFeatureFlagManager(cfg)
	assert.NotNil(t, m)
}

// --- SetDefault + Get ---

func TestFeatureFlag_SetDefault_Get(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("theme", "dark")
	assert.Equal(t, "dark", m.Get("theme"))
}

func TestFeatureFlag_Get_NoDefault(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	assert.Nil(t, m.Get("missing"))
}

// --- Typed accessors ---

func TestFeatureFlag_GetBool(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("enabled", true)
	assert.True(t, m.GetBool("enabled"))
}

func TestFeatureFlag_GetBool_NotBool(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("val", "string")
	assert.False(t, m.GetBool("val"))
}

func TestFeatureFlag_GetString(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("name", "test")
	assert.Equal(t, "test", m.GetString("name"))
}

func TestFeatureFlag_GetString_NotString(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("val", 42)
	assert.Equal(t, "", m.GetString("val"))
}

func TestFeatureFlag_GetInt(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("count", 10)
	assert.Equal(t, 10, m.GetInt("count"))
}

func TestFeatureFlag_GetInt_FromFloat64(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("count", float64(42.9))
	assert.Equal(t, 42, m.GetInt("count"))
}

func TestFeatureFlag_GetInt_FromInt64(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("count", int64(99))
	assert.Equal(t, 99, m.GetInt("count"))
}

func TestFeatureFlag_GetInt_NotNumeric(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("val", "abc")
	assert.Equal(t, 0, m.GetInt("val"))
}

func TestFeatureFlag_GetFloat(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("ratio", 0.75)
	assert.InDelta(t, 0.75, m.GetFloat("ratio"), 0.001)
}

func TestFeatureFlag_GetFloat_FromInt(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("ratio", 5)
	assert.InDelta(t, 5.0, m.GetFloat("ratio"), 0.001)
}

func TestFeatureFlag_GetFloat_NotNumeric(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("val", true)
	assert.InDelta(t, 0.0, m.GetFloat("val"), 0.001)
}

// --- Fetch ---

func TestFeatureFlag_Fetch_Success(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		if name == "color" {
			return "blue", true, nil
		}
		return nil, false, nil
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("color", "red")

	val, err := m.Fetch("color")
	require.NoError(t, err)
	assert.Equal(t, "blue", val)

	// Cached value should be returned by Get
	assert.Equal(t, "blue", m.Get("color"))
	assert.Equal(t, 1, m.CacheSize())
}

func TestFeatureFlag_Fetch_NotFound(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		return nil, false, nil
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("x", "default_val")

	val, err := m.Fetch("x")
	require.NoError(t, err)
	assert.Equal(t, "default_val", val)
}

func TestFeatureFlag_Fetch_Error(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		return nil, false, errors.New("network error")
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("x", "fallback")

	val, err := m.Fetch("x")
	assert.Error(t, err)
	assert.Equal(t, "fallback", val)
}

func TestFeatureFlag_Fetch_NilFetchFunc(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	m.SetDefault("x", "val")

	val, err := m.Fetch("x")
	require.NoError(t, err)
	assert.Equal(t, "val", val)
}

// --- Refresh ---

func TestFeatureFlag_Refresh(t *testing.T) {
	calls := make(map[string]int)
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		calls[name]++
		return name + "_val", true, nil
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("a", "da")
	m.SetDefault("b", "db")

	errs := m.Refresh()
	assert.Empty(t, errs)
	assert.Equal(t, 1, calls["a"])
	assert.Equal(t, 1, calls["b"])
	assert.Equal(t, "a_val", m.Get("a"))
	assert.Equal(t, "b_val", m.Get("b"))
}

func TestFeatureFlag_Refresh_WithErrors(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		if name == "bad" {
			return nil, false, errors.New("fail")
		}
		return "ok", true, nil
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("good", "dg")
	m.SetDefault("bad", "db")

	errs := m.Refresh()
	assert.Len(t, errs, 1)
	assert.Contains(t, errs, "bad")
}

// --- Invalidate ---

func TestFeatureFlag_Invalidate(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		return "remote", true, nil
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("x", "local")
	_, _ = m.Fetch("x")
	assert.Equal(t, "remote", m.Get("x"))

	m.Invalidate("x")
	assert.Equal(t, "local", m.Get("x"))
	assert.Equal(t, 0, m.CacheSize())
}

func TestFeatureFlag_InvalidateAll(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		return "r", true, nil
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("a", "da")
	m.SetDefault("b", "db")
	_, _ = m.Fetch("a")
	_, _ = m.Fetch("b")
	assert.Equal(t, 2, m.CacheSize())

	m.InvalidateAll()
	assert.Equal(t, 0, m.CacheSize())
	assert.Equal(t, "da", m.Get("a"))
}

// --- IsStale ---

func TestFeatureFlag_IsStale_NoCached(t *testing.T) {
	m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
	assert.True(t, m.IsStale("x"))
}

func TestFeatureFlag_IsStale_Fresh(t *testing.T) {
	cfg := agentic.DefaultFeatureFlagConfig()
	cfg.FetchFunc = func(name string) (interface{}, bool, error) {
		return "v", true, nil
	}
	m := agentic.NewFeatureFlagManager(cfg)
	_, _ = m.Fetch("x")
	assert.False(t, m.IsStale("x"))
}

func TestFeatureFlag_IsStale_Expired(t *testing.T) {
	cfg := agentic.FeatureFlagConfig{
		CacheTTL: 1 * time.Millisecond,
		FetchFunc: func(name string) (interface{}, bool, error) {
			return "v", true, nil
		},
	}
	m := agentic.NewFeatureFlagManager(cfg)
	_, _ = m.Fetch("x")
	time.Sleep(5 * time.Millisecond)
	assert.True(t, m.IsStale("x"))
}

// --- Get returns default when cache expired ---

func TestFeatureFlag_Get_ReturnsDefaultWhenExpired(t *testing.T) {
	cfg := agentic.FeatureFlagConfig{
		CacheTTL: 1 * time.Millisecond,
		FetchFunc: func(name string) (interface{}, bool, error) {
			return "remote", true, nil
		},
	}
	m := agentic.NewFeatureFlagManager(cfg)
	m.SetDefault("x", "local")
	_, _ = m.Fetch("x")
	assert.Equal(t, "remote", m.Get("x"))

	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, "local", m.Get("x"))
}
