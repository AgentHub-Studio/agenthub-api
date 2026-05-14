package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFEAT031_BDD_PermissionModeCatalog contains behavioral scenarios that
// verify the §5.1 seven-mode taxonomy as described in the paper.

// Scenario A — §5.1 complete enumeration: all seven modes are registered and
// their visibility tiers sum to the correct counts (5 external + 1 conditional + 1 internal).
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioA_SevenModeVisibilityBreakdown(t *testing.T) {
	// Given a freshly constructed catalog registry
	r := NewPermissionModeCatalogRegistry()

	// When we retrieve all modes
	all := r.AllModes()

	// Then there are exactly seven
	require.Len(t, all, 7,
		"§5.1 enumerates seven permission modes")

	// And they break down as 5 external + 1 conditional + 1 internal
	var external, conditional, internal int
	for _, e := range all {
		switch e.Visibility {
		case PermissionModeVisibilityExternal:
			external++
		case PermissionModeVisibilityConditional:
			conditional++
		case PermissionModeVisibilityInternal:
			internal++
		}
	}
	assert.Equal(t, 5, external, "five externally visible modes")
	assert.Equal(t, 1, conditional, "one conditionally included mode (auto)")
	assert.Equal(t, 1, internal, "one internal-only mode (bubble)")
}

// Scenario B — §5.1 graduated autonomy spectrum: the six ranked modes form a
// strictly increasing autonomy sequence from plan (1) to bypassPermissions (6).
// Bubble is unranked (autonomy_rank=0) because it is an escalation mechanism.
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioB_AutonomyRankSpectrum(t *testing.T) {
	// Given the catalog registry
	r := NewPermissionModeCatalogRegistry()

	// When we collect all ranked modes
	var ranked []*PermissionModeCatalogEntry
	for _, e := range r.AllModes() {
		if e.AutonomyRank > 0 {
			ranked = append(ranked, e)
		}
	}

	// Then six modes are ranked
	require.Len(t, ranked, 6)

	// And the ranks are 1 through 6 without gaps or duplicates
	seen := make(map[int]bool)
	for _, e := range ranked {
		assert.False(t, seen[e.AutonomyRank], "rank %d must appear exactly once", e.AutonomyRank)
		seen[e.AutonomyRank] = true
	}
	for rank := 1; rank <= 6; rank++ {
		assert.True(t, seen[rank], "rank %d must be present", rank)
	}

	// And bubble has autonomy_rank=0 (outside the spectrum)
	bubbleEntry, ok := r.FindPermissionModeCatalogEntryBySlug("bubble")
	require.True(t, ok)
	assert.Equal(t, 0, bubbleEntry.AutonomyRank)
}

// Scenario C — §5.1 dontAsk is external but NOT in the safety gradient:
// The paper lists it in EXTERNAL_PERMISSION_MODES but it is not between any
// gradient positions. DenyRulesEnforced remains true ("deny rules are still enforced").
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioC_DontAskExternalNotGradient(t *testing.T) {
	// Given the catalog registry
	r := NewPermissionModeCatalogRegistry()

	// When we look up dontAsk
	entry, ok := r.FindPermissionModeCatalogEntryBySlug("dont_ask")
	require.True(t, ok, "dontAsk must be a known mode")

	// Then it is externally visible
	assert.Equal(t, PermissionModeVisibilityExternal, entry.Visibility,
		"§5.1: dontAsk is in EXTERNAL_PERMISSION_MODES")

	// And it is NOT in the §11.3 safety gradient
	assert.False(t, entry.InGradient,
		"§5.1: dontAsk is external but outside the graduated autonomy spectrum")

	// And deny rules are still enforced despite no prompting
	assert.True(t, entry.DenyRulesEnforced,
		"§5.1: dontAsk — 'No prompting, but deny rules are still enforced'")

	// And it is NOT subagent-only
	assert.False(t, entry.SubagentOnly)

	// And it is not gated by a feature flag
	assert.Empty(t, entry.ActivationFlag)
}

// Scenario D — §5.1 auto mode is conditionally gated by TRANSCRIPT_CLASSIFIER.
// It belongs to the five-mode safety gradient but is NOT always available.
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioD_AutoConditionalOnTranscriptClassifier(t *testing.T) {
	// Given the catalog registry
	r := NewPermissionModeCatalogRegistry()

	// When we look up auto
	entry, ok := r.FindPermissionModeCatalogEntryBySlug("auto")
	require.True(t, ok)

	// Then its visibility is conditional (not external, not internal)
	assert.Equal(t, PermissionModeVisibilityConditional, entry.Visibility)

	// And the activation flag is TRANSCRIPT_CLASSIFIER
	assert.Equal(t, "TRANSCRIPT_CLASSIFIER", entry.ActivationFlag)

	// But it IS in the safety gradient
	assert.True(t, entry.InGradient,
		"auto participates in the §11.3 five-mode gradient")

	// And its prompt behavior uses the ML classifier
	assert.Equal(t, PermissionModePromptMLClassifier, entry.PromptBehavior)
}

// Scenario E — §5.1 bubble is internal-only, subagent-only, and unranked:
// The paper states it "exists in the type union but not in either mode array;
// it is used internally for subagent permission escalation (Section 8)."
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioE_BubbleInternalSubagentEscalation(t *testing.T) {
	// Given the catalog registry
	r := NewPermissionModeCatalogRegistry()

	// When we retrieve internal-only modes
	internal := r.InternalModes()

	// Then bubble is the only one
	require.Len(t, internal, 1)
	bubble := internal[0]
	assert.Equal(t, PermissionModeBubble, bubble.Mode)

	// And it is subagent-only
	assert.True(t, bubble.SubagentOnly,
		"§5.1: bubble is used for subagent permission escalation")

	// And it is not in the gradient
	assert.False(t, bubble.InGradient)

	// And its prompt behavior is escalation (not user-facing prompting)
	assert.Equal(t, PermissionModePromptEscalate, bubble.PromptBehavior)

	// And SubagentOnlyModes() returns exactly bubble
	subOnly := r.SubagentOnlyModes()
	require.Len(t, subOnly, 1)
	assert.Equal(t, PermissionModeBubble, subOnly[0].Mode)
}

// Scenario F — §5.2 deny-first principle: every mode enforces deny rules.
// The paper states "deny rules always win, even under looser modes" and
// §5.2 confirms "A deny rule always takes precedence over an allow rule."
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioF_DenyFirstPrincipleAcrossAllModes(t *testing.T) {
	// Given the catalog registry
	r := NewPermissionModeCatalogRegistry()

	// When we inspect every mode's DenyRulesEnforced flag
	for _, entry := range r.AllModes() {
		// Then deny rules are enforced regardless of autonomy level
		assert.True(t, entry.DenyRulesEnforced,
			"mode %q must enforce deny rules — §5.2 deny-first principle", entry.Mode)
	}
}

// Scenario G — gradient vs. non-gradient split: exactly five modes form the §11.3
// safety gradient; the two non-gradient modes are dontAsk and bubble, which are
// orthogonal mechanisms rather than positions on the autonomy spectrum.
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioG_GradientVsNonGradientSplit(t *testing.T) {
	// Given the catalog registry
	r := NewPermissionModeCatalogRegistry()

	// When we separate gradient from non-gradient modes
	grad := r.GradientModes()
	nonGrad := r.NonGradientModes()

	// Then gradient has exactly five
	assert.Len(t, grad, 5, "§11.3 gradient covers five modes")

	// And non-gradient has exactly two
	assert.Len(t, nonGrad, 2, "dontAsk and bubble are outside the gradient")

	// And the five gradient modes are the expected set
	gradModeSet := make(map[PermissionMode]bool)
	for _, e := range grad {
		gradModeSet[e.Mode] = true
	}
	for _, expected := range []PermissionMode{
		PermissionModePlan, PermissionModeDefault, PermissionModeAcceptEdits,
		PermissionModeAuto, PermissionModeBypassPermissions,
	} {
		assert.True(t, gradModeSet[expected], "mode %q must be in gradient", expected)
	}

	// And the two non-gradient modes are dontAsk and bubble
	nonGradSet := make(map[PermissionMode]bool)
	for _, e := range nonGrad {
		nonGradSet[e.Mode] = true
	}
	assert.True(t, nonGradSet[PermissionModeDontAsk])
	assert.True(t, nonGradSet[PermissionModeBubble])
}

// Scenario H — catalog complements (not conflicts with) existing gradient manager:
// The five modes that are InGradient in the catalog must correspond exactly to the
// five modes in the existing PermissionGradient slice from permission_mode_gradient.go.
func TestFEAT031_BDD_PermissionModeCatalog_ScenarioH_CatalogConsistentWithExistingGradient(t *testing.T) {
	// Given the catalog registry and the existing gradient slice
	r := NewPermissionModeCatalogRegistry()
	catalogGradient := r.GradientModes()

	// When we build a set from the existing PermissionGradient
	existingGradientSet := make(map[PermissionMode]bool)
	for _, m := range PermissionGradient {
		existingGradientSet[m] = true
	}

	// Then every catalog gradient mode is also in the existing gradient
	for _, e := range catalogGradient {
		assert.True(t, existingGradientSet[e.Mode],
			"catalog gradient mode %q must be in existing PermissionGradient slice", e.Mode)
	}

	// And the sizes match
	assert.Equal(t, len(PermissionGradient), len(catalogGradient))
}
