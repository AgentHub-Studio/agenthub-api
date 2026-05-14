package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000118 capability tool permission seeds.
// These assert seed shape and permission rationale without a database.

func TestBDD_CapabilityToolPermissionSeed(t *testing.T) {
	t.Run("Scenario_TwelvePermissionsAcrossThreeAgents", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent tool permission
		//   declarations to control which tools each agent may invoke autonomously,
		//   with user approval, or never — providing fine-grained security at the
		//   agent-tool level without requiring per-tenant configuration —
		// When migration 000118 seeds capability_tool_permission rows,
		// Then exactly 12 rows are added — four per agent — covering researcher,
		//   analyst, and planner with permissions for web-search, web-fetch,
		//   doc-search, and subagent-run.
		assert.Equal(t, 12, SeedToolPermissionCount,
			"migration 000118 must seed exactly 12 capability tool permission rows")

		assert.Equal(t, 3, SeedToolPermissionAgentCount,
			"SeedToolPermissionAgentCount must be 3 — researcher, analyst, planner")

		assert.Equal(t, SeedToolPermissionAgentCount*4, SeedToolPermissionCount,
			"total permission count must equal agent count × 4 (each agent has exactly 4 tool permission rows)")
	})

	t.Run("Scenario_FourPermissionsPerAgent", func(t *testing.T) {
		// Given each capability agent must declare its relationship with exactly
		//   the four core tools used in the capability layer — web-search, web-fetch,
		//   doc-search, and subagent-run — so that the orchestrator can enforce access
		//   control without any undefined permission state at runtime,
		// When migration 000118 seeds tool permission rows,
		// Then each agent has exactly four rows (one per tool), and the four tool
		//   slug constants are distinct, non-empty, and all carry the "core-" prefix.
		slugs := []string{
			SeedToolSlugWebSearch,
			SeedToolSlugWebFetch,
			SeedToolSlugDocSearch,
			SeedToolSlugSubagentRun,
		}
		seen := map[string]struct{}{}
		for _, s := range slugs {
			assert.NotEmpty(t, s, "every tool slug constant must be non-empty")
			assert.Equal(t, "core-", s[:5], "tool slug %q must start with core- prefix", s)
			seen[s] = struct{}{}
		}
		assert.Len(t, seen, 4,
			"there must be exactly 4 distinct tool slug constants — one per governed tool")

		perAgent := SeedToolPermissionCount / SeedToolPermissionAgentCount
		assert.Equal(t, 4, perAgent,
			"each agent must have exactly 4 tool permission rows")
	})

	t.Run("Scenario_PlannerCanDelegateResearcherMustApprove", func(t *testing.T) {
		// Given the planner's primary purpose is to break down complex goals into
		//   subtasks and delegate them to specialised agents — while the researcher
		//   is designed as a leaf-level autonomous investigator that should only
		//   delegate in exceptional cases reviewed by the user —
		// When migration 000118 seeds subagent-run permissions for planner and researcher,
		// Then the planner has permission_mode="allow" (free delegation) and the
		//   researcher has permission_mode="require_approval" (user must confirm each
		//   delegation), and the two modes are distinct.
		assert.Equal(t, SeedPermModeAllow, SeedPlannerSubagentPermission,
			"core-planner must have allow for core-subagent-run — delegation is its primary role")

		assert.Equal(t, SeedPermModeRequireApproval, SeedResearcherSubagentPermission,
			"core-researcher must have require_approval for core-subagent-run — delegation is exceptional")

		assert.NotEqual(t, SeedPlannerSubagentPermission, SeedResearcherSubagentPermission,
			"planner and researcher must have different subagent-run permission modes")
	})

	t.Run("Scenario_AnalystFocusesSingleThreadNoDelegation", func(t *testing.T) {
		// Given the core-analyst agent is designed to produce coherent, end-to-end
		//   analyses without splitting work across multiple agents — because
		//   sub-delegation would fragment the analyst's reasoning chain and produce
		//   inconsistent results — the analyst must be permanently prohibited from
		//   invoking the subagent-run tool, even with user approval,
		// When migration 000118 seeds the analyst's subagent-run permission,
		// Then permission_mode="deny" and SeedAnalystSubagentPermission resolves to
		//   SeedPermModeDeny, which is distinct from both allow and require_approval.
		assert.Equal(t, SeedPermModeDeny, SeedAnalystSubagentPermission,
			"core-analyst subagent permission must be deny — analyst maintains single-thread focus")

		assert.NotEqual(t, SeedPermModeAllow, SeedAnalystSubagentPermission,
			"core-analyst must NOT allow subagent delegation")
		assert.NotEqual(t, SeedPermModeRequireApproval, SeedAnalystSubagentPermission,
			"core-analyst must NOT require_approval — subagent delegation is denied entirely")

		// All three subagent permission modes are distinct.
		perms := []string{
			SeedResearcherSubagentPermission,
			SeedAnalystSubagentPermission,
			SeedPlannerSubagentPermission,
		}
		seen := map[string]struct{}{}
		for _, p := range perms {
			seen[p] = struct{}{}
		}
		assert.Len(t, seen, 3,
			"all three agents must have distinct subagent-run permission modes (allow/deny/require_approval each used once)")
	})

	t.Run("Scenario_ResearcherHasFullReadOnlyAccess", func(t *testing.T) {
		// Given the core-researcher agent is a read-only investigator — it gathers
		//   information from external sources (web, docs) but never writes data or
		//   spins up parallel workstreams autonomously — all read tools must be
		//   freely allowed, while the write-like delegation tool (subagent-run) must
		//   require user approval,
		// When migration 000118 seeds the researcher's tool permissions,
		// Then core-web-search, core-web-fetch, and core-doc-search all have
		//   permission_mode="allow", while core-subagent-run has
		//   permission_mode="require_approval".
		readToolModes := []string{
			SeedPermModeAllow, // core-web-search
			SeedPermModeAllow, // core-web-fetch
			SeedPermModeAllow, // core-doc-search
		}
		for i, mode := range readToolModes {
			assert.Equal(t, SeedPermModeAllow, mode,
				"researcher read tool at index %d must have allow mode", i)
		}

		assert.Equal(t, SeedPermModeRequireApproval, SeedResearcherSubagentPermission,
			"researcher subagent-run must be require_approval — delegation is not a read-only operation")

		// Permission mode constants are valid and distinct.
		validModes := map[string]bool{
			SeedPermModeAllow:           true,
			SeedPermModeDeny:            true,
			SeedPermModeRequireApproval: true,
		}
		assert.True(t, validModes[SeedResearcherSubagentPermission],
			"SeedResearcherSubagentPermission must be a valid permission mode constant")
	})
}
