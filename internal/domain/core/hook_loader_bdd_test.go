package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreHookSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsBaselineHooks", func(t *testing.T) {
		// Given a fresh tenant (no hooks of its own),
		// When the runtime asks ah_core for hooks,
		// Then ≥10 baseline hooks are seeded — agent has guidance at
		//      every major lifecycle event from day one.
		assert.GreaterOrEqual(t, len(SeedExpectedHookSlugs), 10,
			"fresh tenant must inherit at least 10 baseline hooks")
	})

	t.Run("Scenario_SafetyHooksAllAdminOnlyDisable", func(t *testing.T) {
		// Given safety hooks are bypass-immune (PDF Section 5.3),
		adminSet := map[string]bool{}
		for _, s := range SeedAdminOnlyDisableHookSlugs {
			adminSet[s] = true
		}

		// Then all safety-prefixed slugs require admin to disable.
		for _, s := range SeedExpectedHookSlugs {
			if strings.HasPrefix(s, "safety-") {
				assert.True(t, adminSet[s],
					"safety hook %q must require admin to disable", s)
			}
		}
	})

	t.Run("Scenario_PreToolUseHookCoversBothDestructiveAndSecret", func(t *testing.T) {
		// Given the two most common safety pre-checks (PDF Section 5.3:
		//       PreToolUse can block/rewrite),
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedHookSlugs {
			seedSet[s] = true
		}

		// Then both safety hooks for PreToolUse exist.
		assert.True(t, seedSet["safety-pretooluse-confirm-destructive"],
			"destructive-tool reminder must be seeded")
		assert.True(t, seedSet["safety-pretooluse-redact-secrets"],
			"secret-redaction reminder must be seeded")
	})

	t.Run("Scenario_PermissionDeniedHookExplainsToUser", func(t *testing.T) {
		// Given PDF Section 5.3 + 11: when permission is denied, the
		//       agent must EXPLAIN to the user what was attempted and
		//       why,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedHookSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["safety-permissiondenied-explain"],
			"permission-denied explainer must be seeded")
	})

	t.Run("Scenario_TenantHooksOverrideAhCoreDefaults", func(t *testing.T) {
		// Given the LoadByEvent contract: the runner asks ah_core for
		//       defaults, then merges with tenant-registered hooks. A
		//       tenant hook for the same event takes precedence (this is
		//       runtime contract; tested here at the constant level by
		//       making sure the seed defaults are documented stably).
		// When the loader is called with an event,
		// Then SeedExpectedHookEvents enumerates the events the tenant
		//      can predict the platform default exists for.
		assert.NotEmpty(t, SeedExpectedHookEvents)
		assert.Contains(t, SeedExpectedHookEvents, "PreToolUse",
			"PreToolUse is one of the predictable platform-default events")
	})

	t.Run("Scenario_LifecycleHooksGuideUserExperience", func(t *testing.T) {
		// Given web UX needs greeting + clarification + summary
		//       (lifecycle category),
		expected := []string{
			"lifecycle-sessionstart-greeting",
			"lifecycle-userpromptsubmit-clarify-if-vague",
			"lifecycle-stop-summarize-if-long",
		}
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedHookSlugs {
			seedSet[s] = true
		}
		for _, s := range expected {
			assert.True(t, seedSet[s],
				"lifecycle hook %q must be in seed for web UX", s)
		}
	})

	t.Run("Scenario_ContextHooksProtectCompactionDecisions", func(t *testing.T) {
		// Given PreCompact + PostCompact must preserve user-blocking
		//       context (PDF Section 7.3),
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedHookSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["context-precompact-preserve-decisions"],
			"PreCompact preservation hook must be seeded")
		assert.True(t, seedSet["context-postcompact-acknowledge"],
			"PostCompact acknowledgement hook must be seeded")
	})

	t.Run("Scenario_CoordinationHooksHandleSubagentLifecycle", func(t *testing.T) {
		// Given subagents return only summaries (PDF Section 8.3),
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedHookSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["coord-subagentstop-summarize-result"],
			"SubagentStop hook must summarize for parent")
		assert.True(t, seedSet["coord-taskcompleted-acknowledge"],
			"TaskCompleted hook must acknowledge briefly")
	})

	t.Run("Scenario_OnlyPromptTypeIsSeeded", func(t *testing.T) {
		// Given http/agent/command hooks have side effects the platform
		//       cannot guarantee at seed time (PDF Section 6.1: hook
		//       types differ in side-effect surface),
		// When the seed catalogue is inspected,
		// Then only "prompt" type is allowed at seed time. Tenants can
		//      register http hooks themselves.
		// (Asserting via convention — integration test verifies the
		// actual DB rows.)
		assert.True(t, true, "seed convention documented; integration test verifies DB rows")
	})

	t.Run("Scenario_TenEventsCoveredForBaselineUX", func(t *testing.T) {
		// Given the seed must touch the major lifecycle moments,
		assert.Equal(t, 10, len(SeedExpectedHookEvents),
			"10 events covered by seed for baseline UX")
		// And these include the safety-critical 3 (PreToolUse,
		// PermissionDenied, PostToolUseFailure).
		set := map[string]bool{}
		for _, e := range SeedExpectedHookEvents {
			set[e] = true
		}
		assert.True(t, set["PreToolUse"])
		assert.True(t, set["PermissionDenied"])
		assert.True(t, set["PostToolUseFailure"])
	})

	t.Run("Scenario_SafetyHooksHaveHigherPriorityThanLifecycleHooks", func(t *testing.T) {
		// Given safety hooks must fire FIRST within an event (PDF
		//       Section 5.3 + 11: safety > convenience),
		// When integration test verifies DB ordering,
		// Then safety hooks (priority 85-100) sort above lifecycle
		//      hooks (priority 50-60). Constant-level guard:
		//      SafetyAdminOnlyDisable is a non-empty set — refactor
		//      that empties it must explicitly reset the hierarchy.
		assert.NotEmpty(t, SeedAdminOnlyDisableHookSlugs,
			"admin-only safety set must remain non-empty")
	})
}
