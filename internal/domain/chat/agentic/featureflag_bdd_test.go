package agentic_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// BDD scenarios for FeatureFlagManager — mirrors §11.5 of arXiv:2604.14228v1
// (dynamic config with useDynamicConfig pattern: defaults immediately, remote async).

func TestBDD_FeatureFlagManager(t *testing.T) {
	t.Run("Scenario_DefaultsAvailableImmediatelyBeforeRemoteFetch", func(t *testing.T) {
		// Given a manager with a registered default value for a flag
		m := agentic.NewFeatureFlagManager(agentic.DefaultFeatureFlagConfig())
		m.SetDefault("transcript_classifier", false)

		// When we read the flag before any remote fetch has occurred
		got := m.GetBool("transcript_classifier")

		// Then the default is returned instantly (no network call)
		assert.False(t, got)
	})

	t.Run("Scenario_RemoteValueOverridesDefaultAfterSuccessfulFetch", func(t *testing.T) {
		// Given a remote source that enables the TRANSCRIPT_CLASSIFIER flag
		cfg := agentic.DefaultFeatureFlagConfig()
		cfg.FetchFunc = func(name string) (interface{}, bool, error) {
			if name == "transcript_classifier" {
				return true, true, nil
			}
			return nil, false, nil
		}
		m := agentic.NewFeatureFlagManager(cfg)
		m.SetDefault("transcript_classifier", false)

		// When we explicitly fetch from the remote
		val, err := m.Fetch("transcript_classifier")
		require.NoError(t, err)

		// Then the remote value takes precedence
		assert.Equal(t, true, val)
		assert.True(t, m.GetBool("transcript_classifier"))
	})

	t.Run("Scenario_FetchErrorFallsBackToDefaultGracefully", func(t *testing.T) {
		// Given a remote source that returns an error
		cfg := agentic.DefaultFeatureFlagConfig()
		cfg.FetchFunc = func(name string) (interface{}, bool, error) {
			return nil, false, errors.New("config service unavailable")
		}
		m := agentic.NewFeatureFlagManager(cfg)
		m.SetDefault("kairos_heartbeat", "slow")

		// When we fetch and the remote fails
		val, err := m.Fetch("kairos_heartbeat")

		// Then an error is surfaced but the default is still usable
		assert.Error(t, err)
		assert.Equal(t, "slow", val)
		assert.Equal(t, "slow", m.GetString("kairos_heartbeat"))
	})

	t.Run("Scenario_CachedValueExpiresAndFallsBackToDefault", func(t *testing.T) {
		// Given a very short TTL simulating cache expiry
		cfg := agentic.FeatureFlagConfig{
			CacheTTL: 2 * time.Millisecond,
			FetchFunc: func(name string) (interface{}, bool, error) {
				return "remote_val", true, nil
			},
		}
		m := agentic.NewFeatureFlagManager(cfg)
		m.SetDefault("feature_x", "default_val")
		_, _ = m.Fetch("feature_x")
		assert.Equal(t, "remote_val", m.GetString("feature_x"))

		// When the TTL elapses
		time.Sleep(10 * time.Millisecond)

		// Then Get falls back to default (remote not re-fetched automatically)
		assert.Equal(t, "default_val", m.GetString("feature_x"))
	})

	t.Run("Scenario_InvalidateForcesStaleState", func(t *testing.T) {
		// Given a populated cache
		cfg := agentic.DefaultFeatureFlagConfig()
		cfg.FetchFunc = func(name string) (interface{}, bool, error) {
			return "remote", true, nil
		}
		m := agentic.NewFeatureFlagManager(cfg)
		m.SetDefault("x", "default")
		_, _ = m.Fetch("x")
		assert.Equal(t, "remote", m.GetString("x"))
		assert.False(t, m.IsStale("x"))

		// When we invalidate the entry
		m.Invalidate("x")

		// Then the flag is stale and Get returns the default again
		assert.True(t, m.IsStale("x"))
		assert.Equal(t, "default", m.GetString("x"))
	})

	t.Run("Scenario_RefreshBatchUpdatesAllRegisteredFlags", func(t *testing.T) {
		// Given multiple flags registered with defaults
		calls := map[string]int{}
		cfg := agentic.DefaultFeatureFlagConfig()
		cfg.FetchFunc = func(name string) (interface{}, bool, error) {
			calls[name]++
			return name + "_remote", true, nil
		}
		m := agentic.NewFeatureFlagManager(cfg)
		m.SetDefault("alpha", "a")
		m.SetDefault("beta", "b")
		m.SetDefault("gamma", "c")

		// When we call Refresh
		errs := m.Refresh()

		// Then all flags are fetched and no errors returned
		assert.Empty(t, errs)
		assert.Equal(t, 1, calls["alpha"])
		assert.Equal(t, 1, calls["beta"])
		assert.Equal(t, 1, calls["gamma"])
		assert.Equal(t, "alpha_remote", m.GetString("alpha"))
	})
}
