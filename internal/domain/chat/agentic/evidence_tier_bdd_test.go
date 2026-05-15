package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FEAT025 BDD tests — EvidenceTierRegistry
// PDF reference: Appendix B §B.1 (page 45)
// ---------------------------------------------------------------------------

func TestFEAT025_BDD_EvidenceTierRegistry(t *testing.T) {

	// Scenario 1: The registry boots with exactly three evidence tiers
	t.Run("Scenario_RegistryBootsWithThreeTiers", func(t *testing.T) {
		// Given a freshly constructed registry
		registry := NewEvidenceTierRegistry()

		// When the full list of tiers is retrieved
		tiers := registry.All()

		// Then exactly three tiers are present (Tier A, B, C per §B.1)
		assert.Len(t, tiers, 3, "§B.1 defines exactly three evidence tiers")
		labels := map[EvidenceTierLabel]bool{}
		for _, p := range tiers {
			labels[p.Label] = true
		}
		assert.True(t, labels[EvidenceTierLabelA], "Tier A must be present")
		assert.True(t, labels[EvidenceTierLabelB], "Tier B must be present")
		assert.True(t, labels[EvidenceTierLabelC], "Tier C must be present")
	})

	// Scenario 2: Tier B is designated the strongest evidence tier
	t.Run("Scenario_TierBIsStrongest", func(t *testing.T) {
		// Given the registry
		registry := NewEvidenceTierRegistry()

		// When the strongest tier is queried
		strongest, ok := registry.StrongestTier()

		// Then it is Tier B (code-verified), exactly as stated in §B.1
		require.True(t, ok, "a strongest tier must exist")
		assert.Equal(t, EvidenceTierLabelB, strongest.Label)
		assert.Equal(t, EvidenceStrengthStrong, strongest.Strength)
		assert.Equal(t, "code-verified", strongest.Name)
	})

	// Scenario 3: Tier C claims must be hedged; Tiers A and B need not be
	t.Run("Scenario_OnlyTierCRequiresHedging", func(t *testing.T) {
		// Given the registry
		registry := NewEvidenceTierRegistry()

		// When hedged and direct-claim tiers are collected
		hedged := registry.HedgedTiers()
		direct := registry.DirectClaimTiers()

		// Then only Tier C is hedged and both Tier A and Tier B support direct claims
		require.Len(t, hedged, 1)
		assert.Equal(t, EvidenceTierLabelC, hedged[0].Label)
		assert.Len(t, direct, 2)
	})

	// Scenario 4: A caller can locate any tier by its single-letter label
	t.Run("Scenario_FindByLabelRoundTrip", func(t *testing.T) {
		// Given the registry
		registry := NewEvidenceTierRegistry()

		// When each tier label is looked up
		for _, label := range []EvidenceTierLabel{
			EvidenceTierLabelA,
			EvidenceTierLabelB,
			EvidenceTierLabelC,
		} {
			// Then the corresponding profile is found and its label matches
			p, ok := registry.FindEvidenceTierByLabel(label)
			assert.Truef(t, ok, "label %q must resolve", label)
			if ok {
				assert.Equal(t, label, p.Label)
			}
		}
	})

	// Scenario 5: Tier B description anchors claims to the npm-extracted codebase
	t.Run("Scenario_TierBAnchorsToCodebaseVersion", func(t *testing.T) {
		// Given the registry
		registry := NewEvidenceTierRegistry()

		// When Tier B is retrieved by slug
		tierB, ok := registry.FindEvidenceTierBySlug("tier_b")
		require.True(t, ok)

		// Then its description references the exact package version (v2.1.88)
		// and the npm extraction mechanism as stated in §B.1
		assert.Contains(t, tierB.Description, "v2.1.88",
			"Tier B must cite the specific codebase version")
		assert.Contains(t, tierB.TypicalSource, "npm",
			"Tier B source must reference npm extraction")
		assert.True(t, tierB.CanSupportDirectClaims)
	})

	// Scenario 6: Tier C sources include OpenClaw and community analysis
	t.Run("Scenario_TierCReconstructedSources", func(t *testing.T) {
		// Given the registry
		registry := NewEvidenceTierRegistry()

		// When Tier C is retrieved by slug
		tierC, ok := registry.FindEvidenceTierBySlug("tier_c")
		require.True(t, ok)

		// Then it references OpenClaw and community analysis as source types
		assert.Contains(t, tierC.Description, "OpenClaw",
			"Tier C must reference OpenClaw as a calibration source per §B.1")
		assert.Contains(t, tierC.Description, "community analysis",
			"Tier C must reference community analysis per §B.1")
		assert.False(t, tierC.CanSupportDirectClaims)
	})

	// Scenario 7: All tier profiles carry a non-empty PDF section reference
	t.Run("Scenario_AllTiersHavePDFSection", func(t *testing.T) {
		// Given the registry
		registry := NewEvidenceTierRegistry()

		// When each tier is examined
		for _, tier := range registry.All() {
			// Then its PDFSection is non-empty
			assert.NotEmptyf(t, tier.PDFSection,
				"tier %q must carry a PDF section reference", tier.Slug)
		}
	})

	// Scenario 8: Strength values across the three tiers are mutually distinct
	t.Run("Scenario_StrengthValuesAreDistinct", func(t *testing.T) {
		// Given the registry
		registry := NewEvidenceTierRegistry()

		// When the strength values of all tiers are collected
		seen := map[EvidenceStrength]bool{}
		for _, tier := range registry.All() {
			seen[tier.Strength] = true
		}

		// Then all three strength levels are represented — strong, moderate, speculative
		assert.Len(t, seen, SeedEvidenceTierCount,
			"each tier must have a unique strength level")
		assert.True(t, seen[EvidenceStrengthStrong])
		assert.True(t, seen[EvidenceStrengthModerate])
		assert.True(t, seen[EvidenceStrengthSpeculative])
	})
}
