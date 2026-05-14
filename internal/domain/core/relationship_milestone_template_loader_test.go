package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreRelMilestoneTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedRelMilestoneTemplateSlugs))
	assert.Equal(t, 8, SeedExpectedRelMilestoneTemplateRowCount)
}

func TestCoreRelMilestoneTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedRelMilestoneTemplateSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreRelMilestoneTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedRelMilestoneTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_", "slug %q must use kebab-case", s)
		assert.NotContains(t, s, " ", "slug %q must not contain spaces", s)
	}
}

func TestCoreRelMilestoneTemplate_TrustLevelsMatchFutureTwoEnumMinusUnknown(t *testing.T) {
	expected := map[string]bool{
		"probationary": true, "established": true,
		"trusted": true, "mistrusted": true,
	}
	for _, l := range SeedExpectedRelMilestoneTemplateTrustLevels {
		assert.True(t, expected[l], "level %q outside expected set", l)
	}
	assert.Equal(t, len(expected), len(SeedExpectedRelMilestoneTemplateTrustLevels))
	// "unknown" is the FUTURE-002 default — never a target.
	for _, l := range SeedExpectedRelMilestoneTemplateTrustLevels {
		assert.NotEqual(t, "unknown", l, "unknown must never be a milestone target")
	}
}

func TestCoreRelMilestoneTemplate_EventKindsMatchFutureTwoEnum(t *testing.T) {
	expected := map[string]bool{
		"positive": true, "neutral": true, "negative": true,
		"conflict_resolved": true, "escalation": true,
	}
	for _, k := range SeedExpectedRelMilestoneTemplateEventKinds {
		assert.True(t, expected[k], "event_kind %q outside expected set", k)
	}
	assert.Equal(t, len(expected), len(SeedExpectedRelMilestoneTemplateEventKinds))
}

func TestCoreRelMilestoneTemplate_ActionsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"no_action":                   true,
		"elevate_communication_style": true,
		"send_admin_notification":     true,
		"open_review_ticket":          true,
	}
	for _, a := range SeedExpectedRelMilestoneTemplateActions {
		assert.True(t, expected[a], "action %q outside expected set", a)
	}
}

func TestCoreRelMilestoneTemplate_RecommendedAndCanonicalAreEqual(t *testing.T) {
	// Every milestone implements documented FUTURE-002 promotion rules,
	// so all are safe one-click defaults.
	assert.ElementsMatch(t,
		SeedExpectedRelMilestoneTemplateSlugs,
		SeedRecommendedRelMilestoneTemplateSlugs,
		"all milestones must be recommended (each implements a documented rule)")
}

func TestCoreRelMilestoneTemplate_AdminReviewSubsetIsSensible(t *testing.T) {
	canonical := map[string]bool{}
	for _, s := range SeedExpectedRelMilestoneTemplateSlugs {
		canonical[s] = true
	}
	for _, a := range SeedAdminReviewRelMilestoneTemplateSlugs {
		assert.True(t, canonical[a], "admin-review %q must be canonical seed", a)
	}
	// Trusted promotion + mistrusted detection + escalation ticket
	// require admin vetting; routine recordings don't.
	assert.Equal(t, 3, len(SeedAdminReviewRelMilestoneTemplateSlugs))
}

func TestCoreRelMilestoneTemplate_TriggersEventKindsListParser(t *testing.T) {
	tmpl := CoreRelationshipMilestoneTemplate{
		TriggersEventKinds: "positive, neutral, conflict_resolved",
	}
	got := tmpl.TriggersEventKindsList()
	assert.Equal(t, []string{"positive", "neutral", "conflict_resolved"}, got)

	empty := CoreRelationshipMilestoneTemplate{TriggersEventKinds: "  "}
	assert.Nil(t, empty.TriggersEventKindsList())
}
