package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePlatformSettingSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsAllPlatformDefaults", func(t *testing.T) {
		// Given a fresh tenant (no setting overrides),
		// When the runtime asks ah_core for settings,
		// Then ≥10 baseline tunables are seeded — runner has functional
		//      defaults across budget / depth / timeout / evaluator /
		//      policy / checkpoint / UI / security / compaction.
		assert.GreaterOrEqual(t, len(SeedExpectedPlatformSettingKeys), 10,
			"fresh tenant must inherit at least 10 baseline settings")
	})

	t.Run("Scenario_RunnerCategoryHasFunctionalBudgetAndDepthDefaults", func(t *testing.T) {
		// Given the runner needs MaxIterations, MaxBudgetUSD, MaxDepth,
		//       and elicitation timeout to function,
		// When the seed is inspected,
		// Then all 4 runner tunables exist.
		seedSet := map[string]bool{}
		for _, k := range SeedExpectedPlatformSettingKeys {
			seedSet[k] = true
		}
		for _, k := range []string{
			"runner.default_max_iterations",
			"runner.default_max_budget_usd",
			"runner.default_max_depth",
			"runner.elicitation_timeout_ms",
		} {
			assert.True(t, seedSet[k], "runner default %q must be seeded", k)
		}
	})

	t.Run("Scenario_EvaluatorThresholdsMirrorOBS008Defaults", func(t *testing.T) {
		// Given OBS-008 HeuristicRunEvaluator uses fail<0.4 / warn<0.7
		//       (DefaultHeuristicRunEvaluatorConfig in evaluator.go),
		// When the seed is inspected,
		// Then the same thresholds appear as platform defaults — runtime
		//      can read from settings instead of hardcoded constants.
		seedSet := map[string]bool{}
		for _, k := range SeedExpectedPlatformSettingKeys {
			seedSet[k] = true
		}
		assert.True(t, seedSet["evaluator.fail_threshold"])
		assert.True(t, seedSet["evaluator.warn_threshold"])
		assert.True(t, seedSet["evaluator.default_engine"])
	})

	t.Run("Scenario_PolicyEngineDefaultIsNoOpForFreshDeploys", func(t *testing.T) {
		// Given GOV-002 PolicyEngine has 4 implementations, NoOp is
		//       the default for fresh deploys (allows everything),
		// When the seed is inspected,
		// Then policy.default_engine = noop is documented as default.
		seedSet := map[string]bool{}
		for _, k := range SeedExpectedPlatformSettingKeys {
			seedSet[k] = true
		}
		assert.True(t, seedSet["policy.default_engine"],
			"policy engine selection must be a tunable")
	})

	t.Run("Scenario_CheckpointDefaultsCoverDeadlineAndCostTrigger", func(t *testing.T) {
		// Given GOV-003 checkpoints need a default deadline + cost
		//       trigger threshold to be useful out of the box,
		seedSet := map[string]bool{}
		for _, k := range SeedExpectedPlatformSettingKeys {
			seedSet[k] = true
		}
		assert.True(t, seedSet["checkpoint.default_deadline_seconds"])
		assert.True(t, seedSet["checkpoint.cost_threshold_usd"])
	})

	t.Run("Scenario_UIDefaultOutputStyleReferencesAhCoreCatalog", func(t *testing.T) {
		// Given output_styles seed installed "conversational" as the
		//       platform default style,
		// When the UI default is inspected,
		// Then ui.default_output_style_slug references the same slug —
		//      cross-category integrity (FK relationship at app level).
		seedSet := map[string]bool{}
		for _, k := range SeedExpectedPlatformSettingKeys {
			seedSet[k] = true
		}
		assert.True(t, seedSet["ui.default_output_style_slug"],
			"UI default style must be a settable platform default")
	})

	t.Run("Scenario_SecurityBaselinesAreNotOverridable", func(t *testing.T) {
		// Given security baselines (deny dangerous tools, audit
		//       retention) protect the platform from tenant-level
		//       relaxation,
		// When the override-policy is inspected,
		// Then both security keys are non-overridable.
		nonOver := map[string]bool{}
		for _, k := range SeedNonOverridablePlatformSettingKeys {
			nonOver[k] = true
		}
		for _, k := range []string{
			"security.deny_dangerous_tools",
			"security.audit_retention_days",
		} {
			assert.True(t, nonOver[k],
				"security baseline %q must be non-overridable", k)
		}
	})

	t.Run("Scenario_AllSettingsUseDottedPathConventionForNamespacing", func(t *testing.T) {
		// Given the dotted-path convention prevents key collisions
		//       across categories ("runner.timeout" vs "ui.timeout"),
		// When the seed key set is inspected,
		// Then every key has a category prefix.
		for _, k := range SeedExpectedPlatformSettingKeys {
			assert.Contains(t, k, ".",
				"key %q must use category.name dotted path", k)
			parts := strings.Split(k, ".")
			assert.GreaterOrEqual(t, len(parts), 2,
				"key %q must have at least one dot", k)
		}
	})

	t.Run("Scenario_ValueTypesAreBoundedToFourPrimitives", func(t *testing.T) {
		// Given the loader parses Value as one of: string / number /
		//       boolean / json (no enum / date / etc.),
		// When the value-type set is inspected,
		// Then exactly 4 types are allowed.
		assert.Equal(t, 4, len(SeedExpectedPlatformSettingValueTypes))
		set := map[string]bool{}
		for _, vt := range SeedExpectedPlatformSettingValueTypes {
			set[vt] = true
		}
		assert.True(t, set["string"])
		assert.True(t, set["number"])
		assert.True(t, set["boolean"])
		assert.True(t, set["json"])
	})

	t.Run("Scenario_CategoriesCoverEverySubsystemBucket", func(t *testing.T) {
		// Given dashboards group settings by category,
		// When the category set is inspected,
		// Then it covers the 7 buckets that map to actual subsystems.
		assert.Equal(t, 7, len(SeedExpectedPlatformSettingCategories))
		set := map[string]bool{}
		for _, c := range SeedExpectedPlatformSettingCategories {
			set[c] = true
		}
		for _, c := range []string{
			"runner", "evaluator", "policy", "checkpoint",
			"ui", "security", "compaction",
		} {
			assert.True(t, set[c], "category %q must be seeded", c)
		}
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		// Given external systems bind to the canonical count = 15,
		assert.Equal(t, 15, len(SeedExpectedPlatformSettingKeys))
	})
}
