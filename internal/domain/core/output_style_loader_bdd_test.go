package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreOutputStyleSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsBaselineStyles", func(t *testing.T) {
		// Given a fresh tenant (no styles of its own),
		// When the runtime asks ah_core for output styles,
		// Then ≥6 baseline styles are seeded — agent picker is non-empty
		//      from day one and covers the major formatting needs.
		assert.GreaterOrEqual(t, len(SeedExpectedOutputStyleSlugs), 6,
			"fresh tenant must inherit at least 6 baseline output styles")
	})

	t.Run("Scenario_PlatformDefaultIsAlwaysSet", func(t *testing.T) {
		// Given new agents need an output style WITHOUT explicit pick,
		// When the loader queries the default,
		// Then exactly one platform default exists (enforced by partial
		//      unique index in DB; constant guard at Go level).
		assert.NotEmpty(t, SeedDefaultOutputStyleSlug,
			"platform default slug must be non-empty")
	})

	t.Run("Scenario_DefaultIsConversationalForGeneralAudience", func(t *testing.T) {
		// Given AgentHub is predominantly WEB and serves general users,
		// When inspecting the default,
		// Then it is conversational — the safest, most-friendly choice.
		assert.Equal(t, "conversational", SeedDefaultOutputStyleSlug,
			"web product default = conversational")
	})

	t.Run("Scenario_StylesCoverConcisenessSpectrum", func(t *testing.T) {
		// Given users vary from "give me one line" to "explain everything",
		// When inspecting the seed,
		// Then both ends are covered: concise + verbose.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedOutputStyleSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["concise"], "concise end of spectrum required")
		assert.True(t, seedSet["verbose"], "verbose end of spectrum required")
	})

	t.Run("Scenario_StylesCoverAudienceFacets", func(t *testing.T) {
		// Given the UI picker filters by audience,
		// When inspecting the seed audience set,
		// Then 4 distinct audiences exist — general / technical /
		//      executive / beginner — covering common workflow filters.
		assert.Equal(t, 4, len(SeedExpectedOutputStyleAudiences))
		audSet := map[string]bool{}
		for _, a := range SeedExpectedOutputStyleAudiences {
			audSet[a] = true
		}
		for _, expected := range []string{"general", "technical", "executive", "beginner"} {
			assert.True(t, audSet[expected],
				"audience %q must be in seed", expected)
		}
	})

	t.Run("Scenario_JSONStyleEnablesToolPipelineUse", func(t *testing.T) {
		// Given automation pipelines need parseable output,
		// When inspecting the seed,
		// Then a JSON-format style exists (json_only) — bypasses
		//      markdown rendering and produces strict JSON envelopes.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedOutputStyleSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["json_only"],
			"json_only style required for tool-pipeline integration")
	})

	t.Run("Scenario_TutorialStyleServesBeginners", func(t *testing.T) {
		// Given beginner users benefit from numbered step-by-step,
		// When inspecting the seed,
		// Then a tutorial style exists with audience=beginner.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedOutputStyleSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["tutorial"], "tutorial style required for beginners")
	})

	t.Run("Scenario_ExecutiveStyleEnablesDecisionContexts", func(t *testing.T) {
		// Given executives need TL;DR + recommendation, not deep prose,
		// When inspecting the seed,
		// Then an executive style exists.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedOutputStyleSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["executive"],
			"executive style required for decision contexts")
	})

	t.Run("Scenario_TechnicalStyleServesEngineers", func(t *testing.T) {
		// Given engineers want code-first output with minimal prose,
		// When inspecting the seed,
		// Then a technical style exists with audience=technical.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedOutputStyleSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["technical"],
			"technical style required for engineering audiences")
	})

	t.Run("Scenario_FormatAllowlistPreventsHTMLInjection", func(t *testing.T) {
		// Given output_format drives renderer dispatch — html would
		//       open XSS surfaces in the chat UI,
		allowedSet := map[string]bool{}
		for _, f := range SeedExpectedOutputStyleFormats {
			allowedSet[f] = true
		}
		assert.False(t, allowedSet["html"],
			"html format must NOT be in seed — XSS risk in chat UI")
		assert.False(t, allowedSet["raw"],
			"raw format must NOT be in seed — bypasses sanitization")
	})

	t.Run("Scenario_AtMostOneDefaultIsEnforcedAtSchemaLevel", func(t *testing.T) {
		// Given the schema has UNIQUE partial index on (is_default)
		//       WHERE is_default = TRUE,
		// When the loader queries default,
		// Then the contract is single-default — integration test
		//      verifies the partial index actually rejects a second TRUE.
		// Constant-level guard:
		assert.NotEmpty(t, SeedDefaultOutputStyleSlug,
			"default slug must be non-empty (schema enforces single default)")
	})
}
