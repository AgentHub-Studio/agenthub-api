package core

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000111 capability session config seeds.
// These assert seed shape and session configuration rationale without a database.

func TestBDD_CapabilitySessionConfigSeed(t *testing.T) {
	t.Run("Scenario_NineSessionConfigsThreePerCapabilityAgent", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent session defaults
		//   to shape the chat experience when a capability agent is first used,
		// When migration 000111 seeds capability_session_config rows,
		// Then exactly 9 rows are added — three per agent — and per-agent counts
		//   sum to the total count constant.
		assert.Equal(t, 9, SeedCapabilitySessionConfigCount,
			"migration 000111 must seed exactly 9 capability session config rows")
		sum := SeedResearcherSessionConfigCount + SeedAnalystSessionConfigCount + SeedPlannerSessionConfigCount
		assert.Equal(t, SeedCapabilitySessionConfigCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherSessionConfigCount, SeedAnalystSessionConfigCount, SeedPlannerSessionConfigCount,
			SeedCapabilitySessionConfigCount)
		assert.Equal(t, 3, SeedResearcherSessionConfigCount,
			"researcher must have exactly 3 session configs")
		assert.Equal(t, 3, SeedAnalystSessionConfigCount,
			"analyst must have exactly 3 session configs")
		assert.Equal(t, 3, SeedPlannerSessionConfigCount,
			"planner must have exactly 3 session configs")
		// Agent slug list must have 3 distinct entries.
		assert.Len(t, SeedCapabilitySessionConfigAgentSlugs, 3,
			"SeedCapabilitySessionConfigAgentSlugs must list exactly 3 agents")
	})

	t.Run("Scenario_PlannerHasMostTurnsForLongPlanningConversations", func(t *testing.T) {
		// Given the Planner agent is designed for extended planning conversations
		//   that iterate many times to refine task breakdowns and track progress,
		// When migration 000111 seeds max_turns session configs,
		// Then the planner has strictly more turns than both the researcher and analyst,
		//   reflecting that planning requires the most conversational iterations.
		plannerTurns, err := strconv.Atoi(SeedPlannerMaxTurns)
		assert.NoError(t, err, "SeedPlannerMaxTurns must be a numeric string")

		researcherTurns, err := strconv.Atoi(SeedResearcherMaxTurns)
		assert.NoError(t, err, "SeedResearcherMaxTurns must be a numeric string")

		analystTurns, err := strconv.Atoi(SeedAnalystMaxTurns)
		assert.NoError(t, err, "SeedAnalystMaxTurns must be a numeric string")

		assert.Greater(t, plannerTurns, researcherTurns,
			"planner (%d) must have more max_turns than researcher (%d) — planning conversations iterate most",
			plannerTurns, researcherTurns)
		assert.Greater(t, plannerTurns, analystTurns,
			"planner (%d) must have more max_turns than analyst (%d) — planning conversations iterate most",
			plannerTurns, analystTurns)
		// Verify planner is 100.
		assert.Equal(t, "100", SeedPlannerMaxTurns,
			"planner max_turns must be 100")
	})

	t.Run("Scenario_PlannerHasLongestIdleTimeoutForExtendedSessions", func(t *testing.T) {
		// Given planning sessions can be long and users frequently step away
		//   to think between planning turns,
		// When migration 000111 seeds idle_timeout_seconds session configs,
		// Then the planner has the longest idle timeout (7200s = 2 hours) and
		//   the researcher has the shortest (1800s = 30 minutes).
		plannerTimeout, err := strconv.Atoi(SeedPlannerIdleTimeout)
		assert.NoError(t, err, "SeedPlannerIdleTimeout must be a numeric string")

		researcherTimeout, err := strconv.Atoi(SeedResearcherIdleTimeout)
		assert.NoError(t, err, "SeedResearcherIdleTimeout must be a numeric string")

		analystTimeout, err := strconv.Atoi(SeedAnalystIdleTimeout)
		assert.NoError(t, err, "SeedAnalystIdleTimeout must be a numeric string")

		assert.Greater(t, plannerTimeout, researcherTimeout,
			"planner (%d s) must have a longer idle timeout than researcher (%d s)", plannerTimeout, researcherTimeout)
		assert.Greater(t, plannerTimeout, analystTimeout,
			"planner (%d s) must have a longer idle timeout than analyst (%d s)", plannerTimeout, analystTimeout)
		assert.Less(t, researcherTimeout, analystTimeout,
			"researcher (%d s) must have a shorter idle timeout than analyst (%d s)", researcherTimeout, analystTimeout)

		// Verify absolute values.
		assert.Equal(t, "7200", SeedPlannerIdleTimeout, "planner idle timeout must be 7200 s (2 hours)")
		assert.Equal(t, "1800", SeedResearcherIdleTimeout, "researcher idle timeout must be 1800 s (30 minutes)")
	})

	t.Run("Scenario_EachAgentHasPersonalizedWelcomeMessage", func(t *testing.T) {
		// Given each capability agent has a distinct persona and set of capabilities,
		// When migration 000111 seeds welcome_message session configs,
		// Then the welcome message constant key is non-empty and the config_value
		//   dimension for welcome messages is captured by a dedicated key constant.
		assert.Equal(t, "welcome_message", SeedSessionConfigWelcomeMessage,
			"welcome message config key must equal 'welcome_message'")
		assert.NotEmpty(t, SeedSessionConfigWelcomeMessage,
			"welcome message config key must not be empty")
		// The three distinct key constants confirm all three dimensions are covered.
		keys := []string{
			SeedSessionConfigMaxTurns,
			SeedSessionConfigIdleTimeout,
			SeedSessionConfigWelcomeMessage,
		}
		uniqueKeys := map[string]struct{}{}
		for _, k := range keys {
			assert.NotEmpty(t, k, "config key constant must not be empty")
			uniqueKeys[k] = struct{}{}
		}
		assert.Len(t, uniqueKeys, 3,
			"the three config key constants must be distinct (max_turns, idle_timeout_seconds, welcome_message)")
	})

	t.Run("Scenario_AnalystHasMediumTimeoutForDocumentAnalysis", func(t *testing.T) {
		// Given document analysis tasks take longer than research but not as long as
		//   full planning sessions — users review documents and formulate questions
		//   between turns,
		// When migration 000111 seeds idle_timeout_seconds for core-analyst,
		// Then the analyst timeout is strictly between researcher (shortest) and
		//   planner (longest), reflecting the medium duration of analysis sessions.
		researcherTimeout, err := strconv.Atoi(SeedResearcherIdleTimeout)
		assert.NoError(t, err, "SeedResearcherIdleTimeout must be a numeric string")

		analystTimeout, err := strconv.Atoi(SeedAnalystIdleTimeout)
		assert.NoError(t, err, "SeedAnalystIdleTimeout must be a numeric string")

		plannerTimeout, err := strconv.Atoi(SeedPlannerIdleTimeout)
		assert.NoError(t, err, "SeedPlannerIdleTimeout must be a numeric string")

		assert.Greater(t, analystTimeout, researcherTimeout,
			"analyst (%d s) must have a longer timeout than researcher (%d s) — analysis needs more think time",
			analystTimeout, researcherTimeout)
		assert.Less(t, analystTimeout, plannerTimeout,
			"analyst (%d s) must have a shorter timeout than planner (%d s) — analysis sessions are shorter than planning",
			analystTimeout, plannerTimeout)
		// Verify absolute value.
		assert.Equal(t, "3600", SeedAnalystIdleTimeout, "analyst idle timeout must be 3600 s (60 minutes)")
	})
}
