package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for PermissionModeGradient.
// Maps §11.3 "Safety vs. autonomy" architectural trade-off to AgentHub web agents.

func TestBDD_PermissionModeGradient(t *testing.T) {
	t.Run("Scenario_GradientIsMonotonicallyDecreasingInSafetyScore", func(t *testing.T) {
		// Given the §11.3 architectural invariant
		// When the gradient is traversed from plan to bypassPermissions
		// Then each successive mode has a strictly lower safety score
		assert.True(t, IsPermissionGradientMonotonicallyDecreasing(),
			"§11.3 requires a strictly decreasing safety gradient")
		m := NewPermissionModeGradientManager()
		modes := m.AllModes()
		for i := 1; i < len(modes); i++ {
			pa, _ := m.Profile(modes[i-1])
			pb, _ := m.Profile(modes[i])
			assert.Less(t, pb.SafetyScore, pa.SafetyScore,
				"mode %q (%d) must have lower safety score than %q (%d)",
				modes[i], pb.SafetyScore, modes[i-1], pa.SafetyScore)
		}
	})

	t.Run("Scenario_PlanModeRequiresConfirmationForEveryAction", func(t *testing.T) {
		// Given a regulated tenant requiring maximum human oversight
		// When the plan permission mode is selected
		// Then every action requires explicit user confirmation before execution
		m := NewPermissionModeGradientManager()
		p, found := m.Profile(PermissionModePlan)
		require.True(t, found)
		assert.True(t, p.RequiresConfirmation,
			"plan mode must require confirmation for regulated compliance")
		assert.False(t, p.AllowsBackgroundRun,
			"plan mode must not allow background execution")
		assert.Equal(t, 100, p.SafetyScore,
			"plan mode is the safest position on the gradient")
	})

	t.Run("Scenario_BypassPermissionsEnablesFullAutomation", func(t *testing.T) {
		// Given a trusted automation agent (CI/CD pipeline, service agent)
		// When bypassPermissions mode is configured
		// Then tool bypass, background run, and edit auto-acceptance are all enabled
		m := NewPermissionModeGradientManager()
		p, found := m.Profile(PermissionModeBypassPermissions)
		require.True(t, found)
		assert.True(t, p.AllowsToolBypass, "bypassPermissions must bypass permission prompts")
		assert.True(t, p.AllowsBackgroundRun, "bypassPermissions must allow background execution")
		assert.True(t, p.AutoAcceptsEdits, "bypassPermissions must auto-accept file edits")
		assert.Equal(t, 0, p.SafetyScore, "bypassPermissions is the most autonomous mode")
	})

	t.Run("Scenario_EscalateMovesOneStepTowardAutonomy", func(t *testing.T) {
		// Given an agent currently in default permission mode
		// When the operator escalates permissions one step
		// Then the agent moves to acceptEdits without jumping to bypass
		m := NewPermissionModeGradientManager()
		next, ok := m.Escalate(PermissionModeDefault)
		require.True(t, ok, "escalation from default must succeed")
		assert.Equal(t, PermissionModeAcceptEdits, next,
			"one escalation from default must land on acceptEdits")
		assert.True(t, m.IsSaferThan(PermissionModeDefault, next),
			"original mode must be safer than escalated mode")
	})

	t.Run("Scenario_SafestBackgroundCapableModeIsAuto", func(t *testing.T) {
		// Given the KAIROS heartbeat design requires background execution
		// When looking for the most conservative mode that still allows background run
		// Then auto is returned (plan and default do not allow background; bypass is too permissive)
		m := NewPermissionModeGradientManager()
		mode, ok := m.SafestBackgroundCapable()
		require.True(t, ok)
		assert.Equal(t, PermissionModeAuto, mode,
			"auto is the safest KAIROS-compatible permission mode")
	})

	t.Run("Scenario_OnlyPlanAndDefaultRequireHumanConfirmation", func(t *testing.T) {
		// Given the human authority design value requires user oversight for sensitive agents
		// When querying which modes enforce confirmation gates
		// Then exactly plan and default are returned, in gradient order
		m := NewPermissionModeGradientManager()
		modes := m.ModesRequiringConfirmation()
		assert.Equal(t, 2, len(modes),
			"exactly 2 modes must require confirmation (plan and default)")
		assert.Equal(t, PermissionModePlan, modes[0])
		assert.Equal(t, PermissionModeDefault, modes[1])
		// acceptEdits, auto, bypassPermissions must NOT require confirmation
		for _, mode := range []PermissionMode{PermissionModeAcceptEdits, PermissionModeAuto, PermissionModeBypassPermissions} {
			p, _ := m.Profile(mode)
			assert.False(t, p.RequiresConfirmation,
				"mode %q must not require confirmation", mode)
		}
	})
}
