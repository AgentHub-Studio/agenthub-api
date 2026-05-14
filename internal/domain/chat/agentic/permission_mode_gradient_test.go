package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionGradient_Length(t *testing.T) {
	assert.Equal(t, 5, len(PermissionGradient))
}

func TestPermissionGradient_OrderSafestFirst(t *testing.T) {
	assert.Equal(t, PermissionModePlan, PermissionGradient[0])
	assert.Equal(t, PermissionModeBypassPermissions, PermissionGradient[4])
}

func TestPermissionGradient_IsMonotonicallyDecreasing(t *testing.T) {
	assert.True(t, IsPermissionGradientMonotonicallyDecreasing(),
		"§11.3: safety scores must be strictly decreasing along the gradient")
}

func TestPermissionModeGradientManager_ProfilePlan(t *testing.T) {
	m := NewPermissionModeGradientManager()
	p, ok := m.Profile(PermissionModePlan)
	require.True(t, ok)
	assert.Equal(t, 0, p.GradientIndex)
	assert.Equal(t, 100, p.SafetyScore)
	assert.True(t, p.RequiresConfirmation)
	assert.False(t, p.AutoAcceptsEdits)
	assert.False(t, p.AllowsBackgroundRun)
	assert.False(t, p.AllowsToolBypass)
}

func TestPermissionModeGradientManager_ProfileBypass(t *testing.T) {
	m := NewPermissionModeGradientManager()
	p, ok := m.Profile(PermissionModeBypassPermissions)
	require.True(t, ok)
	assert.Equal(t, 4, p.GradientIndex)
	assert.Equal(t, 0, p.SafetyScore)
	assert.False(t, p.RequiresConfirmation)
	assert.True(t, p.AutoAcceptsEdits)
	assert.True(t, p.AllowsBackgroundRun)
	assert.True(t, p.AllowsToolBypass)
}

func TestPermissionModeGradientManager_ProfileDefault(t *testing.T) {
	m := NewPermissionModeGradientManager()
	p, ok := m.Profile(PermissionModeDefault)
	require.True(t, ok)
	assert.Equal(t, 1, p.GradientIndex)
	assert.Equal(t, 80, p.SafetyScore)
	assert.True(t, p.RequiresConfirmation)
	assert.False(t, p.AutoAcceptsEdits)
}

func TestPermissionModeGradientManager_ProfileAcceptEdits(t *testing.T) {
	m := NewPermissionModeGradientManager()
	p, ok := m.Profile(PermissionModeAcceptEdits)
	require.True(t, ok)
	assert.Equal(t, 2, p.GradientIndex)
	assert.False(t, p.RequiresConfirmation)
	assert.True(t, p.AutoAcceptsEdits)
	assert.False(t, p.AllowsBackgroundRun)
}

func TestPermissionModeGradientManager_ProfileAuto(t *testing.T) {
	m := NewPermissionModeGradientManager()
	p, ok := m.Profile(PermissionModeAuto)
	require.True(t, ok)
	assert.Equal(t, 3, p.GradientIndex)
	assert.True(t, p.AllowsBackgroundRun)
	assert.False(t, p.AllowsToolBypass)
}

func TestPermissionModeGradientManager_ProfileUnknown(t *testing.T) {
	m := NewPermissionModeGradientManager()
	_, ok := m.Profile("unknown-mode")
	assert.False(t, ok)
}

func TestPermissionModeGradientManager_ComparePlanSaferThanDefault(t *testing.T) {
	m := NewPermissionModeGradientManager()
	assert.Equal(t, -1, m.Compare(PermissionModePlan, PermissionModeDefault))
}

func TestPermissionModeGradientManager_CompareBypassMoreAutonomousThanAuto(t *testing.T) {
	m := NewPermissionModeGradientManager()
	assert.Equal(t, 1, m.Compare(PermissionModeBypassPermissions, PermissionModeAuto))
}

func TestPermissionModeGradientManager_CompareSameModeEqualsZero(t *testing.T) {
	m := NewPermissionModeGradientManager()
	assert.Equal(t, 0, m.Compare(PermissionModeDefault, PermissionModeDefault))
}

func TestPermissionModeGradientManager_IsSaferThan(t *testing.T) {
	m := NewPermissionModeGradientManager()
	assert.True(t, m.IsSaferThan(PermissionModePlan, PermissionModeBypassPermissions))
	assert.False(t, m.IsSaferThan(PermissionModeAuto, PermissionModeDefault))
	assert.False(t, m.IsSaferThan(PermissionModeDefault, PermissionModeDefault))
}

func TestPermissionModeGradientManager_EscalateFromPlan(t *testing.T) {
	m := NewPermissionModeGradientManager()
	next, ok := m.Escalate(PermissionModePlan)
	require.True(t, ok)
	assert.Equal(t, PermissionModeDefault, next)
}

func TestPermissionModeGradientManager_EscalateFromBypassReturnsFalse(t *testing.T) {
	m := NewPermissionModeGradientManager()
	next, ok := m.Escalate(PermissionModeBypassPermissions)
	assert.False(t, ok)
	assert.Equal(t, PermissionModeBypassPermissions, next)
}

func TestPermissionModeGradientManager_DeescalateFromBypass(t *testing.T) {
	m := NewPermissionModeGradientManager()
	prev, ok := m.Deescalate(PermissionModeBypassPermissions)
	require.True(t, ok)
	assert.Equal(t, PermissionModeAuto, prev)
}

func TestPermissionModeGradientManager_DeescalateFromPlanReturnsFalse(t *testing.T) {
	m := NewPermissionModeGradientManager()
	prev, ok := m.Deescalate(PermissionModePlan)
	assert.False(t, ok)
	assert.Equal(t, PermissionModePlan, prev)
}

func TestPermissionModeGradientManager_AllModesLengthFive(t *testing.T) {
	m := NewPermissionModeGradientManager()
	assert.Equal(t, 5, len(m.AllModes()))
}

func TestPermissionModeGradientManager_AllModesIsCopy(t *testing.T) {
	m := NewPermissionModeGradientManager()
	modes := m.AllModes()
	modes[0] = "mutated"
	assert.Equal(t, PermissionModePlan, PermissionGradient[0], "original gradient must not be mutated")
}

func TestPermissionModeGradientManager_SafestBackgroundCapableIsAuto(t *testing.T) {
	m := NewPermissionModeGradientManager()
	mode, ok := m.SafestBackgroundCapable()
	require.True(t, ok)
	assert.Equal(t, PermissionModeAuto, mode,
		"auto is the safest mode that allows background run per §11.3")
}

func TestPermissionModeGradientManager_ModesRequiringConfirmation(t *testing.T) {
	m := NewPermissionModeGradientManager()
	modes := m.ModesRequiringConfirmation()
	assert.Equal(t, 2, len(modes), "plan and default require confirmation")
	assert.Equal(t, PermissionModePlan, modes[0])
	assert.Equal(t, PermissionModeDefault, modes[1])
}

func TestPermissionModeGradientManager_EscalateFullChain(t *testing.T) {
	m := NewPermissionModeGradientManager()
	current := PermissionModePlan
	chain := []PermissionMode{current}
	for {
		next, ok := m.Escalate(current)
		if !ok {
			break
		}
		chain = append(chain, next)
		current = next
	}
	assert.Equal(t, PermissionGradient, chain, "escalation must traverse full gradient")
}

func TestPermissionModeGradientManager_DeescalateFullChain(t *testing.T) {
	m := NewPermissionModeGradientManager()
	current := PermissionModeBypassPermissions
	chain := []PermissionMode{current}
	for {
		prev, ok := m.Deescalate(current)
		if !ok {
			break
		}
		chain = append(chain, prev)
		current = prev
	}
	// chain should be reverse of gradient
	assert.Equal(t, 5, len(chain))
	assert.Equal(t, PermissionModePlan, chain[4])
}

func TestPermissionMode_AllProfilesHaveAllGradientIndices(t *testing.T) {
	seen := make(map[int]bool)
	for _, mode := range PermissionGradient {
		p := permissionModeProfiles[mode]
		seen[p.GradientIndex] = true
	}
	for i := 0; i < 5; i++ {
		assert.True(t, seen[i], "gradient index %d must be present", i)
	}
}
