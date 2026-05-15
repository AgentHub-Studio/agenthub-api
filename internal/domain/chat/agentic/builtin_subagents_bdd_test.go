package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_BuiltinSubagents(t *testing.T) {
	t.Run("Scenario_FreshTenantSpawnsResearcherWithoutCustomConfig", func(t *testing.T) {
		// Given a fresh tenant wants to spawn a researcher without
		// designing the subagent from scratch,
		// When the platform's builtin researcher is registered,
		// Then Lookup returns its descriptor with default toolset +
		// inheritance + summary policy references — the runner has
		// everything it needs.
		r := NewBuiltinSubagentRegistry()
		d := BuiltinSubagentDescriptor{
			Role: BuiltinSubagentResearcher, Slug: "research-baseline",
			Name: "Research Baseline",
			Description: "Investigates topics using read-only tools.",
			SystemPromptTemplate: "You are a thorough researcher...",
			DefaultToolsetPolicySlug: "documentation-readonly-allowlist",
			DefaultInheritanceModeSlug: "extend-parent-rights",
			DefaultSummaryShapeSlug: "success-with-artifacts",
		}
		require.NoError(t, r.Register(d))
		got, ok := r.Lookup("research-baseline")
		require.True(t, ok)
		assert.NotEmpty(t, got.SystemPromptTemplate)
		assert.NotEmpty(t, got.DefaultToolsetPolicySlug)
	})

	t.Run("Scenario_SevenCuratedRolesCoverCommonAgenticPatterns", func(t *testing.T) {
		// Given PDF §8.2 lists curated roles (researcher/coder/reviewer/
		// explorer/planner/curator/documenter),
		// When AllBuiltinSubagentRoles is queried,
		// Then 7 roles are returned — the full curated set.
		roles := AllBuiltinSubagentRoles()
		assert.Equal(t, 7, len(roles))
	})

	t.Run("Scenario_BuiltinReferencesSUB005ToolsetPolicy", func(t *testing.T) {
		// Given SUB-005 catalog has documentation-readonly-allowlist,
		// And a builtin researcher references it as default,
		// When the runner spawns the researcher,
		// Then it can resolve the toolset policy by slug — loose
		// coupling between SUB-002 and SUB-005 catalogs.
		d := BuiltinSubagentDescriptor{
			Role: BuiltinSubagentResearcher, Slug: "researcher",
			Name: "x", Description: "x", SystemPromptTemplate: "x",
			DefaultToolsetPolicySlug:   "documentation-readonly-allowlist",
			DefaultInheritanceModeSlug: "extend-parent-rights",
			DefaultSummaryShapeSlug:    "success-with-artifacts",
		}
		assert.NoError(t, d.Validate())
	})

	t.Run("Scenario_RegistryRejectsDuplicateSlug", func(t *testing.T) {
		// Given a registry already has a researcher,
		// When a second register attempt uses the same slug,
		// Then duplicate is rejected — prevents accidental overwrite.
		r := NewBuiltinSubagentRegistry()
		d := BuiltinSubagentDescriptor{
			Role: BuiltinSubagentResearcher, Slug: "researcher-1",
			Name: "x", Description: "x", SystemPromptTemplate: "x",
			DefaultToolsetPolicySlug:   "documentation-readonly-allowlist",
			DefaultInheritanceModeSlug: "extend-parent-rights",
			DefaultSummaryShapeSlug:    "success-with-artifacts",
		}
		require.NoError(t, r.Register(d))
		err := r.Register(d)
		assert.ErrorIs(t, err, ErrBuiltinSubagentDuplicateSlug)
	})

	t.Run("Scenario_ListByRoleSupportsAdminUIRoster", func(t *testing.T) {
		// Given an admin UI shows builtins grouped by role,
		// When ListByRole runs,
		// Then it returns only that role's descriptors, sorted by slug.
		r := NewBuiltinSubagentRegistry()
		for _, slug := range []string{"researcher-a", "researcher-b", "coder-a"} {
			d := BuiltinSubagentDescriptor{
				Role: BuiltinSubagentResearcher, Slug: slug,
				Name: "x", Description: "x", SystemPromptTemplate: "x",
				DefaultToolsetPolicySlug:   "documentation-readonly-allowlist",
				DefaultInheritanceModeSlug: "extend-parent-rights",
				DefaultSummaryShapeSlug:    "success-with-artifacts",
			}
			if slug == "coder-a" {
				d.Role = BuiltinSubagentCoder
			}
			_ = r.Register(d)
		}
		researchers := r.ListByRole(BuiltinSubagentResearcher)
		assert.Equal(t, 2, len(researchers))
		assert.Equal(t, "researcher-a", researchers[0].Slug)
	})

	t.Run("Scenario_SlugMustBeKebabForRoutability", func(t *testing.T) {
		// Given slugs are used as URL fragments and JSON keys,
		// When a snake_case slug is attempted,
		// Then validation rejects — only kebab-case is accepted.
		d := BuiltinSubagentDescriptor{
			Role: BuiltinSubagentResearcher, Slug: "snake_case",
			Name: "x", Description: "x", SystemPromptTemplate: "x",
			DefaultToolsetPolicySlug: "x", DefaultInheritanceModeSlug: "x",
			DefaultSummaryShapeSlug: "x",
		}
		assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentBadSlug)
	})

	t.Run("Scenario_AllPolicyReferencesMandatory", func(t *testing.T) {
		// Given the runner needs all 3 policy refs (toolset + inheritance
		// + summary),
		// When any is empty,
		// Then validation rejects — runner cannot spawn safely without
		// all three.
		base := BuiltinSubagentDescriptor{
			Role: BuiltinSubagentResearcher, Slug: "researcher",
			Name: "x", Description: "x", SystemPromptTemplate: "x",
			DefaultToolsetPolicySlug:   "a",
			DefaultInheritanceModeSlug: "b",
			DefaultSummaryShapeSlug:    "c",
		}
		base.DefaultToolsetPolicySlug = ""
		assert.ErrorIs(t, base.Validate(), ErrBuiltinSubagentEmptyToolsetSlug)
		base.DefaultToolsetPolicySlug = "a"
		base.DefaultInheritanceModeSlug = ""
		assert.ErrorIs(t, base.Validate(), ErrBuiltinSubagentEmptyInheritanceSlug)
		base.DefaultInheritanceModeSlug = "b"
		base.DefaultSummaryShapeSlug = ""
		assert.ErrorIs(t, base.Validate(), ErrBuiltinSubagentEmptySummarySlug)
	})

	t.Run("Scenario_SystemPromptRequiredForRunnerExecution", func(t *testing.T) {
		// Given the runner needs a system prompt to start the LLM,
		// When the prompt template is empty,
		// Then validation rejects.
		d := BuiltinSubagentDescriptor{
			Role: BuiltinSubagentResearcher, Slug: "researcher",
			Name: "x", Description: "x", SystemPromptTemplate: "",
			DefaultToolsetPolicySlug:   "a",
			DefaultInheritanceModeSlug: "b",
			DefaultSummaryShapeSlug:    "c",
		}
		assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentEmptySystemPrompt)
	})

	t.Run("Scenario_RegistryThreadSafeForConcurrentBootstrap", func(t *testing.T) {
		// Given the platform bootstraps 7 builtins concurrently,
		// When all register at once,
		// Then no race + final Size matches.
		r := NewBuiltinSubagentRegistry()
		roles := AllBuiltinSubagentRoles()
		done := make(chan struct{})
		for i, role := range roles {
			go func(idx int, rr BuiltinSubagentRole) {
				d := BuiltinSubagentDescriptor{
					Role: rr, Slug: "agent-" + string(rune('a'+idx)),
					Name: "x", Description: "x", SystemPromptTemplate: "x",
					DefaultToolsetPolicySlug:   "a",
					DefaultInheritanceModeSlug: "b",
					DefaultSummaryShapeSlug:    "c",
				}
				_ = r.Register(d)
				done <- struct{}{}
			}(i, role)
		}
		for range roles {
			<-done
		}
		assert.Equal(t, 7, r.Size())
	})

	t.Run("Scenario_LookupCrossReferencesSUB006SUB010Catalogs", func(t *testing.T) {
		// Given a builtin's descriptor carries SUB-005/006/010 slugs,
		// When the runner resolves them at spawn time,
		// Then it loads matching policies from the live catalogs by slug —
		// type-level integration. (Wiring is downstream; SUB-002 only
		// declares the references.)
		r := NewBuiltinSubagentRegistry()
		d := BuiltinSubagentDescriptor{
			Role: BuiltinSubagentReviewer, Slug: "reviewer-baseline",
			Name: "Reviewer", Description: "code reviewer",
			SystemPromptTemplate:       "You review code.",
			DefaultToolsetPolicySlug:   "documentation-readonly-allowlist", // SUB-005 slug
			DefaultInheritanceModeSlug: "sandboxed-worker",                 // SUB-006 slug
			DefaultSummaryShapeSlug:    "success-with-artifacts",           // SUB-010 slug
		}
		require.NoError(t, r.Register(d))
		got, _ := r.Lookup("reviewer-baseline")
		assert.Equal(t, "sandboxed-worker", got.DefaultInheritanceModeSlug)
	})
}
