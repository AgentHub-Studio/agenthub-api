package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD-style scenarios that ratify the §8.1 BuiltinSubagentTypeRegistry against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1), Section 8.1 ("The Agent Tool and Delegation Criteria").
//
// §8.1 states: "Claude Code provides up to six built-in subagent types,
// depending on feature flags and entrypoint" and enumerates the six types
// with their toolset restrictions, permission model, and routing behaviour.
//
// These BDD scenarios ratify the four most consequential structural claims:
//  A. Explore is the only read-only type (write+edit deny-list).
//  B. General-purpose is the only type that may route through the fork path.
//  C. Claude Code Guide is the only type with a permissionMode override.
//  D. Exactly two types are feature-gated (Guide + Statusline-setup).
//  E. All six types carry distinct use-case categories.
//  F. The registry is exhaustive: SeedBuiltinSubagentTypeSlugs round-trips.
//  G. Toolset categories partition the six types without overlap.
//  H. Non-read-only types do not carry write tool denials.

func TestFEAT035_BDD_BuiltinSubagentTypeRegistry(t *testing.T) {
	t.Run("Scenario_A_ExploreIsTheOnlyReadOnlyType", func(t *testing.T) {
		// Given the §8.1 registry of six built-in subagent types,
		r := NewBuiltinSubagentTypeRegistry()

		// When we ask which types restrict write and edit tools,
		writeDenied := r.TypesWithWriteToolsDenied()

		// Then exactly one type carries the deny-list restriction and
		//      it is Explore — PDF §8.1: "Explore: primarily read/search-oriented
		//      investigation, with write and edit tools in its deny-list."
		require.Len(t, writeDenied, 1,
			"exactly one §8.1 type has write+edit tools in the deny-list")
		assert.Equal(t, BuiltinTypeExplore, writeDenied[0].Slug,
			"the write-denied type must be Explore")
		assert.Equal(t, "read_only", writeDenied[0].ToolsetCategory,
			"Explore's toolset category must be read_only")
	})

	t.Run("Scenario_B_GeneralPurposeIsTheOnlyForkRoutingType", func(t *testing.T) {
		// Given the §8.1 registry,
		r := NewBuiltinSubagentTypeRegistry()

		// When we ask which types may route to the fork-subagent path,
		forkTypes := r.TypesThatMayFork()

		// Then exactly one type carries the fork-routing caveat and
		//      it is General-purpose — PDF §8.1: "General-purpose: broadly capable,
		//      used when explicitly requested (note: omitting the type may route to
		//      the fork-subagent path instead)."
		require.Len(t, forkTypes, 1,
			"exactly one §8.1 type may route to the fork-subagent path")
		assert.Equal(t, BuiltinTypeGeneralPurpose, forkTypes[0].Slug,
			"the fork-routing type must be General-purpose")
		assert.Equal(t, "full", forkTypes[0].ToolsetCategory,
			"General-purpose has full toolset access")
	})

	t.Run("Scenario_C_ClaudeCodeGuideIsTheOnlyPermissionOverrideType", func(t *testing.T) {
		// Given the §8.1 registry,
		r := NewBuiltinSubagentTypeRegistry()

		// When we ask which types carry a permissionMode override,
		overrideTypes := r.TypesWithPermissionModeOverride()

		// Then exactly one type carries an override and it is Claude Code Guide —
		//      PDF §8.1: "Claude Code Guide: onboarding and documentation assistance,
		//      with its own permissionMode override."
		require.Len(t, overrideTypes, 1,
			"exactly one §8.1 type has a permissionMode override")
		assert.Equal(t, BuiltinTypeClaudeCodeGuide, overrideTypes[0].Slug,
			"the permission-override type must be Claude Code Guide")
		assert.NotEmpty(t, overrideTypes[0].PermissionModelOverride,
			"the permissionMode override field must be non-empty")
	})

	t.Run("Scenario_D_ExactlyTwoTypesAreFeatureGated", func(t *testing.T) {
		// Given the §8.1 registry,
		r := NewBuiltinSubagentTypeRegistry()

		// When we ask which types depend on feature flags or entrypoint,
		gated := r.FeatureGatedTypes()
		gatedSlugs := make(map[BuiltinSubagentTypeSlug]bool, len(gated))
		for _, g := range gated {
			gatedSlugs[g.Slug] = true
		}

		// Then exactly two types are feature-gated — PDF §8.1: "Claude Code
		//      provides up to six built-in subagent types, depending on feature
		//      flags and entrypoint" implies some are conditional.
		assert.Len(t, gated, 2,
			"exactly two §8.1 types must be feature-gated")
		assert.True(t, gatedSlugs[BuiltinTypeClaudeCodeGuide],
			"Claude Code Guide must be feature-gated")
		assert.True(t, gatedSlugs[BuiltinTypeStatuslineSetup],
			"Statusline-setup must be feature-gated")
	})

	t.Run("Scenario_E_AllSixTypesHaveDistinctUseCaseCategories", func(t *testing.T) {
		// Given the §8.1 registry,
		r := NewBuiltinSubagentTypeRegistry()

		// When we collect the use-case categories of all six types,
		seen := make(map[string]BuiltinSubagentTypeSlug)
		for _, p := range r.AllBuiltinSubagentTypes() {
			// Then no two types share a use-case category — each §8.1 type
			//      has a distinct operational purpose.
			if prev, exists := seen[p.UseCaseCategory]; exists {
				t.Errorf("use-case category %q shared by %q and %q — must be distinct",
					p.UseCaseCategory, prev, p.Slug)
			}
			seen[p.UseCaseCategory] = p.Slug
		}
		assert.Len(t, seen, 6, "six types must produce exactly six distinct use-case categories")
	})

	t.Run("Scenario_F_RegistryIsExhaustiveViaSlugRoundTrip", func(t *testing.T) {
		// Given the SeedBuiltinSubagentTypeSlugs sentinel slice,
		r := NewBuiltinSubagentTypeRegistry()

		// When we look up every seed slug in the registry,
		// Then all lookups succeed — the registry is exhaustive and the
		//      slug slice and the internal map agree.
		for _, slug := range SeedBuiltinSubagentTypeSlugs {
			p, ok := r.FindBuiltinSubagentTypeBySlug(slug)
			assert.True(t, ok,
				"seed slug %q must resolve in the registry", slug)
			assert.Equal(t, slug, p.Slug,
				"resolved profile must carry the queried slug")
		}
		assert.Len(t, SeedBuiltinSubagentTypeSlugs, SeedBuiltinSubagentTypeCount,
			"slug slice length must equal SeedBuiltinSubagentTypeCount")
	})

	t.Run("Scenario_G_ToolsetCategoriesAreDistinctAcrossAllTypes", func(t *testing.T) {
		// Given the §8.1 registry,
		r := NewBuiltinSubagentTypeRegistry()

		// When we collect the toolset categories of all six types,
		seen := make(map[string]BuiltinSubagentTypeSlug)
		for _, p := range r.AllBuiltinSubagentTypes() {
			// Then no two types share a toolset category — each §8.1 type
			//      operates with a distinct toolset profile, ensuring that
			//      the harness can select the right toolset by type.
			if prev, exists := seen[p.ToolsetCategory]; exists {
				t.Errorf("toolset category %q shared by %q and %q — must be distinct",
					p.ToolsetCategory, prev, p.Slug)
			}
			seen[p.ToolsetCategory] = p.Slug
		}
		assert.Len(t, seen, 6, "six types must produce exactly six distinct toolset categories")
	})

	t.Run("Scenario_H_NonReadOnlyTypesDontDenyWriteTools", func(t *testing.T) {
		// Given the §8.1 registry,
		r := NewBuiltinSubagentTypeRegistry()

		// When we inspect every type that is NOT Explore,
		for _, p := range r.AllBuiltinSubagentTypes() {
			if p.Slug == BuiltinTypeExplore {
				continue
			}
			// Then none of them carry the write-tools denial flag —
			//      PDF §8.1 exclusively attributes the deny-list to Explore.
			assert.False(t, p.HasWriteToolsDenied,
				"type %q must not have write tools denied — only Explore has this restriction",
				p.Slug)
		}
	})
}
