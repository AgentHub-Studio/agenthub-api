package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreBgJobTriggerTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedBgJobTriggerTemplateSlugs))
	assert.Equal(t, 8, SeedExpectedBgJobTriggerTemplateRowCount)
}

func TestCoreBgJobTriggerTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedBgJobTriggerTemplateSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreBgJobTriggerTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedBgJobTriggerTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_", "slug %q must use kebab-case", s)
	}
}

func TestCoreBgJobTriggerTemplate_KindsMatchFutureThreeEnum(t *testing.T) {
	expected := map[string]bool{
		"schedule_cron": true, "event_arrived": true,
		"threshold_crossed": true, "absence_timeout": true,
	}
	for _, k := range SeedExpectedBgJobTriggerTemplateKinds {
		assert.True(t, expected[k], "kind %q outside FUTURE-003 enum", k)
	}
	assert.Equal(t, len(expected), len(SeedExpectedBgJobTriggerTemplateKinds))
}

func TestCoreBgJobTriggerTemplate_OnFailureActionsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"retry_with_backoff": true,
		"alert_admin":        true,
		"quarantine_trigger": true,
	}
	for _, a := range SeedExpectedBgJobTriggerTemplateOnFailureActions {
		assert.True(t, expected[a])
	}
}

func TestCoreBgJobTriggerTemplate_RecommendedAndCanonicalAreEqual(t *testing.T) {
	// Each pairs with a documented FUTURE-003 trigger pattern; tenants
	// opt in per template, not per category.
	assert.ElementsMatch(t,
		SeedExpectedBgJobTriggerTemplateSlugs,
		SeedRecommendedBgJobTriggerTemplateSlugs,
		"all bg-job templates must be recommended (each implements a FUTURE-003 pattern)")
}

func TestCoreBgJobTriggerTemplate_AdminReviewIsBusinessImpactSubset(t *testing.T) {
	canonical := map[string]bool{}
	for _, s := range SeedExpectedBgJobTriggerTemplateSlugs {
		canonical[s] = true
	}
	for _, a := range SeedAdminReviewBgJobTriggerTemplateSlugs {
		assert.True(t, canonical[a], "admin-review %q must be canonical seed", a)
	}
	// cost-pause has business impact; agent-archival risks losing tenant
	// work; routine cron/event/inactivity templates do not require review.
	assert.Equal(t, 2, len(SeedAdminReviewBgJobTriggerTemplateSlugs))
}

func TestCoreBgJobTriggerTemplate_TriggerConfigParser_HandlesAllKinds(t *testing.T) {
	cases := []CoreBackgroundJobTriggerTemplate{
		{Slug: "x", TriggerConfigJSON: `{"cron":"0 8 * * *"}`},
		{Slug: "x", TriggerConfigJSON: `{"event_name":"kb.uploaded"}`},
		{Slug: "x", TriggerConfigJSON: `{"metric":"x","threshold":1}`},
		{Slug: "x", TriggerConfigJSON: `{"absence_seconds":2592000}`},
	}
	for _, c := range cases {
		got, err := c.TriggerConfig()
		require.NoError(t, err)
		assert.NotEmpty(t, got)
	}
}

func TestCoreBgJobTriggerTemplate_TriggerConfigParser_RejectsMalformed(t *testing.T) {
	c := CoreBackgroundJobTriggerTemplate{Slug: "x", TriggerConfigJSON: `not-json`}
	_, err := c.TriggerConfig()
	assert.Error(t, err)
}
