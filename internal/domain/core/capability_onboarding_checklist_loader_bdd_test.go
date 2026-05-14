package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000106 capability onboarding checklist seeds.
// These assert seed shape and onboarding rationale without a database.

func TestBDD_CapabilityOnboardingChecklistSeed(t *testing.T) {
	t.Run("Scenario_FiveOnboardingStepsGuideNewTenantToFirstRun", func(t *testing.T) {
		// Given a new tenant has provisioned their workspace but has not yet
		//   configured any agents or LLM providers,
		// When migration 000106 seeds onboarding checklist rows,
		// Then exactly 5 steps are added — one per guided milestone — and the
		//   slug list length matches the total count constant.
		assert.Equal(t, 5, SeedCapabilityOnboardingChecklistCount,
			"migration 000106 must seed exactly 5 onboarding checklist rows")
		assert.Len(t, SeedCapabilityOnboardingChecklistSlugs, 5,
			"SeedCapabilityOnboardingChecklistSlugs must list exactly 5 step slugs")
		assert.Equal(t, SeedCapabilityOnboardingChecklistCount, len(SeedCapabilityOnboardingChecklistSlugs),
			"count constant must match slug list length — they describe the same set")
	})

	t.Run("Scenario_FirstThreeStepsAreBlockingForMinimalViableSetup", func(t *testing.T) {
		// Given a new tenant must connect an LLM provider, create an agent, and
		//   assign a skill before any chat session can succeed,
		// When migration 000106 seeds is_blocking flags,
		// Then exactly 3 steps are blocking and 2 are not, and their counts sum
		//   to the total checklist row count.
		assert.Equal(t, 3, SeedOnboardingBlockingStepCount,
			"SeedOnboardingBlockingStepCount must be 3 — connect-llm, create-agent, assign-skill are mandatory")
		assert.Equal(t, 2, SeedOnboardingNonBlockingStepCount,
			"SeedOnboardingNonBlockingStepCount must be 2 — configure-kb and test-run are optional")
		sum := SeedOnboardingBlockingStepCount + SeedOnboardingNonBlockingStepCount
		assert.Equal(t, SeedCapabilityOnboardingChecklistCount, sum,
			"blocking + non-blocking step counts must sum to total checklist count")
	})

	t.Run("Scenario_KBConfigurationIsOptionalNonBlocking", func(t *testing.T) {
		// Given a knowledge base is a powerful feature but not required for a
		//   basic agent to function (agents can operate purely via LLM reasoning),
		// When migration 000106 seeds step 4 (configure-kb),
		// Then the configure-kb slug is present but does NOT appear among the
		//   first SeedOnboardingBlockingStepCount entries (the blocking steps),
		//   confirming it is optional.
		assert.Equal(t, "onboarding-configure-kb", SeedOnboardingConfigureKBSlug,
			"SeedOnboardingConfigureKBSlug must equal \"onboarding-configure-kb\"")
		blockingSlugs := SeedCapabilityOnboardingChecklistSlugs[:SeedOnboardingBlockingStepCount]
		for _, s := range blockingSlugs {
			assert.NotEqual(t, SeedOnboardingConfigureKBSlug, s,
				"configure-kb must not appear among the 3 blocking onboarding steps")
		}
	})

	t.Run("Scenario_TestRunIsLastStepVerifyingEndToEndFunction", func(t *testing.T) {
		// Given a successful chat session is the ultimate proof that onboarding
		//   worked — LLM connected, agent created, skill bound, and the full
		//   pipeline fires without error,
		// When migration 000106 seeds step 5 (test-run),
		// Then the test-run slug is last in the ordered slug list and does NOT
		//   appear among the blocking steps, confirming it is the final optional
		//   validation step.
		assert.Equal(t, "onboarding-test-run", SeedOnboardingTestRunSlug,
			"SeedOnboardingTestRunSlug must equal \"onboarding-test-run\"")
		last := SeedCapabilityOnboardingChecklistSlugs[len(SeedCapabilityOnboardingChecklistSlugs)-1]
		assert.Equal(t, SeedOnboardingTestRunSlug, last,
			"test-run must be the last slug in SeedCapabilityOnboardingChecklistSlugs (step 5)")
		blockingSlugs := SeedCapabilityOnboardingChecklistSlugs[:SeedOnboardingBlockingStepCount]
		for _, s := range blockingSlugs {
			assert.NotEqual(t, SeedOnboardingTestRunSlug, s,
				"test-run must not appear among the 3 blocking onboarding steps")
		}
	})

	t.Run("Scenario_OnboardingResourceTypesMapToAgentHubEntities", func(t *testing.T) {
		// Given each onboarding step targets a distinct AgentHub entity that
		//   the tenant must create or configure (settings, agent, agent_skill,
		//   knowledge_base, chat_session),
		// When migration 000106 seeds resource_type values,
		// Then SeedOnboardingResourceTypes has exactly 5 distinct non-empty
		//   entries — one per step — and the list length matches the checklist
		//   count constant.
		assert.Len(t, SeedOnboardingResourceTypes, 5,
			"SeedOnboardingResourceTypes must have exactly 5 entries — one per onboarding step")
		unique := map[string]struct{}{}
		for _, rt := range SeedOnboardingResourceTypes {
			assert.NotEmpty(t, rt, "each resource_type must be a non-empty string")
			unique[rt] = struct{}{}
		}
		assert.Len(t, unique, 5,
			"all 5 resource types must be distinct — each step targets a different AgentHub entity")
		assert.Equal(t, SeedCapabilityOnboardingChecklistCount, len(SeedOnboardingResourceTypes),
			"SeedOnboardingResourceTypes length must match SeedCapabilityOnboardingChecklistCount")
	})
}
