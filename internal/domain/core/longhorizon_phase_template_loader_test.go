package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreLongHorizonPhaseTemplate_TaskTemplatesCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedExpectedLongHorizonTaskTemplateSlugs))
}

func TestCoreLongHorizonPhaseTemplate_PhasesPerTemplateMatchExpected(t *testing.T) {
	expected := map[string]int{
		"customer-30day-monitoring":  3,
		"quarterly-product-research": 3,
		"compliance-annual-recert":   2,
	}
	for tmpl, count := range expected {
		assert.Equal(t, count, len(SeedExpectedLongHorizonPhaseSlugs[tmpl]),
			"task template %q must have %d phases", tmpl, count)
	}
}

func TestCoreLongHorizonPhaseTemplate_TotalRowCountSumsToEight(t *testing.T) {
	total := 0
	for _, phases := range SeedExpectedLongHorizonPhaseSlugs {
		total += len(phases)
	}
	assert.Equal(t, 8, total)
	assert.Equal(t, total, SeedExpectedLongHorizonPhaseRowCount)
}

func TestCoreLongHorizonPhaseTemplate_TaskTemplateSlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedLongHorizonTaskTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreLongHorizonPhaseTemplate_PhaseSlugsAreKebabCase(t *testing.T) {
	for _, phases := range SeedExpectedLongHorizonPhaseSlugs {
		for _, p := range phases {
			assert.Equal(t, strings.ToLower(p), p)
			assert.NotContains(t, p, "_")
		}
	}
}

func TestCoreLongHorizonPhaseTemplate_PhaseSlugsAreUniqueWithinTemplate(t *testing.T) {
	for tmpl, phases := range SeedExpectedLongHorizonPhaseSlugs {
		seen := map[string]bool{}
		for _, p := range phases {
			assert.False(t, seen[p], "phase %q duplicated in template %q", p, tmpl)
			seen[p] = true
		}
	}
}

func TestCoreLongHorizonPhaseTemplate_AdminReviewSubsetIsTerminalPhases(t *testing.T) {
	// Admin review attaches to the FINAL phase of each template
	// (sign-off / publish / attestation) — never intermediate ones.
	expectedFinals := []string{
		"customer-30day-monitoring/final-report",
		"quarterly-product-research/final-review-and-publish",
		"compliance-annual-recert/admin-attestation",
	}
	assert.ElementsMatch(t, expectedFinals, SeedAdminReviewLongHorizonPhases)
	assert.Equal(t, 3, len(SeedAdminReviewLongHorizonPhases))
}

func TestCoreLongHorizonPhaseTemplate_DependsOnListParser(t *testing.T) {
	tmpl := CoreLongHorizonPhaseTemplate{DependsOn: "phase-a, phase-b"}
	assert.Equal(t, []string{"phase-a", "phase-b"}, tmpl.DependsOnList())

	empty := CoreLongHorizonPhaseTemplate{DependsOn: "  "}
	assert.Nil(t, empty.DependsOnList())

	none := CoreLongHorizonPhaseTemplate{DependsOn: ""}
	assert.Nil(t, none.DependsOnList())
}
