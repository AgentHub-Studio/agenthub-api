package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability tool permission seed constants (migration 000118).
// These run without a database and guard against accidental constant drift.

// ─────────────────────────────────────────────────────────
// Count constant tests
// ─────────────────────────────────────────────────────────

func TestSeedToolPermissionCount_IsTwelve(t *testing.T) {
	assert.Equal(t, 12, SeedToolPermissionCount,
		"migration 000118 seeds exactly 12 capability tool permission rows (four per capability agent)")
}

func TestSeedToolPermissionAgentCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedToolPermissionAgentCount,
		"SeedToolPermissionAgentCount must be 3 — researcher, analyst, planner")
}

func TestSeedToolPermissionCount_EqualsAgentCountTimesFour(t *testing.T) {
	assert.Equal(t, SeedToolPermissionAgentCount*4, SeedToolPermissionCount,
		"total permission count must equal agent count × 4 (each agent has exactly 4 tool permission rows)")
}

func TestSeedToolPermissionCount_IsPositive(t *testing.T) {
	assert.Greater(t, SeedToolPermissionCount, 0,
		"SeedToolPermissionCount must be positive")
}

func TestSeedToolPermissionAgentCount_IsPositive(t *testing.T) {
	assert.Greater(t, SeedToolPermissionAgentCount, 0,
		"SeedToolPermissionAgentCount must be positive")
}

// ─────────────────────────────────────────────────────────
// Permission mode constant tests
// ─────────────────────────────────────────────────────────

func TestSeedPermModeAllow_Value(t *testing.T) {
	assert.Equal(t, "allow", SeedPermModeAllow,
		"SeedPermModeAllow must equal \"allow\"")
}

func TestSeedPermModeDeny_Value(t *testing.T) {
	assert.Equal(t, "deny", SeedPermModeDeny,
		"SeedPermModeDeny must equal \"deny\"")
}

func TestSeedPermModeRequireApproval_Value(t *testing.T) {
	assert.Equal(t, "require_approval", SeedPermModeRequireApproval,
		"SeedPermModeRequireApproval must equal \"require_approval\"")
}

func TestSeedPermModes_AreDistinct(t *testing.T) {
	modes := []string{
		SeedPermModeAllow,
		SeedPermModeDeny,
		SeedPermModeRequireApproval,
	}
	seen := map[string]struct{}{}
	for _, m := range modes {
		assert.NotEmpty(t, m, "every permission mode constant must be non-empty")
		seen[m] = struct{}{}
	}
	assert.Len(t, seen, 3,
		"there must be exactly 3 distinct permission mode constants")
}

func TestSeedPermModes_AreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedPermModeAllow)
	assert.NotEmpty(t, SeedPermModeDeny)
	assert.NotEmpty(t, SeedPermModeRequireApproval)
}

func TestSeedPermModes_ValidSet(t *testing.T) {
	validModes := map[string]bool{
		"allow":            true,
		"deny":             true,
		"require_approval": true,
	}
	assert.True(t, validModes[SeedPermModeAllow],
		"SeedPermModeAllow must be in the valid permission mode set")
	assert.True(t, validModes[SeedPermModeDeny],
		"SeedPermModeDeny must be in the valid permission mode set")
	assert.True(t, validModes[SeedPermModeRequireApproval],
		"SeedPermModeRequireApproval must be in the valid permission mode set")
}

// ─────────────────────────────────────────────────────────
// Tool slug constant tests
// ─────────────────────────────────────────────────────────

func TestSeedToolSlugWebSearch_Value(t *testing.T) {
	assert.Equal(t, "core-web-search", SeedToolSlugWebSearch,
		"SeedToolSlugWebSearch must equal \"core-web-search\"")
}

func TestSeedToolSlugWebFetch_Value(t *testing.T) {
	assert.Equal(t, "core-web-fetch", SeedToolSlugWebFetch,
		"SeedToolSlugWebFetch must equal \"core-web-fetch\"")
}

func TestSeedToolSlugDocSearch_Value(t *testing.T) {
	assert.Equal(t, "core-doc-search", SeedToolSlugDocSearch,
		"SeedToolSlugDocSearch must equal \"core-doc-search\"")
}

func TestSeedToolSlugSubagentRun_Value(t *testing.T) {
	assert.Equal(t, "core-subagent-run", SeedToolSlugSubagentRun,
		"SeedToolSlugSubagentRun must equal \"core-subagent-run\"")
}

func TestSeedToolSlugs_AreDistinct(t *testing.T) {
	slugs := []string{
		SeedToolSlugWebSearch,
		SeedToolSlugWebFetch,
		SeedToolSlugDocSearch,
		SeedToolSlugSubagentRun,
	}
	seen := map[string]struct{}{}
	for _, s := range slugs {
		assert.NotEmpty(t, s, "every tool slug constant must be non-empty")
		seen[s] = struct{}{}
	}
	assert.Len(t, seen, 4,
		"there must be exactly 4 distinct tool slug constants")
}

func TestSeedToolSlugs_AreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedToolSlugWebSearch)
	assert.NotEmpty(t, SeedToolSlugWebFetch)
	assert.NotEmpty(t, SeedToolSlugDocSearch)
	assert.NotEmpty(t, SeedToolSlugSubagentRun)
}

func TestSeedToolSlugs_UseCorePrefix(t *testing.T) {
	slugs := []string{
		SeedToolSlugWebSearch,
		SeedToolSlugWebFetch,
		SeedToolSlugDocSearch,
		SeedToolSlugSubagentRun,
	}
	for _, s := range slugs {
		assert.Equal(t, "core-", s[:5],
			"every seed tool slug must start with the \"core-\" namespace prefix; got %q", s)
	}
}

func TestSeedToolSlugs_CountMatchesPermissionsPerAgent(t *testing.T) {
	slugs := []string{
		SeedToolSlugWebSearch,
		SeedToolSlugWebFetch,
		SeedToolSlugDocSearch,
		SeedToolSlugSubagentRun,
	}
	perAgent := SeedToolPermissionCount / SeedToolPermissionAgentCount
	assert.Equal(t, len(slugs), perAgent,
		"number of tool slug constants (%d) must equal permissions per agent (%d)", len(slugs), perAgent)
}

// ─────────────────────────────────────────────────────────
// Semantic subagent permission constant tests
// ─────────────────────────────────────────────────────────

func TestSeedResearcherSubagentPermission_IsRequireApproval(t *testing.T) {
	assert.Equal(t, SeedPermModeRequireApproval, SeedResearcherSubagentPermission,
		"core-researcher subagent permission must be require_approval")
}

func TestSeedAnalystSubagentPermission_IsDeny(t *testing.T) {
	assert.Equal(t, SeedPermModeDeny, SeedAnalystSubagentPermission,
		"core-analyst subagent permission must be deny")
}

func TestSeedPlannerSubagentPermission_IsAllow(t *testing.T) {
	assert.Equal(t, SeedPermModeAllow, SeedPlannerSubagentPermission,
		"core-planner subagent permission must be allow")
}

func TestSeedSubagentPermissions_AreDistinct(t *testing.T) {
	perms := []string{
		SeedResearcherSubagentPermission,
		SeedAnalystSubagentPermission,
		SeedPlannerSubagentPermission,
	}
	seen := map[string]struct{}{}
	for _, p := range perms {
		assert.NotEmpty(t, p, "every subagent permission constant must be non-empty")
		seen[p] = struct{}{}
	}
	assert.Len(t, seen, 3,
		"each capability agent must have a distinct subagent permission mode")
}

func TestSeedSubagentPermissions_AreValidModes(t *testing.T) {
	validModes := map[string]bool{
		SeedPermModeAllow:           true,
		SeedPermModeDeny:            true,
		SeedPermModeRequireApproval: true,
	}
	assert.True(t, validModes[SeedResearcherSubagentPermission],
		"SeedResearcherSubagentPermission must be a valid permission mode")
	assert.True(t, validModes[SeedAnalystSubagentPermission],
		"SeedAnalystSubagentPermission must be a valid permission mode")
	assert.True(t, validModes[SeedPlannerSubagentPermission],
		"SeedPlannerSubagentPermission must be a valid permission mode")
}

// ─────────────────────────────────────────────────────────
// Researcher tool permission tests
// ─────────────────────────────────────────────────────────

func TestSeedResearcher_AllowsWebSearch(t *testing.T) {
	// core-researcher primary capability is web search — must be allowed.
	assert.Equal(t, SeedPermModeAllow, SeedPermModeAllow,
		"core-researcher allows core-web-search (primary capability)")
	assert.Equal(t, "core-web-search", SeedToolSlugWebSearch,
		"web search tool slug must match the seeded value")
}

func TestSeedResearcher_AllowsWebFetch(t *testing.T) {
	// core-researcher secondary capability is web fetch — must be allowed.
	assert.Equal(t, "allow", SeedPermModeAllow,
		"core-researcher allows core-web-fetch (secondary capability)")
	assert.Equal(t, "core-web-fetch", SeedToolSlugWebFetch,
		"web fetch tool slug must match the seeded value")
}

func TestSeedResearcher_AllowsDocSearch(t *testing.T) {
	// core-researcher fallback is doc search — must be allowed.
	assert.Equal(t, "allow", SeedPermModeAllow,
		"core-researcher allows core-doc-search (fallback capability)")
	assert.Equal(t, "core-doc-search", SeedToolSlugDocSearch,
		"doc search tool slug must match the seeded value")
}

func TestSeedResearcher_RequiresApprovalForSubagent(t *testing.T) {
	// core-researcher subagent delegation requires approval.
	assert.Equal(t, SeedPermModeRequireApproval, SeedResearcherSubagentPermission,
		"core-researcher must require_approval for core-subagent-run")
	assert.NotEqual(t, SeedPermModeAllow, SeedResearcherSubagentPermission,
		"core-researcher must NOT freely allow subagent delegation")
	assert.NotEqual(t, SeedPermModeDeny, SeedResearcherSubagentPermission,
		"core-researcher must NOT deny subagent delegation entirely")
}

// ─────────────────────────────────────────────────────────
// Analyst tool permission tests
// ─────────────────────────────────────────────────────────

func TestSeedAnalyst_DeniesSubagentRun(t *testing.T) {
	// core-analyst must not delegate — single-thread analysis focus.
	assert.Equal(t, SeedPermModeDeny, SeedAnalystSubagentPermission,
		"core-analyst subagent permission must be deny — analyst maintains single-thread focus")
	assert.NotEqual(t, SeedPermModeAllow, SeedAnalystSubagentPermission,
		"core-analyst must NOT allow subagent delegation")
	assert.NotEqual(t, SeedPermModeRequireApproval, SeedAnalystSubagentPermission,
		"core-analyst must NOT require_approval for subagent delegation — it is denied entirely")
}

// ─────────────────────────────────────────────────────────
// Planner tool permission tests
// ─────────────────────────────────────────────────────────

func TestSeedPlanner_AllowsSubagentRun(t *testing.T) {
	// core-planner primary role is delegation — must freely allow subagent runs.
	assert.Equal(t, SeedPermModeAllow, SeedPlannerSubagentPermission,
		"core-planner subagent permission must be allow — delegation is planner's primary role")
	assert.NotEqual(t, SeedPermModeDeny, SeedPlannerSubagentPermission,
		"core-planner must NOT deny subagent delegation")
	assert.NotEqual(t, SeedPermModeRequireApproval, SeedPlannerSubagentPermission,
		"core-planner must NOT require_approval for subagent delegation")
}
