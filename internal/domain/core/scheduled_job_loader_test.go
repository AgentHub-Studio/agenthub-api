package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSchedJob_SlugsNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedScheduledJobTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreSchedJob_SlugsCanonicalCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedScheduledJobTemplateSlugs))
}

func TestCoreSchedJob_KindsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedScheduledJobTemplateKinds))
}

func TestCoreSchedJob_KindsCoverCommonScheduledTaskCategories(t *testing.T) {
	allowed := map[string]bool{}
	for _, k := range SeedExpectedScheduledJobTemplateKinds {
		allowed[k] = true
	}
	for _, want := range []string{"reporting", "monitoring", "compliance", "maintenance"} {
		assert.True(t, allowed[want])
	}
}

func TestCoreSchedJob_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedScheduledJobTemplateSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedScheduledJobTemplateSlugs {
		assert.True(t, seedSet[r])
	}
}

func TestCoreSchedJob_AdminApprovalSlugsAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedScheduledJobTemplateSlugs {
		seedSet[s] = true
	}
	for _, a := range SeedAdminApprovalScheduledJobTemplateSlugs {
		assert.True(t, seedSet[a])
	}
}

func TestCoreSchedJob_AdminApprovalIsComplianceOnly(t *testing.T) {
	// Only compliance jobs require admin approval (audit + ongoing-cost contract).
	assert.Equal(t, 2, len(SeedAdminApprovalScheduledJobTemplateSlugs))
	for _, s := range SeedAdminApprovalScheduledJobTemplateSlugs {
		// All compliance prefix names
		isCompliance := strings.Contains(s, "governance") || strings.Contains(s, "audit")
		assert.True(t, isCompliance,
			"admin-approval slug %q must be compliance-related", s)
	}
}

func TestCoreSchedJob_RecommendedExcludesAdminApprovalJobs(t *testing.T) {
	// Recommended = one-click; admin-approval = explicit opt-in.
	adminSet := map[string]bool{}
	for _, a := range SeedAdminApprovalScheduledJobTemplateSlugs {
		adminSet[a] = true
	}
	for _, r := range SeedRecommendedScheduledJobTemplateSlugs {
		assert.False(t, adminSet[r],
			"recommended %q must not require admin approval", r)
	}
}

func TestCoreSchedJob_SlugsKebabCase(t *testing.T) {
	for _, s := range SeedExpectedScheduledJobTemplateSlugs {
		assert.False(t, strings.Contains(s, "_"))
	}
}
