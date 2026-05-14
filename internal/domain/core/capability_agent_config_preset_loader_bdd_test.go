package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000098 capability agent config preset seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityAgentConfigPresetSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilityAgentConfigPresets", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000098 seeds capability agent config presets,
		// Then exactly 3 presets are added — one per capability agent — providing
		//   a recommended LLM configuration profile for each capability role.
		assert.Equal(t, 3, SeedCapabilityAgentConfigPresetCount,
			"migration 000098 must seed exactly 3 capability agent config presets")
		assert.Equal(t, 3, len(SeedCapabilityAgentConfigPresetSlugs),
			"slug list must have exactly 3 entries — one per capability agent")
		assert.Len(t, SeedCapabilityAgentConfigTemperatures, 3,
			"temperature map must have exactly 3 entries — one per capability agent config preset")
	})

	t.Run("Scenario_ResearchConfigUsesLowTemperatureForPrecision", func(t *testing.T) {
		// Given the core-researcher agent explores multiple sources to synthesize
		//   research findings, requiring a balance of exploration and attribution
		//   accuracy,
		// When migration 000098 seeds the research agent config preset,
		// Then the preset uses temperature 0.3 — lower than planning (0.5) to
		//   reduce hallucination risk while still allowing synthesis variation,
		//   and large max_tokens (8192) to accommodate multi-source summaries.
		researchTemp := SeedCapabilityAgentConfigTemperatures[SeedResearchAgentConfigSlug]
		planningTemp := SeedCapabilityAgentConfigTemperatures[SeedPlanningAgentConfigSlug]
		assert.Equal(t, 0.3, researchTemp,
			"research agent config must use temperature 0.3 for balanced exploration and attribution")
		assert.Less(t, researchTemp, planningTemp,
			"research temperature (%.1f) must be lower than planning temperature (%.1f)",
			researchTemp, planningTemp)
		assert.Equal(t, 8192, SeedCapabilityAgentConfigMaxTokens[SeedResearchAgentConfigSlug],
			"research agent config must use max_tokens 8192 for multi-source summaries")
	})

	t.Run("Scenario_AnalysisConfigUsesLowestTemperatureForConsistency", func(t *testing.T) {
		// Given the core-analyst agent produces structured document analysis that
		//   requires high consistency and reproducibility across repeated runs,
		// When migration 000098 seeds the analysis agent config preset,
		// Then the preset uses temperature 0.1 — the lowest of all three capability
		//   presets — and the highest max_tokens (16384) to support deep analysis
		//   of large document corpora.
		analysisTemp := SeedCapabilityAgentConfigTemperatures[SeedAnalysisAgentConfigSlug]
		researchTemp := SeedCapabilityAgentConfigTemperatures[SeedResearchAgentConfigSlug]
		planningTemp := SeedCapabilityAgentConfigTemperatures[SeedPlanningAgentConfigSlug]
		assert.Equal(t, 0.1, analysisTemp,
			"analysis agent config must use the lowest temperature (0.1) for consistent, reproducible structured findings")
		assert.Less(t, analysisTemp, researchTemp,
			"analysis temperature (%.1f) must be lower than research temperature (%.1f)",
			analysisTemp, researchTemp)
		assert.Less(t, analysisTemp, planningTemp,
			"analysis temperature (%.1f) must be lower than planning temperature (%.1f)",
			analysisTemp, planningTemp)
		assert.Equal(t, 16384, SeedCapabilityAgentConfigMaxTokens[SeedAnalysisAgentConfigSlug],
			"analysis agent config must use max_tokens 16384 (highest) for large document corpora")
	})

	t.Run("Scenario_PlanningConfigUsesHigherTemperatureForCreativity", func(t *testing.T) {
		// Given the core-planner agent decomposes goals into actionable tasks,
		//   requiring creative flexibility to explore different task structures,
		// When migration 000098 seeds the planning agent config preset,
		// Then the preset uses temperature 0.5 — the highest of all three capability
		//   presets — and lean max_tokens (4096) to keep generated plans concise
		//   and actionable.
		planningTemp := SeedCapabilityAgentConfigTemperatures[SeedPlanningAgentConfigSlug]
		researchTemp := SeedCapabilityAgentConfigTemperatures[SeedResearchAgentConfigSlug]
		analysisTemp := SeedCapabilityAgentConfigTemperatures[SeedAnalysisAgentConfigSlug]
		assert.Equal(t, 0.5, planningTemp,
			"planning agent config must use temperature 0.5 for creative task decomposition")
		assert.Greater(t, planningTemp, researchTemp,
			"planning temperature (%.1f) must be higher than research temperature (%.1f)",
			planningTemp, researchTemp)
		assert.Greater(t, planningTemp, analysisTemp,
			"planning temperature (%.1f) must be higher than analysis temperature (%.1f)",
			planningTemp, analysisTemp)
		assert.Equal(t, 4096, SeedCapabilityAgentConfigMaxTokens[SeedPlanningAgentConfigSlug],
			"planning agent config must use max_tokens 4096 (lean) to keep plans concise")
	})

	t.Run("Scenario_AllPresetsUseAnthropicClaudeSonnet", func(t *testing.T) {
		// Given the capability agents ship with a single opinionated model choice
		//   to reduce configuration friction for fresh tenants,
		// When migration 000098 seeds capability agent config presets,
		// Then all 3 presets use the same provider (anthropic) and the same model
		//   (claude-sonnet-4-6) — tenants can override per-agent later, but the
		//   default provides a consistent out-of-the-box experience.
		assert.Equal(t, "anthropic", SeedCapabilityAgentConfigModelProvider,
			"all capability agent config presets must use the 'anthropic' provider")
		assert.Equal(t, "claude-sonnet-4-6", SeedCapabilityAgentConfigModelID,
			"all capability agent config presets must reference claude-sonnet-4-6")

		// Verify the temperature map covers all 3 slugs (i.e., all 3 presets have
		// an entry — indirect confirmation all 3 use the same model).
		for _, slug := range SeedCapabilityAgentConfigPresetSlugs {
			_, hasTemp := SeedCapabilityAgentConfigTemperatures[slug]
			assert.True(t, hasTemp,
				"slug %q must have an entry in SeedCapabilityAgentConfigTemperatures",
				slug)
			_, hasMaxTokens := SeedCapabilityAgentConfigMaxTokens[slug]
			assert.True(t, hasMaxTokens,
				"slug %q must have an entry in SeedCapabilityAgentConfigMaxTokens",
				slug)
		}
	})
}
