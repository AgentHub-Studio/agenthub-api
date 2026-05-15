package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Unit tests (FEAT031) ---

func TestFEAT031_PermissionModeCatalog_SeedCountIsSeven(t *testing.T) {
	assert.Equal(t, 7, SeedPermissionModeCatalogCount,
		"§5.1 names exactly 7 permission modes")
}

func TestFEAT031_PermissionModeCatalog_StructuralInvariant_SevenEntries(t *testing.T) {
	assert.True(t, PermissionModeCatalogHasSevenEntries(),
		"catalog must contain exactly seven §5.1 entries")
}

func TestFEAT031_PermissionModeCatalog_StructuralInvariant_FiveExternal(t *testing.T) {
	assert.True(t, PermissionModeCatalogExternalCountIsFive(),
		"§5.1: exactly five modes are in EXTERNAL_PERMISSION_MODES")
}

func TestFEAT031_PermissionModeCatalog_StructuralInvariant_OneBubble(t *testing.T) {
	assert.True(t, PermissionModeCatalogOneBubbleMode(),
		"§5.1: exactly one internal-only mode (bubble)")
}

func TestFEAT031_PermissionModeCatalog_StructuralInvariant_OneConditional(t *testing.T) {
	assert.True(t, PermissionModeCatalogOneConditionalMode(),
		"§5.1: exactly one conditional mode (auto)")
}

func TestFEAT031_PermissionModeCatalog_StructuralInvariant_FiveGradientModes(t *testing.T) {
	assert.True(t, PermissionModeCatalogGradientCountIsFive(),
		"§11.3 gradient covers exactly five modes")
}

func TestFEAT031_PermissionModeCatalog_StructuralInvariant_AutonomyRanksUnique(t *testing.T) {
	assert.True(t, PermissionModeCatalogAutonomyRanksAreUnique(),
		"each ranked mode must have a distinct autonomy rank")
}

func TestFEAT031_PermissionModeCatalog_StructuralInvariant_OnlyBubbleIsSubagentOnly(t *testing.T) {
	assert.True(t, PermissionModeCatalogOnlySubagentModeIsBubble(),
		"bubble is the only subagent-exclusive mode")
}

func TestFEAT031_PermissionModeCatalog_NewRegistry_NotNil(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	require.NotNil(t, r)
}

func TestFEAT031_PermissionModeCatalog_FindBySlug_Plan(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	entry, ok := r.FindPermissionModeCatalogEntryBySlug("plan")
	require.True(t, ok)
	assert.Equal(t, PermissionModePlan, entry.Mode)
	assert.Equal(t, PermissionModeVisibilityExternal, entry.Visibility)
	assert.Equal(t, 1, entry.AutonomyRank)
	assert.True(t, entry.InGradient)
}

func TestFEAT031_PermissionModeCatalog_FindBySlug_Bubble(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	entry, ok := r.FindPermissionModeCatalogEntryBySlug("bubble")
	require.True(t, ok)
	assert.Equal(t, PermissionModeBubble, entry.Mode)
	assert.Equal(t, PermissionModeVisibilityInternal, entry.Visibility)
	assert.Equal(t, 0, entry.AutonomyRank, "bubble is not ranked in the autonomy spectrum")
	assert.False(t, entry.InGradient, "bubble is not in the §11.3 gradient")
	assert.True(t, entry.SubagentOnly)
}

func TestFEAT031_PermissionModeCatalog_FindBySlug_Auto(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	entry, ok := r.FindPermissionModeCatalogEntryBySlug("auto")
	require.True(t, ok)
	assert.Equal(t, PermissionModeVisibilityConditional, entry.Visibility)
	assert.Equal(t, "TRANSCRIPT_CLASSIFIER", entry.ActivationFlag)
	assert.Equal(t, PermissionModePromptMLClassifier, entry.PromptBehavior)
	assert.True(t, entry.InGradient)
}

func TestFEAT031_PermissionModeCatalog_FindBySlug_DontAsk(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	entry, ok := r.FindPermissionModeCatalogEntryBySlug("dont_ask")
	require.True(t, ok)
	assert.Equal(t, PermissionModeDontAsk, entry.Mode)
	assert.Equal(t, PermissionModeVisibilityExternal, entry.Visibility)
	assert.False(t, entry.InGradient, "dontAsk is external but NOT in the §11.3 gradient")
	assert.Equal(t, PermissionModePromptDenyOnAsk, entry.PromptBehavior)
	assert.True(t, entry.DenyRulesEnforced)
}

func TestFEAT031_PermissionModeCatalog_FindBySlug_Unknown(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	_, ok := r.FindPermissionModeCatalogEntryBySlug("nonexistent")
	assert.False(t, ok)
}

func TestFEAT031_PermissionModeCatalog_AllModes_LengthSeven(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	modes := r.AllModes()
	assert.Len(t, modes, 7)
}

func TestFEAT031_PermissionModeCatalog_ExternalModes_LengthFive(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	ext := r.ExternalModes()
	assert.Len(t, ext, 5)
	for _, e := range ext {
		assert.Equal(t, PermissionModeVisibilityExternal, e.Visibility)
	}
}

func TestFEAT031_PermissionModeCatalog_ConditionalModes_OnlyAuto(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	cond := r.ConditionalModes()
	require.Len(t, cond, 1)
	assert.Equal(t, PermissionModeAuto, cond[0].Mode)
}

func TestFEAT031_PermissionModeCatalog_InternalModes_OnlyBubble(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	internal := r.InternalModes()
	require.Len(t, internal, 1)
	assert.Equal(t, PermissionModeBubble, internal[0].Mode)
}

func TestFEAT031_PermissionModeCatalog_GradientModes_FiveModes(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	grad := r.GradientModes()
	assert.Len(t, grad, 5)
	expectedGradient := []PermissionMode{
		PermissionModePlan,
		PermissionModeDefault,
		PermissionModeAcceptEdits,
		PermissionModeAuto,
		PermissionModeBypassPermissions,
	}
	for i, e := range grad {
		assert.Equal(t, expectedGradient[i], e.Mode,
			"gradient mode at index %d must be %s", i, expectedGradient[i])
	}
}

func TestFEAT031_PermissionModeCatalog_NonGradientModes_DontAskAndBubble(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	nonGrad := r.NonGradientModes()
	require.Len(t, nonGrad, 2)
	modes := []PermissionMode{nonGrad[0].Mode, nonGrad[1].Mode}
	assert.Contains(t, modes, PermissionModeDontAsk)
	assert.Contains(t, modes, PermissionModeBubble)
}

func TestFEAT031_PermissionModeCatalog_MostAutonomousRanked_IsBypassPermissions(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	best := r.MostAutonomousRankedMode()
	require.NotNil(t, best)
	assert.Equal(t, PermissionModeBypassPermissions, best.Mode)
	assert.Equal(t, 6, best.AutonomyRank)
}

func TestFEAT031_PermissionModeCatalog_MostSupervisedRanked_IsPlan(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	safest := r.MostSupervisedRankedMode()
	require.NotNil(t, safest)
	assert.Equal(t, PermissionModePlan, safest.Mode)
	assert.Equal(t, 1, safest.AutonomyRank)
}

func TestFEAT031_PermissionModeCatalog_ModesRequiringFeatureFlag_OnlyAuto(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	flagged := r.ModesRequiringFeatureFlag()
	require.Len(t, flagged, 1)
	assert.Equal(t, PermissionModeAuto, flagged[0].Mode)
	assert.Equal(t, "TRANSCRIPT_CLASSIFIER", flagged[0].ActivationFlag)
}

func TestFEAT031_PermissionModeCatalog_SubagentOnlyModes_OnlyBubble(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	sub := r.SubagentOnlyModes()
	require.Len(t, sub, 1)
	assert.Equal(t, PermissionModeBubble, sub[0].Mode)
}

func TestFEAT031_PermissionModeCatalog_IsKnownMode_ValidSlugs(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	validSlugs := []string{"plan", "default", "acceptEdits", "auto", "dont_ask", "bypassPermissions", "bubble"}
	for _, slug := range validSlugs {
		assert.True(t, r.IsKnownPermissionMode(slug), "slug %q must be known", slug)
	}
}

func TestFEAT031_PermissionModeCatalog_IsKnownMode_InvalidSlugs(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	for _, bad := range []string{"", "unknown", "PLAN", "Auto"} {
		assert.False(t, r.IsKnownPermissionMode(bad), "slug %q must not be known", bad)
	}
}

func TestFEAT031_PermissionModeCatalog_AllModesHaveDenyRulesEnforced(t *testing.T) {
	// §5.1 / §5.2: deny rules always take precedence ("deny-first, ask-by-default").
	// Every mode, even bypassPermissions, still enforces bypass-immune rules.
	r := NewPermissionModeCatalogRegistry()
	for _, e := range r.AllModes() {
		assert.True(t, e.DenyRulesEnforced,
			"mode %q must have DenyRulesEnforced=true per §5.2 deny-first principle", e.Mode)
	}
}

func TestFEAT031_PermissionModeCatalog_ModesByPromptBehavior_AlwaysAsk(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	modes := r.ModesByPromptBehavior(PermissionModePromptAlwaysAsk)
	// plan and default both always prompt.
	require.Len(t, modes, 2)
	modeSlice := []PermissionMode{modes[0].Mode, modes[1].Mode}
	assert.Contains(t, modeSlice, PermissionModePlan)
	assert.Contains(t, modeSlice, PermissionModeDefault)
}

func TestFEAT031_PermissionModeCatalog_BubblePDFSection(t *testing.T) {
	r := NewPermissionModeCatalogRegistry()
	entry, ok := r.FindPermissionModeCatalogEntryBySlug("bubble")
	require.True(t, ok)
	assert.Equal(t, "5.1", entry.PDFSection)
}
