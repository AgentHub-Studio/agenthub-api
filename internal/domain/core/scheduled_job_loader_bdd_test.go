package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSchedJobSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsScheduledJobLibrary", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(SeedExpectedScheduledJobTemplateSlugs), 6)
	})

	t.Run("Scenario_FourJobKindsCoverCommonRecurringPatterns", func(t *testing.T) {
		set := map[string]bool{}
		for _, k := range SeedExpectedScheduledJobTemplateKinds {
			set[k] = true
		}
		for _, want := range []string{"reporting", "monitoring", "compliance", "maintenance"} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_DailyMorningBriefingIsRecommendedSafeDefault", func(t *testing.T) {
		// Given the most common recurring need is "tell me what happened",
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedScheduledJobTemplateSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["daily-morning-briefing"])
	})

	t.Run("Scenario_ComplianceJobsRequireAdminApprovalForOngoingCost", func(t *testing.T) {
		// Given enabling weekly governance / monthly audit creates
		//       ongoing cost commitment + compliance obligations,
		set := map[string]bool{}
		for _, s := range SeedAdminApprovalScheduledJobTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["weekly-governance-export"])
		assert.True(t, set["monthly-decision-audit"])
	})

	t.Run("Scenario_RecommendedExcludesAdminApprovalJobs", func(t *testing.T) {
		// Given recommended = one-click safe enable,
		adminSet := map[string]bool{}
		for _, a := range SeedAdminApprovalScheduledJobTemplateSlugs {
			adminSet[a] = true
		}
		for _, r := range SeedRecommendedScheduledJobTemplateSlugs {
			assert.False(t, adminSet[r],
				"recommended %q must not require admin (one-click constraint)", r)
		}
	})

	t.Run("Scenario_JobNamesEncodeFrequencyForReadability", func(t *testing.T) {
		// Given operators see "daily-X" / "hourly-X" / "weekly-X" /
		//       "monthly-X" / "every-15min-X" — frequency in slug
		//       saves a column in dashboards,
		validPrefixes := []string{
			"daily-", "hourly-", "weekly-", "nightly-", "monthly-", "every-",
		}
		for _, s := range SeedExpectedScheduledJobTemplateSlugs {
			matches := false
			for _, p := range validPrefixes {
				if strings.HasPrefix(s, p) {
					matches = true
					break
				}
			}
			assert.True(t, matches,
				"slug %q must encode frequency (daily/hourly/weekly/nightly/monthly/every-)", s)
		}
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 8, len(SeedExpectedScheduledJobTemplateSlugs))
	})

	t.Run("Scenario_StalledRunSweepIsHighFrequencyMaintenance", func(t *testing.T) {
		// Given stalled-run cleanup must happen often (15min) to avoid
		//       resource leaks,
		set := map[string]bool{}
		for _, s := range SeedExpectedScheduledJobTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["every-15min-stalled-run-sweep"])
	})
}
