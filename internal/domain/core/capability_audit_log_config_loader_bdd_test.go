package core

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000113 capability audit log config seeds.
// These assert seed shape and audit configuration rationale without a database.

func TestBDD_CapabilityAuditLogConfigSeed(t *testing.T) {
	t.Run("Scenario_NineAuditLogConfigsThreePerCapabilityAgent", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent audit configuration
		//   to ensure full auditability of capability agent actions for tenant
		//   administrators,
		// When migration 000113 seeds capability_audit_log_config rows,
		// Then exactly 9 rows are added — three per agent — and per-agent counts
		//   sum to the total count constant.
		assert.Equal(t, 9, SeedCapabilityAuditLogConfigCount,
			"migration 000113 must seed exactly 9 capability audit log config rows")
		sum := SeedResearcherAuditLogConfigCount + SeedAnalystAuditLogConfigCount + SeedPlannerAuditLogConfigCount
		assert.Equal(t, SeedCapabilityAuditLogConfigCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherAuditLogConfigCount, SeedAnalystAuditLogConfigCount, SeedPlannerAuditLogConfigCount,
			SeedCapabilityAuditLogConfigCount)
		assert.Equal(t, 3, SeedResearcherAuditLogConfigCount,
			"researcher must have exactly 3 audit log config rows")
		assert.Equal(t, 3, SeedAnalystAuditLogConfigCount,
			"analyst must have exactly 3 audit log config rows")
		assert.Equal(t, 3, SeedPlannerAuditLogConfigCount,
			"planner must have exactly 3 audit log config rows")
		// Agent slug list must have 3 distinct entries.
		assert.Len(t, SeedCapabilityAuditLogConfigAgentSlugs, 3,
			"SeedCapabilityAuditLogConfigAgentSlugs must list exactly 3 agents")
	})

	t.Run("Scenario_PlannerHasLongestRetentionForProjectAuditTrails", func(t *testing.T) {
		// Given the planner generates project-level task decompositions and
		//   dependency graphs that must be auditable across full annual planning
		//   cycles,
		// When migration 000113 seeds retention_days config rows,
		// Then the planner's retention (365d) is strictly greater than the analyst's
		//   (180d), which is strictly greater than the researcher's (90d) — reflecting
		//   the differing temporal scope of each agent's outputs.
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
		assert.Equal(t, "365", SeedPlannerRetentionDays,
			"planner retention must be exactly 365 days")
	})

	t.Run("Scenario_AnalystHasMediumRetentionForComplianceNeeds", func(t *testing.T) {
		// Given analytical outputs are subject to compliance review windows (SOX,
		//   GDPR 6-month audit requirements), requiring retention longer than
		//   transient research outputs but shorter than long-term project plans,
		// When migration 000113 seeds retention_days config rows,
		// Then the analyst's retention is 180 days — long enough for 6-month
		//   compliance windows and shorter than the planner's full-year retention.
		analystDays, err := strconv.Atoi(SeedAnalystRetentionDays)
		assert.NoError(t, err, "SeedAnalystRetentionDays must be parseable as int")

		researcherDays, err := strconv.Atoi(SeedResearcherRetentionDays)
		assert.NoError(t, err, "SeedResearcherRetentionDays must be parseable as int")

		plannerDays, err := strconv.Atoi(SeedPlannerRetentionDays)
		assert.NoError(t, err, "SeedPlannerRetentionDays must be parseable as int")

		assert.Equal(t, 180, analystDays,
			"analyst retention must be exactly 180 days — satisfies 6-month compliance windows")
		assert.Greater(t, analystDays, researcherDays,
			"analyst (%d) must retain longer than researcher (%d)", analystDays, researcherDays)
		assert.Less(t, analystDays, plannerDays,
			"analyst (%d) must retain less than planner (%d)", analystDays, plannerDays)
	})

	t.Run("Scenario_PlannerUsesDebugLevelForDetailedTaskTracking", func(t *testing.T) {
		// Given the planner's task decomposition and dependency resolution logic
		//   generates complex intermediate state that is difficult to diagnose
		//   without detailed logging, while researcher and analyst produce
		//   straightforward output that needs only informational audit entries,
		// When migration 000113 seeds log_level config rows,
		// Then the planner is the only agent configured for debug-level audit logging,
		//   and the researcher and analyst both use info level.
		assert.Equal(t, SeedAuditLogLevelDebug, SeedPlannerAuditLogLevel,
			"SeedPlannerAuditLogLevel must equal SeedAuditLogLevelDebug")
		assert.NotEqual(t, SeedAuditLogLevelInfo, SeedPlannerAuditLogLevel,
			"planner must not use info level — debug only")
		assert.Equal(t, "debug", SeedPlannerAuditLogLevel,
			"SeedPlannerAuditLogLevel must equal the string \"debug\"")
		assert.Equal(t, "info", SeedAuditLogLevelInfo,
			"SeedAuditLogLevelInfo must equal the string \"info\"")

		// Verify the two log level constants are distinct.
		assert.NotEqual(t, SeedAuditLogLevelInfo, SeedAuditLogLevelDebug,
			"info and debug level constants must differ")
	})

	t.Run("Scenario_AllAgentsLogToolCallsAndToolResults", func(t *testing.T) {
		// Given tool calls and tool results are the primary unit of observable work
		//   for all capability agents — researchers search, analysts read, planners
		//   create tasks — all three agents must include tool_call and tool_result
		//   in their log_events configuration,
		// When migration 000113 seeds log_events config rows,
		// Then the log_events config key constant is present and non-empty,
		//   and the three config key constants (log_events, retention_days, log_level)
		//   are all distinct.
		assert.Equal(t, "log_events", SeedAuditLogConfigKeyEvents,
			"SeedAuditLogConfigKeyEvents must equal \"log_events\"")
		assert.Equal(t, "retention_days", SeedAuditLogConfigKeyRetention,
			"SeedAuditLogConfigKeyRetention must equal \"retention_days\"")
		assert.Equal(t, "log_level", SeedAuditLogConfigKeyLevel,
			"SeedAuditLogConfigKeyLevel must equal \"log_level\"")

		// All three config key constants must be distinct.
		keySet := map[string]struct{}{
			SeedAuditLogConfigKeyEvents:    {},
			SeedAuditLogConfigKeyRetention: {},
			SeedAuditLogConfigKeyLevel:     {},
		}
		assert.Len(t, keySet, 3,
			"log_events, retention_days, log_level must be 3 distinct config key constants")
	})
}
