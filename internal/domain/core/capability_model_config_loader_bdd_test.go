package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for CoreCapabilityModelConfig seed (migration 000120).
// Each scenario validates a distinct design decision in the per-agent
// model configuration catalog.

func TestBDD_AhCoreCapabilityModelConfigSeed(t *testing.T) {
	t.Run("Scenario_NineConfigsAcrossThreeAgents", func(t *testing.T) {
		// Given three core capability agents (researcher, analyst, planner)
		// And each agent has exactly three model config keys
		// When the seed migration 000120 runs
		// Then exactly 9 config rows exist in capability_model_config
		const keysPerAgent = 3
		assert.Equal(t, 9, SeedModelConfigCount)
		assert.Equal(t, 3, SeedModelConfigAgentCount)
		assert.Equal(t, SeedModelConfigCount, SeedModelConfigAgentCount*keysPerAgent)
	})

	t.Run("Scenario_ResearcherUsesHaikuForSpeed", func(t *testing.T) {
		// Given core-researcher is a search-heavy agent requiring fast responses
		// When the orchestrator loads its default model config
		// Then the model is claude-haiku-4-5-20251001 (fast + cost-effective)
		// And the temperature is 0.3 (low — factual accuracy)
		// And the analyst model is different (sonnet — higher quality)
		assert.Equal(t, "claude-haiku-4-5-20251001", SeedResearcherModel)
		assert.Equal(t, "0.3", SeedResearcherTemperature)
		assert.NotEqual(t, SeedResearcherModel, SeedAnalystModel,
			"researcher (haiku) uses a different model than analyst (sonnet)")
	})

	t.Run("Scenario_AnalystAndPlannerUseSonnet", func(t *testing.T) {
		// Given core-analyst requires deep reasoning for complex analysis
		// And core-planner requires multi-step reasoning + trade-off evaluation
		// When the orchestrator loads their default model configs
		// Then both use claude-sonnet-4-6 (balanced capability)
		assert.Equal(t, "claude-sonnet-4-6", SeedAnalystModel)
		assert.Equal(t, "claude-sonnet-4-6", SeedPlannerModel)
		assert.Equal(t, SeedAnalystModel, SeedPlannerModel,
			"analyst and planner share the same sonnet-4-6 model")
	})

	t.Run("Scenario_AnalystHasLowestTemperatureForDeterminism", func(t *testing.T) {
		// Given core-analyst produces structured analysis reports
		// And reproducibility and determinism are critical for analysis
		// When the orchestrator reads the analyst temperature config
		// Then temperature is 0.2 — the lowest of the three agents
		assert.Equal(t, "0.2", SeedAnalystTemperature)
		assert.Less(t, SeedAnalystTemperature, SeedResearcherTemperature,
			"analyst must be more deterministic than researcher")
		assert.Less(t, SeedAnalystTemperature, SeedPlannerTemperature,
			"analyst must be more deterministic than planner")
	})

	t.Run("Scenario_PlannerHasHigherTemperatureForCreativity", func(t *testing.T) {
		// Given core-planner must evaluate multiple plan alternatives
		// And creative trade-off exploration requires some sampling diversity
		// When the orchestrator reads the planner temperature config
		// Then temperature is 0.5 — the highest of the three agents
		assert.Equal(t, "0.5", SeedPlannerTemperature)
		assert.Greater(t, SeedPlannerTemperature, SeedAnalystTemperature,
			"planner must be more creative than analyst")
		assert.Greater(t, SeedPlannerTemperature, SeedResearcherTemperature,
			"planner must be more creative than researcher")
	})
}
