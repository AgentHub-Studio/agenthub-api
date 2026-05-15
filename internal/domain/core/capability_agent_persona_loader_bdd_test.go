package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000104 capability agent persona seeds.
// These assert seed shape and identity-model alignment without a database.

func TestBDD_CapabilityAgentPersonaSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilityAgentPersonasOnePerAgent", func(t *testing.T) {
		// Given the capability layer ships with three specialist agents
		//   (researcher, analyst, planner) each requiring a distinct identity profile,
		// When migration 000104 seeds persona rows,
		// Then exactly 3 rows are added — one persona per capability agent — and
		//   the slug slice length matches the declared count constant.
		assert.Equal(t, 3, SeedCapabilityAgentPersonaCount,
			"migration 000104 must seed exactly 3 capability agent persona rows")
		assert.Len(t, SeedCapabilityAgentPersonaSlugs, 3,
			"SeedCapabilityAgentPersonaSlugs must list exactly 3 persona slugs")
		assert.Equal(t, SeedCapabilityAgentPersonaCount, len(SeedCapabilityAgentPersonaSlugs),
			"SeedCapabilityAgentPersonaCount must equal len(SeedCapabilityAgentPersonaSlugs)")
	})

	t.Run("Scenario_ResearcherPersonaHasCuriousToneAdaptedFromClaudeCodeIdentity", func(t *testing.T) {
		// Given the researcher capability agent is modelled after the curious,
		//   source-aware inquiry style described in Claude Code Identity Model §3,
		// When migration 000104 seeds the researcher persona,
		// Then the tone constant is "curious", the slug is canonical, and the
		//   slug appears in the full persona slug list.
		assert.Equal(t, "curious", SeedResearcherTone,
			"researcher persona tone must be 'curious' — derived from Identity Model §3 curious/methodical traits")
		assert.Equal(t, "capability-researcher-persona", SeedResearcherPersonaSlug,
			"SeedResearcherPersonaSlug must equal \"capability-researcher-persona\"")
		assert.Contains(t, SeedCapabilityAgentPersonaSlugs, SeedResearcherPersonaSlug,
			"researcher persona slug must be present in SeedCapabilityAgentPersonaSlugs")
	})

	t.Run("Scenario_AnalystPersonaHasPreciseToneForEvidenceBasedWork", func(t *testing.T) {
		// Given the analyst capability agent requires a precise, evidence-driven
		//   communication style to deliver trustworthy document analysis,
		// When migration 000104 seeds the analyst persona,
		// Then the tone constant is "precise", the slug is canonical, and the
		//   slug appears in the full persona slug list.
		assert.Equal(t, "precise", SeedAnalystTone,
			"analyst persona tone must be 'precise' — evidence-driven analysis demands exact, unambiguous communication")
		assert.Equal(t, "capability-analyst-persona", SeedAnalystPersonaSlug,
			"SeedAnalystPersonaSlug must equal \"capability-analyst-persona\"")
		assert.Contains(t, SeedCapabilityAgentPersonaSlugs, SeedAnalystPersonaSlug,
			"analyst persona slug must be present in SeedCapabilityAgentPersonaSlugs")
	})

	t.Run("Scenario_PlannerPersonaHasPragmaticToneForActionOrientation", func(t *testing.T) {
		// Given the planner capability agent must decompose tasks into actionable
		//   steps with clear dependencies, favouring practical outcomes over theory,
		// When migration 000104 seeds the planner persona,
		// Then the tone constant is "pragmatic", the slug is canonical, and the
		//   slug appears in the full persona slug list.
		assert.Equal(t, "pragmatic", SeedPlannerTone,
			"planner persona tone must be 'pragmatic' — action-oriented decomposition demands a results-first mindset")
		assert.Equal(t, "capability-planner-persona", SeedPlannerPersonaSlug,
			"SeedPlannerPersonaSlug must equal \"capability-planner-persona\"")
		assert.Contains(t, SeedCapabilityAgentPersonaSlugs, SeedPlannerPersonaSlug,
			"planner persona slug must be present in SeedCapabilityAgentPersonaSlugs")
	})

	t.Run("Scenario_EachPersonaHasFourDistinctTraits", func(t *testing.T) {
		// Given each capability agent persona captures four personality trait labels
		//   providing a concise but rich identity fingerprint,
		// When migration 000104 seeds trait arrays for each persona,
		// Then each per-persona trait count constant is 4, the total is 12, and
		//   the sum of per-persona counts equals the total trait count.
		assert.Equal(t, 4, SeedResearcherTraitCount,
			"researcher persona must have exactly 4 traits (methodical/thorough/source-aware/skeptical)")
		assert.Equal(t, 4, SeedAnalystTraitCount,
			"analyst persona must have exactly 4 traits (analytical/evidence-driven/systematic/objective)")
		assert.Equal(t, 4, SeedPlannerTraitCount,
			"planner persona must have exactly 4 traits (structured/action-oriented/dependency-aware/iterative)")
		assert.Equal(t, 12, SeedTotalTraitCount,
			"total trait count must be 12 across all 3 seeded personas (3 × 4)")
		traitSum := SeedResearcherTraitCount + SeedAnalystTraitCount + SeedPlannerTraitCount
		assert.Equal(t, SeedTotalTraitCount, traitSum,
			"SeedTotalTraitCount must equal the sum of all per-persona trait counts")
	})
}
