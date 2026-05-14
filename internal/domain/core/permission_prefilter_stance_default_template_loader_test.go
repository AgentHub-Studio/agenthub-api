package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePPSDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedPPSDTemplateSlugs))
	assert.Equal(t, 4, SeedExpectedPPSDTemplateRowCount)
}

func TestCorePPSDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPPSDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePPSDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPPSDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePPSDTemplate_StancesMatchPERM004Enum(t *testing.T) {
	expected := map[string]bool{"show_confirm": true, "hide_confirm": true}
	for _, s := range SeedExpectedPPSDTemplateStances {
		assert.True(t, expected[s], "stance %q outside PERM-004 enum", s)
	}
	assert.Equal(t, 2, len(SeedExpectedPPSDTemplateStances))
}

func TestCorePPSDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"interactive_chat": true, "background_job": true,
		"incident_response": true, "compliance_audit": true,
	}
	for _, u := range SeedExpectedPPSDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 4, len(SeedExpectedPPSDTemplateUseCases))
}

func TestCorePPSDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	for _, k := range SeedExpectedPPSDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCorePPSDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPPSDTemplateSlugs,
		SeedRecommendedPPSDTemplateSlugs)
}

func TestCorePPSDTemplate_AdminReviewExcludesInteractive(t *testing.T) {
	// Interactive is the routine default. The other 3 materially change
	// security posture and require admin review.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewPPSDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["interactive-default"])
	assert.True(t, set["unattended-batch"])
	assert.True(t, set["lockdown-readonly"])
	assert.True(t, set["audit-strict-trace"])
}
