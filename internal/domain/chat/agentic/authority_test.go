package agentic

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthority_LevelEnumIsBounded(t *testing.T) {
	for _, l := range AllAuthorityLevels() {
		assert.True(t, IsValidAuthorityLevel(l))
	}
	assert.False(t, IsValidAuthorityLevel(AuthorityLevel(99)))
	assert.False(t, IsValidAuthorityLevel(AuthorityLevel(-1)))
}

func TestAuthority_AllLevelsCount(t *testing.T) {
	// 6 levels: platform / enterprise / tenant / agent / user / runtime.
	assert.Equal(t, 6, len(AllAuthorityLevels()),
		"6 authority levels expected")
}

func TestAuthority_LevelStringIsStable(t *testing.T) {
	cases := map[AuthorityLevel]string{
		AuthorityPlatform:   "platform",
		AuthorityEnterprise: "enterprise",
		AuthorityTenant:     "tenant",
		AuthorityAgent:      "agent",
		AuthorityUser:       "user",
		AuthorityRuntime:    "runtime",
	}
	for level, str := range cases {
		assert.Equal(t, str, level.String(), "level %d wire string", level)
	}
}

func TestAuthority_LevelOrderingIsCorrect(t *testing.T) {
	assert.True(t, IsLowerAuthority(AuthorityPlatform, AuthorityRuntime))
	assert.True(t, IsHigherAuthority(AuthorityRuntime, AuthorityPlatform))
	assert.False(t, IsLowerAuthority(AuthorityRuntime, AuthorityPlatform))
	assert.False(t, IsHigherAuthority(AuthorityPlatform, AuthorityRuntime))
}

func TestResolver_EmptyLayersReturnsEmpty(t *testing.T) {
	r := NewLayeredResolver()
	got, err := r.Resolve(nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestResolver_SingleLayerReturnsValues(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{Level: AuthorityPlatform, Settings: map[string]string{"x": "1", "y": "2"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "1", got["x"].Value)
	assert.Equal(t, "2", got["y"].Value)
	assert.Equal(t, AuthorityPlatform, got["x"].SourceLevel)
}

func TestResolver_HigherLayerOverridesLower(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{Level: AuthorityPlatform, Settings: map[string]string{"temperature": "0.7"}},
		{Level: AuthorityTenant, Settings: map[string]string{"temperature": "0.5"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "0.5", got["temperature"].Value, "tenant overrides platform")
	assert.Equal(t, AuthorityTenant, got["temperature"].SourceLevel)
}

func TestResolver_LayerOrderIsByLevelNotInput(t *testing.T) {
	r := NewLayeredResolver()
	// Input order: high → low. Resolver must sort low → high before applying.
	layers := []AuthorityLayer{
		{Level: AuthorityRuntime, Settings: map[string]string{"x": "runtime"}},
		{Level: AuthorityPlatform, Settings: map[string]string{"x": "platform"}},
		{Level: AuthorityTenant, Settings: map[string]string{"x": "tenant"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "runtime", got["x"].Value, "runtime is highest, wins regardless of input order")
}

func TestResolver_LockFromLowerLayerBlocksHigherOverride(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{
			Level:    AuthorityPlatform,
			Settings: map[string]string{"audit_retention_days": "365"},
			Locked:   map[string]string{"audit_retention_days": "compliance baseline"},
		},
		{
			Level:    AuthorityTenant,
			Settings: map[string]string{"audit_retention_days": "30"}, // attempted override
		},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "365", got["audit_retention_days"].Value,
		"locked by platform; tenant override silently dropped")
	assert.True(t, got["audit_retention_days"].IsLocked)
	assert.Equal(t, AuthorityPlatform, got["audit_retention_days"].LockSourceLevel)
	assert.Equal(t, "compliance baseline", got["audit_retention_days"].LockReason)
}

func TestResolver_FirstLockWinsAcrossLayers(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{
			Level:    AuthorityPlatform,
			Settings: map[string]string{"x": "1"},
			Locked:   map[string]string{"x": "platform lock"},
		},
		{
			Level:  AuthorityTenant,
			Locked: map[string]string{"x": "tenant lock"},
		},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "platform lock", got["x"].LockReason,
		"first lock wins; tenant lock ignored")
	assert.Equal(t, AuthorityPlatform, got["x"].LockSourceLevel)
}

func TestResolver_LockWithNoValueRecordsEmptyLockedSetting(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{Level: AuthorityPlatform, Locked: map[string]string{"feature_flag_x": "kill switch"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	require.Contains(t, got, "feature_flag_x")
	assert.Equal(t, "", got["feature_flag_x"].Value)
	assert.True(t, got["feature_flag_x"].IsLocked)
	assert.Equal(t, "kill switch", got["feature_flag_x"].LockReason)
}

func TestResolver_IntegrityCheck_ErrorsOnLockedOverrideAttempt(t *testing.T) {
	r := NewLayeredResolver().WithIntegrityCheck()
	layers := []AuthorityLayer{
		{
			Level:    AuthorityPlatform,
			Settings: map[string]string{"x": "1"},
			Locked:   map[string]string{"x": "locked"},
		},
		{
			Level:    AuthorityTenant,
			Settings: map[string]string{"x": "2"},
		},
	}
	_, err := r.Resolve(layers)
	assert.True(t, errors.Is(err, ErrLockedKeyOverrideAttempt),
		"integrity check must error on attempted override")
}

func TestResolver_IntegrityCheck_NoErrorWhenLayersAgree(t *testing.T) {
	r := NewLayeredResolver().WithIntegrityCheck()
	layers := []AuthorityLayer{
		{Level: AuthorityPlatform, Settings: map[string]string{"x": "1"}, Locked: map[string]string{"x": "ok"}},
		// Tenant doesn't try to override "x" — only sets a different key.
		{Level: AuthorityTenant, Settings: map[string]string{"y": "2"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "1", got["x"].Value)
	assert.Equal(t, "2", got["y"].Value)
}

func TestResolver_RejectsInvalidLevel(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{Level: AuthorityLevel(99), Settings: map[string]string{"x": "1"}},
	}
	_, err := r.Resolve(layers)
	assert.True(t, errors.Is(err, ErrInvalidAuthorityLevel))
}

func TestResolver_AttributionTracksScopeID(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{Level: AuthorityPlatform, ScopeID: "global", Settings: map[string]string{"x": "default"}},
		{Level: AuthorityTenant, ScopeID: "tenant-acme", Settings: map[string]string{"x": "acme-value"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "tenant-acme", got["x"].SourceScopeID,
		"audit must know WHICH tenant overrode")
}

func TestResolver_ResolveKey_Hit(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{Level: AuthorityPlatform, Settings: map[string]string{"x": "1"}},
	}
	v, ok, err := r.ResolveKey(layers, "x")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "1", v.Value)
}

func TestResolver_ResolveKey_Miss(t *testing.T) {
	r := NewLayeredResolver()
	v, ok, err := r.ResolveKey(nil, "x")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "", v.Value)
}

func TestResolver_RuntimeOverridesEverythingExceptLocks(t *testing.T) {
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{Level: AuthorityPlatform, Settings: map[string]string{"x": "platform"}},
		{Level: AuthorityTenant, Settings: map[string]string{"x": "tenant"}},
		{Level: AuthorityAgent, Settings: map[string]string{"x": "agent"}},
		{Level: AuthorityUser, Settings: map[string]string{"x": "user"}},
		{Level: AuthorityRuntime, Settings: map[string]string{"x": "runtime"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "runtime", got["x"].Value)
	assert.Equal(t, AuthorityRuntime, got["x"].SourceLevel)
}

func TestResolver_LockBlocksEvenRuntime(t *testing.T) {
	// Lock semantics: a platform lock holds against EVERY higher layer
	// including runtime — security baselines cannot be bypassed.
	r := NewLayeredResolver()
	layers := []AuthorityLayer{
		{
			Level:    AuthorityPlatform,
			Settings: map[string]string{"x": "platform-locked"},
			Locked:   map[string]string{"x": "security baseline"},
		},
		{Level: AuthorityRuntime, Settings: map[string]string{"x": "runtime-attempt"}},
	}
	got, err := r.Resolve(layers)
	require.NoError(t, err)
	assert.Equal(t, "platform-locked", got["x"].Value,
		"runtime cannot bypass platform lock — security baseline invariant")
}
