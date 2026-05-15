package agentic

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GOV-006 — Authority hierarchy BDD.
//
// PDF arXiv:2604.14228v1 §11 (governance — enterprise > tenant > user
// settings layering with lock semantics); §6.1 (settings is one of 10
// plugin manifest types). CLAUDE.md describes ah_core/tenant/agent/user
// settings layers; this resolver collapses them into one effective view
// with lock enforcement.
//
// These scenarios validate the layered resolution contract: bounded
// levels, ordered override, first-lock-wins, attribution preserved,
// integrity-check mode for caller-bug detection.

func TestBDD_AuthorityHierarchy(t *testing.T) {

	t.Run("Scenario_PlatformDefaultsAreInheritedByEveryone", func(t *testing.T) {
		// Given a fresh tenant with no overrides,
		// When the resolver collapses ONLY the platform layer,
		// Then the platform values become effective for everyone.
		r := NewLayeredResolver()
		layers := []AuthorityLayer{
			{Level: AuthorityPlatform, ScopeID: "global", Settings: map[string]string{
				"runner.max_iterations": "25",
				"evaluator.fail_threshold": "0.4",
			}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "25", got["runner.max_iterations"].Value)
		assert.Equal(t, AuthorityPlatform, got["runner.max_iterations"].SourceLevel)
	})

	t.Run("Scenario_TenantOverridesPlatformDefault", func(t *testing.T) {
		// Given a tenant tightens the iteration cap (tenant > platform),
		r := NewLayeredResolver()
		layers := []AuthorityLayer{
			{Level: AuthorityPlatform, Settings: map[string]string{"max": "25"}},
			{Level: AuthorityTenant, ScopeID: "tenant-acme", Settings: map[string]string{"max": "10"}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "10", got["max"].Value)
		assert.Equal(t, AuthorityTenant, got["max"].SourceLevel)
		assert.Equal(t, "tenant-acme", got["max"].SourceScopeID,
			"audit knows WHICH tenant overrode")
	})

	t.Run("Scenario_RuntimeOverridesEveryNormalLayer", func(t *testing.T) {
		// Given a per-request runtime override (highest authority),
		// When all 6 layers are present,
		// Then runtime wins.
		r := NewLayeredResolver()
		layers := []AuthorityLayer{
			{Level: AuthorityPlatform, Settings: map[string]string{"x": "p"}},
			{Level: AuthorityEnterprise, Settings: map[string]string{"x": "e"}},
			{Level: AuthorityTenant, Settings: map[string]string{"x": "t"}},
			{Level: AuthorityAgent, Settings: map[string]string{"x": "a"}},
			{Level: AuthorityUser, Settings: map[string]string{"x": "u"}},
			{Level: AuthorityRuntime, Settings: map[string]string{"x": "r"}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "r", got["x"].Value)
	})

	t.Run("Scenario_PlatformLockBlocksAllOverrides_SecurityBaselineInvariant", func(t *testing.T) {
		// Given platform locks audit_retention_days at 365 (compliance),
		// When tenant tries 30 AND user tries 7 AND runtime tries 1,
		// Then the locked value persists — security baseline cannot
		//      be bypassed by any layer including runtime.
		r := NewLayeredResolver()
		layers := []AuthorityLayer{
			{
				Level:    AuthorityPlatform,
				Settings: map[string]string{"audit_retention_days": "365"},
				Locked:   map[string]string{"audit_retention_days": "GDPR retention floor"},
			},
			{Level: AuthorityTenant, Settings: map[string]string{"audit_retention_days": "30"}},
			{Level: AuthorityUser, Settings: map[string]string{"audit_retention_days": "7"}},
			{Level: AuthorityRuntime, Settings: map[string]string{"audit_retention_days": "1"}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "365", got["audit_retention_days"].Value,
			"PLATFORM LOCK invariant — even runtime cannot bypass GDPR floor")
		assert.True(t, got["audit_retention_days"].IsLocked)
		assert.Equal(t, "GDPR retention floor", got["audit_retention_days"].LockReason)
	})

	t.Run("Scenario_FirstLockWinsAuditAttribution", func(t *testing.T) {
		// Given multiple layers attempt to lock the same key,
		// When resolution runs,
		// Then the FIRST locker (lowest layer that locked) is the audit
		//      source — locks are first-write-wins (matches checkpoint).
		r := NewLayeredResolver()
		layers := []AuthorityLayer{
			{Level: AuthorityPlatform, Settings: map[string]string{"x": "1"}, Locked: map[string]string{"x": "platform reason"}},
			{Level: AuthorityTenant, Locked: map[string]string{"x": "tenant reason"}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "platform reason", got["x"].LockReason,
			"platform locked first; tenant lock ignored for attribution")
		assert.Equal(t, AuthorityPlatform, got["x"].LockSourceLevel)
	})

	t.Run("Scenario_LayersUnsortedAreNormalizedByLevel", func(t *testing.T) {
		// Given the caller passes layers in arbitrary order,
		// When the resolver runs,
		// Then it sorts internally — output is deterministic.
		r := NewLayeredResolver()
		// Input: tenant, runtime, platform — unsorted.
		layers := []AuthorityLayer{
			{Level: AuthorityTenant, Settings: map[string]string{"x": "t"}},
			{Level: AuthorityRuntime, Settings: map[string]string{"x": "r"}},
			{Level: AuthorityPlatform, Settings: map[string]string{"x": "p"}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "r", got["x"].Value, "runtime wins — sort independent of input order")
	})

	t.Run("Scenario_IntegrityCheckCatchesAccidentalLockOverride", func(t *testing.T) {
		// Given an admin tooling pipeline must NEVER write a value to
		//       a locked key (would silently get dropped — the kind of
		//       bug operators want to know about),
		// When the resolver runs in IntegrityCheck mode,
		// Then accidental locked-key writes ERROR loudly instead of
		//      silently dropping.
		r := NewLayeredResolver().WithIntegrityCheck()
		layers := []AuthorityLayer{
			{
				Level:    AuthorityPlatform,
				Settings: map[string]string{"x": "1"},
				Locked:   map[string]string{"x": "locked"},
			},
			{Level: AuthorityTenant, Settings: map[string]string{"x": "2"}},
		}
		_, err := r.Resolve(layers)
		assert.True(t, errors.Is(err, ErrLockedKeyOverrideAttempt),
			"integrity mode surfaces silent-drop bugs early")
	})

	t.Run("Scenario_BoundedLevelsRejectInvalid", func(t *testing.T) {
		// Given a caller-bug passes an out-of-range level,
		// When Resolve runs,
		// Then it rejects — never silently routes the layer to position 0.
		r := NewLayeredResolver()
		_, err := r.Resolve([]AuthorityLayer{
			{Level: AuthorityLevel(42), Settings: map[string]string{"x": "1"}},
		})
		assert.True(t, errors.Is(err, ErrInvalidAuthorityLevel))
	})

	t.Run("Scenario_AllSixLevelsHaveStableWireStrings", func(t *testing.T) {
		// Given audit logs / dashboards bind to level strings,
		expected := []string{"platform", "enterprise", "tenant", "agent", "user", "runtime"}
		got := []string{}
		for _, l := range AllAuthorityLevels() {
			got = append(got, l.String())
		}
		assert.Equal(t, expected, got,
			"levels MUST stringify to stable wire labels in low→high order")
	})

	t.Run("Scenario_LockWithoutValueIsLockedEmptyKillSwitch", func(t *testing.T) {
		// Given a kill-switch pattern: lock without value = "this
		//       feature is disabled, no overrides allowed",
		r := NewLayeredResolver()
		layers := []AuthorityLayer{
			{
				Level:  AuthorityPlatform,
				Locked: map[string]string{"experimental_feature_x": "kill switch — security review pending"},
			},
			{Level: AuthorityTenant, Settings: map[string]string{"experimental_feature_x": "enabled"}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "", got["experimental_feature_x"].Value,
			"kill switch keeps value empty — feature stays disabled")
		assert.True(t, got["experimental_feature_x"].IsLocked)
	})

	t.Run("Scenario_AuditAttributionTracksScopeIDAcrossLayers", func(t *testing.T) {
		// Given multi-tenant audit needs to know WHICH tenant + WHICH
		//       agent + WHICH user contributed to a value,
		r := NewLayeredResolver()
		layers := []AuthorityLayer{
			{Level: AuthorityPlatform, ScopeID: "global"},
			{Level: AuthorityTenant, ScopeID: "tenant-acme", Settings: map[string]string{"x": "from-acme"}},
			{Level: AuthorityAgent, ScopeID: "agent-bot-42", Settings: map[string]string{"y": "from-bot-42"}},
		}
		got, _ := r.Resolve(layers)
		assert.Equal(t, "tenant-acme", got["x"].SourceScopeID)
		assert.Equal(t, "agent-bot-42", got["y"].SourceScopeID)
	})

	t.Run("Scenario_NoLayersProducesEmptyResolutionNotPanic", func(t *testing.T) {
		// Given a fresh deploy with no settings configured anywhere,
		r := NewLayeredResolver()
		got, err := r.Resolve(nil)
		require.NoError(t, err)
		assert.Empty(t, got, "empty input → empty output, never panic")
	})

	t.Run("Scenario_HierarchyComparisonHelpersExposeOrdering", func(t *testing.T) {
		// Given downstream code asks "is X higher than Y?",
		// When the comparison helpers are used,
		// Then they return correctly per the level enum order.
		assert.True(t, IsLowerAuthority(AuthorityPlatform, AuthorityTenant))
		assert.True(t, IsHigherAuthority(AuthorityRuntime, AuthorityAgent))
		assert.False(t, IsLowerAuthority(AuthorityRuntime, AuthorityRuntime),
			"strictly lower — equal returns false")
	})
}
