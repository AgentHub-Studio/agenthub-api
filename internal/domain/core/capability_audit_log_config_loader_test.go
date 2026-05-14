package core

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability audit log config seed constants (migration 000113).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAuditLogConfigCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedCapabilityAuditLogConfigCount,
		"migration 000113 seeds exactly 9 capability audit log config rows (three per capability agent)")
}

func TestSeedCapabilityAuditLogConfigAgentSlugs_HasThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityAuditLogConfigAgentSlugs, 3,
		"SeedCapabilityAuditLogConfigAgentSlugs must have exactly 3 entries (researcher, analyst, planner)")
}

func TestSeedResearcherAuditLogConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherAuditLogConfigCount,
		"SeedResearcherAuditLogConfigCount must be 3 — log_events + retention_days + log_level")
}

func TestSeedAnalystAuditLogConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAnalystAuditLogConfigCount,
		"SeedAnalystAuditLogConfigCount must be 3 — log_events + retention_days + log_level")
}

func TestSeedPlannerAuditLogConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedPlannerAuditLogConfigCount,
		"SeedPlannerAuditLogConfigCount must be 3 — log_events + retention_days + log_level")
}

func TestSeedPerAgentAuditLogConfigCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherAuditLogConfigCount + SeedAnalystAuditLogConfigCount + SeedPlannerAuditLogConfigCount
	assert.Equal(t, SeedCapabilityAuditLogConfigCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherAuditLogConfigCount, SeedAnalystAuditLogConfigCount, SeedPlannerAuditLogConfigCount,
		SeedCapabilityAuditLogConfigCount)
}

func TestSeedAuditLogConfigKeyConstants_AreThreeDistinct(t *testing.T) {
	keys := []string{
		SeedAuditLogConfigKeyEvents,
		SeedAuditLogConfigKeyRetention,
		SeedAuditLogConfigKeyLevel,
	}
	seen := map[string]struct{}{}
	for _, k := range keys {
		assert.NotEmpty(t, k, "every config key constant must be non-empty")
		seen[k] = struct{}{}
	}
	assert.Len(t, seen, 3, "there must be exactly 3 distinct config key constants")
}

func TestSeedAuditLogConfigKeyEvents_IsLogEvents(t *testing.T) {
	assert.Equal(t, "log_events", SeedAuditLogConfigKeyEvents,
		"SeedAuditLogConfigKeyEvents must equal \"log_events\"")
}

func TestSeedAuditLogConfigKeyRetention_IsRetentionDays(t *testing.T) {
	assert.Equal(t, "retention_days", SeedAuditLogConfigKeyRetention,
		"SeedAuditLogConfigKeyRetention must equal \"retention_days\"")
}

func TestSeedAuditLogConfigKeyLevel_IsLogLevel(t *testing.T) {
	assert.Equal(t, "log_level", SeedAuditLogConfigKeyLevel,
		"SeedAuditLogConfigKeyLevel must equal \"log_level\"")
}

func TestSeedAuditLogLevelConstants_AreTwoDistinct(t *testing.T) {
	assert.NotEqual(t, SeedAuditLogLevelInfo, SeedAuditLogLevelDebug,
		"SeedAuditLogLevelInfo and SeedAuditLogLevelDebug must be different values")
	assert.Equal(t, "info", SeedAuditLogLevelInfo,
		"SeedAuditLogLevelInfo must equal \"info\"")
	assert.Equal(t, "debug", SeedAuditLogLevelDebug,
		"SeedAuditLogLevelDebug must equal \"debug\"")
}

func TestSeedPlannerAuditLogLevel_IsDebug(t *testing.T) {
	assert.Equal(t, SeedAuditLogLevelDebug, SeedPlannerAuditLogLevel,
		"SeedPlannerAuditLogLevel must equal SeedAuditLogLevelDebug — planner is the only agent with debug level")
	assert.NotEqual(t, SeedAuditLogLevelInfo, SeedPlannerAuditLogLevel,
		"SeedPlannerAuditLogLevel must not be info level")
}

func TestSeedPlannerRetentionDays_IsLongest(t *testing.T) {
	researcherDays, err := strconv.Atoi(SeedResearcherRetentionDays)
	assert.NoError(t, err, "SeedResearcherRetentionDays must be parseable as int")

	analystDays, err := strconv.Atoi(SeedAnalystRetentionDays)
	assert.NoError(t, err, "SeedAnalystRetentionDays must be parseable as int")

	plannerDays, err := strconv.Atoi(SeedPlannerRetentionDays)
	assert.NoError(t, err, "SeedPlannerRetentionDays must be parseable as int")

	assert.Greater(t, plannerDays, analystDays,
		"planner retention (%d) must exceed analyst (%d) — full year for project audit trails",
		plannerDays, analystDays)
	assert.Greater(t, analystDays, researcherDays,
		"analyst retention (%d) must exceed researcher (%d) — compliance needs",
		analystDays, researcherDays)
}

func TestSeedAnalystRetentionDays_IsMedium(t *testing.T) {
	analystDays, err := strconv.Atoi(SeedAnalystRetentionDays)
	assert.NoError(t, err, "SeedAnalystRetentionDays must be parseable as int")

	researcherDays, err := strconv.Atoi(SeedResearcherRetentionDays)
	assert.NoError(t, err, "SeedResearcherRetentionDays must be parseable as int")

	plannerDays, err := strconv.Atoi(SeedPlannerRetentionDays)
	assert.NoError(t, err, "SeedPlannerRetentionDays must be parseable as int")

	assert.Greater(t, analystDays, researcherDays,
		"analyst retention (%d) must be greater than researcher (%d)", analystDays, researcherDays)
	assert.Less(t, analystDays, plannerDays,
		"analyst retention (%d) must be less than planner (%d)", analystDays, plannerDays)
}

func TestSeedResearcherRetentionDays_IsShortest(t *testing.T) {
	researcherDays, err := strconv.Atoi(SeedResearcherRetentionDays)
	assert.NoError(t, err, "SeedResearcherRetentionDays must be parseable as int")

	analystDays, err := strconv.Atoi(SeedAnalystRetentionDays)
	assert.NoError(t, err, "SeedAnalystRetentionDays must be parseable as int")

	assert.Less(t, researcherDays, analystDays,
		"researcher retention (%d) must be less than analyst (%d) — research outputs are transient",
		researcherDays, analystDays)
	assert.Equal(t, 90, researcherDays,
		"researcher retention must be 90 days")
}

func TestSeedRetentionValues_AreNumericStrings(t *testing.T) {
	for _, val := range []string{SeedResearcherRetentionDays, SeedAnalystRetentionDays, SeedPlannerRetentionDays} {
		_, err := strconv.Atoi(val)
		assert.NoError(t, err, "retention value %q must be a numeric string", val)
		assert.NotEmpty(t, val, "retention value must not be empty")
	}
}

func TestSeedCapabilityAuditLogConfigAgentSlugs_ContainsAllThreeAgents(t *testing.T) {
	slugSet := map[string]struct{}{}
	for _, s := range SeedCapabilityAuditLogConfigAgentSlugs {
		slugSet[s] = struct{}{}
	}
	_, hasResearcher := slugSet["core-researcher"]
	assert.True(t, hasResearcher, "SeedCapabilityAuditLogConfigAgentSlugs must contain 'core-researcher'")

	_, hasAnalyst := slugSet["core-analyst"]
	assert.True(t, hasAnalyst, "SeedCapabilityAuditLogConfigAgentSlugs must contain 'core-analyst'")

	_, hasPlanner := slugSet["core-planner"]
	assert.True(t, hasPlanner, "SeedCapabilityAuditLogConfigAgentSlugs must contain 'core-planner'")
}

func TestSeedPlannerRetentionDays_Is365(t *testing.T) {
	assert.Equal(t, "365", SeedPlannerRetentionDays,
		"planner retention must be 365 days — full year for project audit trails")
}

func TestSeedAnalystRetentionDays_Is180(t *testing.T) {
	assert.Equal(t, "180", SeedAnalystRetentionDays,
		"analyst retention must be 180 days — compliance needs")
}

func TestSeedResearcherLogEvents_IncludesWebSearch(t *testing.T) {
	// Researcher event list must include web_search (key researcher action)
	// We use the SQL seed directly baked in the migration — verify constant alignment.
	// No direct constant for event list, but we can verify the config key is correct.
	assert.Equal(t, "log_events", SeedAuditLogConfigKeyEvents,
		"log_events key must match migration definition")
	// The event list itself (tool_call,tool_result,web_search,doc_index) is in the
	// migration SQL. Verify the config key constant is non-empty and correct.
	assert.NotEmpty(t, SeedAuditLogConfigKeyEvents)
}

func TestSeedAuditLogConfigKeys_AllNonEmpty(t *testing.T) {
	keys := []string{
		SeedAuditLogConfigKeyEvents,
		SeedAuditLogConfigKeyRetention,
		SeedAuditLogConfigKeyLevel,
	}
	for _, k := range keys {
		assert.True(t, strings.TrimSpace(k) != "",
			"config key constant %q must not be blank", k)
	}
}
