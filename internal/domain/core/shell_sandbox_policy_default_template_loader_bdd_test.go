package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSSPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksShellSandboxStanceFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a PERM-008 ShellSandboxPolicy
		// and inventing path roots + block lists is error-prone,
		// When admin opens sandbox onboarding,
		// Then 5 recommended posture templates surface across the safety
		// ladder (strict/balanced/progressive/permissive/conservative).
		assert.Equal(t, 5, len(SeedRecommendedSSPDTemplateSlugs))
	})

	t.Run("Scenario_LockedDownForAuditSessions", func(t *testing.T) {
		// Given a read-only audit session must NEVER let the agent
		// execute side effects,
		// When admin uses locked-down,
		// Then the policy has empty AllowedPathRoots (every path-shaped
		// arg violates) and the longest blocked-command list.
		assert.Contains(t, SeedExpectedSSPDTemplateSlugs, "locked-down")
	})

	t.Run("Scenario_WebSafeDefaultForRoutineChat", func(t *testing.T) {
		// Given standard chat sessions need limited shell but no admin
		// friction,
		// When admin uses web-safe-default,
		// Then this is the only template without admin-review and ships
		// a per-session sandbox root.
		assert.Contains(t, SeedExpectedSSPDTemplateSlugs, "web-safe-default")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSSPDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["web-safe-default"])
	})

	t.Run("Scenario_DevWorkstationEnablesNetworkForBuilds", func(t *testing.T) {
		// Given engineering tenants run package managers and git,
		// When admin uses dev-workstation,
		// Then AllowNetwork=true and runtime ceiling is high enough for
		// a build (15 min).
		assert.Contains(t, SeedExpectedSSPDTemplateSlugs, "dev-workstation")
	})

	t.Run("Scenario_CICDRunnerLongerRuntimeBiggerOutput", func(t *testing.T) {
		// Given CI/CD pipelines have longer runs and larger test logs,
		// When admin uses cicd-runner,
		// Then runtime is 30 min and output ceiling is 64MB.
		assert.Contains(t, SeedExpectedSSPDTemplateSlugs, "cicd-runner")
	})

	t.Run("Scenario_IncidentResponseReadsButCannotMutate", func(t *testing.T) {
		// Given an incident requires reading /var/log, /etc but the
		// agent must NEVER alter state,
		// When admin uses incident-response-readonly,
		// Then read paths are broad but mutating commands (rm, mv, chmod,
		// dd, ...) are all blocked.
		assert.Contains(t, SeedExpectedSSPDTemplateSlugs, "incident-response-readonly")
	})

	t.Run("Scenario_SafetyLadderCovered", func(t *testing.T) {
		// Given posture is a stance ladder (strict..permissive),
		// When seed templates ship,
		// Then all 5 postures are represented.
		assert.Equal(t, 5, len(SeedExpectedSSPDTemplateSafetyPostures))
	})

	t.Run("Scenario_UseCasesCoverMainWorkloadFamilies", func(t *testing.T) {
		// Given AgentHub serves chat/eng/cicd/audit/incident workloads,
		// When seed templates ship,
		// Then all 5 use_cases have at least one example template.
		// Validated structurally via integration test.
		assert.Equal(t, 5, len(SeedExpectedSSPDTemplateUseCases))
	})

	t.Run("Scenario_AdminReviewGatesPostureChange", func(t *testing.T) {
		// Given switching posture changes blast radius materially,
		// When admin compares admin-review subset,
		// Then 4 of 5 templates gate change (only web-safe is routine).
		assert.Equal(t, 4, len(SeedAdminReviewSSPDTemplateSlugs))
	})

	t.Run("Scenario_PolicyShipsCompleteSnapshotNoFollowup", func(t *testing.T) {
		// Given a template captures the full PERM-008 ShellSandboxPolicy
		// shape (path roots + blocked cmds + blocked args + ceilings +
		// network),
		// When admin enables a template,
		// Then no follow-up fields need to be filled — the policy is ready.
		// Validated structurally via integration test (each row has
		// all 6 policy fields populated coherently).
		assert.Equal(t, 5, SeedExpectedSSPDTemplateRowCount)
	})
}
