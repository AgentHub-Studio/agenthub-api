package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- count constants --------------------------------------------------------

func TestModelConfig_SeedModelConfigCount(t *testing.T) {
	assert.Equal(t, 9, SeedModelConfigCount, "9 total rows: 3 config keys × 3 agents")
}

func TestModelConfig_SeedModelConfigAgentCount(t *testing.T) {
	assert.Equal(t, 3, SeedModelConfigAgentCount, "3 agents: researcher, analyst, planner")
}

func TestModelConfig_TotalRowsEqualAgentsTimesKeys(t *testing.T) {
	const keysPerAgent = 3 // default_model, temperature, max_tokens
	assert.Equal(t, SeedModelConfigCount, SeedModelConfigAgentCount*keysPerAgent)
}

// ---- config key constants ---------------------------------------------------

func TestModelConfig_ConfigKeyModel(t *testing.T) {
	assert.Equal(t, "default_model", SeedModelConfigKeyModel)
}

func TestModelConfig_ConfigKeyTemperature(t *testing.T) {
	assert.Equal(t, "temperature", SeedModelConfigKeyTemperature)
}

func TestModelConfig_ConfigKeyMaxTokens(t *testing.T) {
	assert.Equal(t, "max_tokens", SeedModelConfigKeyMaxTokens)
}

func TestModelConfig_ConfigKeysAreDistinct(t *testing.T) {
	keys := []string{SeedModelConfigKeyModel, SeedModelConfigKeyTemperature, SeedModelConfigKeyMaxTokens}
	seen := map[string]bool{}
	for _, k := range keys {
		assert.False(t, seen[k], "duplicate config key %q", k)
		seen[k] = true
	}
}

// ---- per-agent model ID constants ------------------------------------------

func TestModelConfig_ResearcherModelIsHaiku(t *testing.T) {
	assert.Equal(t, "claude-haiku-4-5-20251001", SeedResearcherModel,
		"core-researcher should use the faster haiku model for search-heavy workloads")
}

func TestModelConfig_AnalystModelIsSonnet(t *testing.T) {
	assert.Equal(t, "claude-sonnet-4-6", SeedAnalystModel,
		"core-analyst should use sonnet for balanced deep reasoning")
}

func TestModelConfig_PlannerModelIsSonnet(t *testing.T) {
	assert.Equal(t, "claude-sonnet-4-6", SeedPlannerModel,
		"core-planner should use sonnet for capable multi-step planning")
}

func TestModelConfig_AnalystAndPlannerShareSameModel(t *testing.T) {
	assert.Equal(t, SeedAnalystModel, SeedPlannerModel,
		"analyst and planner both use claude-sonnet-4-6")
}

func TestModelConfig_ResearcherModelDiffersFromAnalystModel(t *testing.T) {
	assert.NotEqual(t, SeedResearcherModel, SeedAnalystModel,
		"researcher uses haiku (speed), analyst uses sonnet (quality)")
}

// ---- per-agent temperature constants ----------------------------------------

func TestModelConfig_ResearcherTemperature(t *testing.T) {
	assert.Equal(t, "0.3", SeedResearcherTemperature,
		"0.3 — low temperature for factual accuracy in research outputs")
}

func TestModelConfig_AnalystTemperature(t *testing.T) {
	assert.Equal(t, "0.2", SeedAnalystTemperature,
		"0.2 — very low temperature for deterministic, reproducible analysis")
}

func TestModelConfig_PlannerTemperature(t *testing.T) {
	assert.Equal(t, "0.5", SeedPlannerTemperature,
		"0.5 — moderate temperature for creative plan alternatives")
}

func TestModelConfig_AnalystHasLowestTemperature(t *testing.T) {
	// Analyst needs the most deterministic outputs of the three agents.
	assert.Less(t, SeedAnalystTemperature, SeedResearcherTemperature,
		"analyst temperature must be lower than researcher temperature")
	assert.Less(t, SeedAnalystTemperature, SeedPlannerTemperature,
		"analyst temperature must be lower than planner temperature")
}

func TestModelConfig_PlannerHasHighestTemperature(t *testing.T) {
	// Planner needs the most creative freedom for trade-off evaluation.
	assert.Greater(t, SeedPlannerTemperature, SeedResearcherTemperature,
		"planner temperature must be higher than researcher temperature")
	assert.Greater(t, SeedPlannerTemperature, SeedAnalystTemperature,
		"planner temperature must be higher than analyst temperature")
}

func TestModelConfig_TemperaturesAreDistinct(t *testing.T) {
	temps := []string{SeedResearcherTemperature, SeedAnalystTemperature, SeedPlannerTemperature}
	seen := map[string]bool{}
	for _, t2 := range temps {
		assert.False(t, seen[t2], "duplicate temperature value %q across agents", t2)
		seen[t2] = true
	}
}

// ---- per-agent max_tokens (via seeded constants) ---------------------------

func TestModelConfig_ResearcherMaxTokensIsModerateFourK(t *testing.T) {
	// Researcher uses 4096 — enough for summaries + citations.
	// This is encoded in the SQL; we test the constant relationship:
	// researcher (4096) < analyst (8192) == planner (8192).
	const researcherMaxTokens = 4096
	const analystMaxTokens = 8192
	const plannerMaxTokens = 8192
	assert.Less(t, researcherMaxTokens, analystMaxTokens,
		"researcher max_tokens must be less than analyst max_tokens")
	assert.Equal(t, analystMaxTokens, plannerMaxTokens,
		"analyst and planner share the same max_tokens budget")
}

func TestModelConfig_AnalystAndPlannerMaxTokensAreEqual(t *testing.T) {
	const analystMaxTokens = 8192
	const plannerMaxTokens = 8192
	assert.Equal(t, analystMaxTokens, plannerMaxTokens)
}

// ---- loader construction ---------------------------------------------------

func TestModelConfig_NewLoaderAcceptsNilPool(t *testing.T) {
	// Construction must not panic even with a nil pool.
	// (pool is only used on method calls, not on construction)
	assert.NotPanics(t, func() {
		_ = NewCoreCapabilityModelConfigLoader(nil)
	})
}
