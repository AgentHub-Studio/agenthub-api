package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FEAT026 BDD tests — LimitationRegistry
// PDF reference: Appendix B §B.3 "Limitations" (page 45)
// ---------------------------------------------------------------------------

func TestFEAT026_BDD_LimitationRegistry(t *testing.T) {

	// Scenario 1: The registry boots with exactly four named limitations
	t.Run("Scenario_RegistryBootsWithFourLimitations", func(t *testing.T) {
		// Given a freshly constructed registry
		registry := NewLimitationRegistry()

		// When the full list of limitations is retrieved
		all := registry.All()

		// Then exactly four profiles are present, each with a non-empty slug and label
		assert.Len(t, all, SeedLimitationCount,
			"§B.3 names exactly four study limitations")
		for _, p := range all {
			assert.NotEmpty(t, p.Slug, "every limitation must have a slug")
			assert.NotEmpty(t, p.Label, "every limitation must have a label")
		}
	})

	// Scenario 2: The static-snapshot limitation flags feature-flag build variability
	t.Run("Scenario_StaticSnapshotFlagsFeatureFlagVariability", func(t *testing.T) {
		// Given the registry
		registry := NewLimitationRegistry()

		// When the static_snapshot profile is retrieved
		p, ok := registry.FindLimitationBySlug("static_snapshot")
		require.True(t, ok, "static_snapshot limitation must exist")

		// Then its description references the analysed version and two feature flags
		// that create build-time variability per §B.3
		assert.Equal(t, LimitationTypeTemporal, p.LimitationType)
		assert.Contains(t, p.Description, "v2.1.88",
			"must reference the specific version under analysis")
		assert.Contains(t, p.Description, "TRANSCRIPT_CLASSIFIER",
			"must mention TRANSCRIPT_CLASSIFIER feature flag")
		assert.Contains(t, p.Description, "CONTEXT_COLLAPSE",
			"must mention CONTEXT_COLLAPSE feature flag")
		// And it affects every claim type (not just a subset)
		assert.Equal(t, AffectedClaimTypeAll, p.AffectsClaimType)
		assert.True(t, p.RequiresHedgedGeneralisation)
	})

	// Scenario 3: Reverse-engineering epistemology limits design-intent claims
	t.Run("Scenario_ReverseEngineeringBoundsDesignIntentClaims", func(t *testing.T) {
		// Given the registry
		registry := NewLimitationRegistry()

		// When the reverse-engineering epistemology limitation is retrieved
		p, ok := registry.FindLimitationBySlug("reverse_engineering_epistemology")
		require.True(t, ok, "reverse_engineering_epistemology must exist")

		// Then it is marked epistemological and specifically affects design-intent claims
		assert.True(t, p.IsEpistemological,
			"reverse-engineering bounds what can be known, not what exists")
		assert.Equal(t, AffectedClaimTypeDesignIntent, p.AffectsClaimType)
		assert.Contains(t, p.Description, "design intent",
			"description must explicitly name design intent as a bounded claim type")
		// And exactly one limitation in the registry is epistemological
		epistemological := registry.EpistemologicalLimitations()
		assert.Len(t, epistemological, 1,
			"only the reverse-engineering limitation is epistemological")
	})

	// Scenario 4: Single-system scope bounds generalisability across coding agents
	t.Run("Scenario_SingleSystemAnalysisBoundsGeneralisation", func(t *testing.T) {
		// Given the registry
		registry := NewLimitationRegistry()

		// When the single_system_analysis profile is retrieved
		p, ok := registry.FindLimitationBySlug("single_system_analysis")
		require.True(t, ok, "single_system_analysis must exist")

		// Then it carries the scope type and explicitly states that generalisations
		// are bounded, as written in §B.3
		assert.Equal(t, LimitationTypeScope, p.LimitationType)
		assert.Contains(t, p.Description, "bounded",
			"§B.3 says 'Generalisations are bounded.'")
		assert.True(t, p.RequiresHedgedGeneralisation)
		// And it affects comparative claims (cross-agent comparisons)
		assert.Equal(t, AffectedClaimTypeComparative, p.AffectsClaimType)
	})

	// Scenario 5: OpenClaw calibration baseline is a snapshot, not ground truth
	t.Run("Scenario_OpenClawSnapshotIsCalibrationNotGroundTruth", func(t *testing.T) {
		// Given the registry
		registry := NewLimitationRegistry()

		// When the openclaw_snapshot profile is retrieved
		p, ok := registry.FindLimitationBySlug("openclaw_snapshot")
		require.True(t, ok, "openclaw_snapshot must exist")

		// Then it is typed as calibration and its mitigation notes state that
		// OpenClaw is used for calibration only, not cited as ground truth
		assert.Equal(t, LimitationTypeCalibration, p.LimitationType)
		assert.Contains(t, p.MitigationNotes, "calibration",
			"mitigation must note OpenClaw is used for calibration only")
		assert.Contains(t, p.MitigationNotes, "not cited as ground truth",
			"mitigation must state that OpenClaw is not used as ground truth")
		assert.Equal(t, AffectedClaimTypeComparative, p.AffectsClaimType)
	})

	// Scenario 6: All four limitations require hedged generalisations
	t.Run("Scenario_AllLimitationsRequireHedgedGeneralisation", func(t *testing.T) {
		// Given the registry
		registry := NewLimitationRegistry()

		// When the hedged-generalisation limitations are collected
		hedged := registry.HedgedGeneralisationLimitations()

		// Then all four named limitations require bounded/hedged generalisations —
		// none of the §B.3 limitations is unconditional
		assert.Len(t, hedged, SeedLimitationCount,
			"every §B.3 limitation bounds how widely findings can be generalised")
	})

	// Scenario 7: A caller can locate any limitation by its exact §B.3 label
	t.Run("Scenario_FindByLabelRoundTrip", func(t *testing.T) {
		// Given the registry
		registry := NewLimitationRegistry()

		// When each known label is looked up
		labels := []string{
			"Static snapshot.",
			"Reverse-engineering epistemology.",
			"Single-system analysis.",
			"OpenClaw snapshot.",
		}
		for _, label := range labels {
			// Then the corresponding profile is found and its label matches exactly
			p, ok := registry.FindLimitationByLabel(label)
			assert.Truef(t, ok, "label %q must resolve to a profile", label)
			if ok {
				assert.Equal(t, label, p.Label)
			}
		}
	})

	// Scenario 8: Type-based lookup returns the correct subset of limitations
	t.Run("Scenario_TypeBasedLookupReturnsCorrectSubsets", func(t *testing.T) {
		// Given the registry
		registry := NewLimitationRegistry()

		// When limitations are queried by each limitation type
		temporal := registry.FindLimitationsByType(LimitationTypeTemporal)
		epistemological := registry.FindLimitationsByType(LimitationTypeEpistemology)
		scope := registry.FindLimitationsByType(LimitationTypeScope)
		calibration := registry.FindLimitationsByType(LimitationTypeCalibration)

		// Then each type yields exactly one limitation and covers all four profiles
		assert.Len(t, temporal, 1, "one temporal limitation: static_snapshot")
		assert.Len(t, epistemological, 1, "one epistemological limitation: reverse_engineering_epistemology")
		assert.Len(t, scope, 1, "one scope limitation: single_system_analysis")
		assert.Len(t, calibration, 1, "one calibration limitation: openclaw_snapshot")

		// The union covers all four seeded limitations without overlap
		total := len(temporal) + len(epistemological) + len(scope) + len(calibration)
		assert.Equal(t, SeedLimitationCount, total,
			"each limitation belongs to exactly one type")
	})
}
