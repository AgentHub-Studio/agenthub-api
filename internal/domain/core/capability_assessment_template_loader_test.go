package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreCapabilityTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedCapabilityTemplateSlugs))
	assert.Equal(t, 8, SeedExpectedCapabilityTemplateRowCount)
	assert.Equal(t,
		len(SeedExpectedCapabilityTemplateSlugs),
		SeedExpectedCapabilityTemplateRowCount,
		"row count must match canonical slug list length")
}

func TestCoreCapabilityTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCapabilityTemplateSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreCapabilityTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedCapabilityTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s, "slug %q must be lowercase", s)
		assert.NotContains(t, s, "_", "slug %q must use kebab-case (no underscores)", s)
		assert.NotContains(t, s, " ", "slug %q must not contain spaces", s)
	}
}

func TestCoreCapabilityTemplate_DimensionsMatchFutureSixEnumPlusMulti(t *testing.T) {
	// Five FUTURE-006 dimensions + "multi" sentinel = 6 values.
	expected := map[string]bool{
		"domain_knowledge":      true,
		"decision_independence": true,
		"task_throughput":       true,
		"quality_output":        true,
		"collaboration":         true,
		"multi":                 true,
	}
	for _, d := range SeedExpectedCapabilityTemplateDimensions {
		assert.True(t, expected[d], "dimension %q outside expected set", d)
	}
	assert.Equal(t, len(expected), len(SeedExpectedCapabilityTemplateDimensions))
}

func TestCoreCapabilityTemplate_CadencesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"weekly": true, "monthly": true, "quarterly": true,
		"one_shot": true, "event_driven": true,
	}
	for _, c := range SeedExpectedCapabilityTemplateCadences {
		assert.True(t, expected[c], "cadence %q outside expected set", c)
	}
	assert.Equal(t, len(expected), len(SeedExpectedCapabilityTemplateCadences))
}

func TestCoreCapabilityTemplate_EvaluatorKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"rubric_scored":    true,
		"behavior_log":     true,
		"metric_aggregate": true,
		"peer_review":      true,
		"incident_review":  true,
	}
	for _, k := range SeedExpectedCapabilityTemplateEvaluatorKinds {
		assert.True(t, expected[k], "evaluator_kind %q outside expected set", k)
	}
}

func TestCoreCapabilityTemplate_RecommendedSubsetIsSensible(t *testing.T) {
	canonical := map[string]bool{}
	for _, s := range SeedExpectedCapabilityTemplateSlugs {
		canonical[s] = true
	}
	for _, r := range SeedRecommendedCapabilityTemplateSlugs {
		assert.True(t, canonical[r], "recommended %q must be canonical seed", r)
	}
	// Recommended must be a strict subset (incident-postmortem +
	// collaboration-quarterly are deliberately NOT recommended one-click).
	assert.Less(t, len(SeedRecommendedCapabilityTemplateSlugs),
		len(SeedExpectedCapabilityTemplateSlugs),
		"recommended must be strict subset")
}

func TestCoreCapabilityTemplate_AdminReviewSubsetIsSensible(t *testing.T) {
	canonical := map[string]bool{}
	for _, s := range SeedExpectedCapabilityTemplateSlugs {
		canonical[s] = true
	}
	for _, a := range SeedAdminReviewCapabilityTemplateSlugs {
		assert.True(t, canonical[a], "admin-review %q must be canonical seed", a)
	}
	// Holistic + collaboration + incident require admin vetting; per-
	// dimension monthly/weekly are routine and don't.
	assert.Equal(t, 3, len(SeedAdminReviewCapabilityTemplateSlugs))
}

func TestCoreCapabilityTemplate_HolisticIsBothRecommendedAndAdminReviewed(t *testing.T) {
	rec, admin := false, false
	for _, s := range SeedRecommendedCapabilityTemplateSlugs {
		if s == "holistic-capability-quarterly" {
			rec = true
		}
	}
	for _, s := range SeedAdminReviewCapabilityTemplateSlugs {
		if s == "holistic-capability-quarterly" {
			admin = true
		}
	}
	assert.True(t, rec, "holistic must be recommended (default for orgs with periodic reviews)")
	assert.True(t, admin, "holistic must require admin review (used for promotion decisions)")
}

func TestCoreCapabilityTemplate_OnboardingIsRecommendedButNotAdminReviewed(t *testing.T) {
	rec, admin := false, false
	for _, s := range SeedRecommendedCapabilityTemplateSlugs {
		if s == "onboarding-baseline" {
			rec = true
		}
	}
	for _, s := range SeedAdminReviewCapabilityTemplateSlugs {
		if s == "onboarding-baseline" {
			admin = true
		}
	}
	assert.True(t, rec, "onboarding baseline must be recommended (every fresh user)")
	assert.False(t, admin, "onboarding is one-shot baseline, not promotion-grade")
}
