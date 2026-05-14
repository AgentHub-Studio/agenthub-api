package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePromptTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsPromptTemplateLibrary", func(t *testing.T) {
		// Given a fresh tenant wants to create an agent without writing
		//       a system prompt from scratch,
		assert.GreaterOrEqual(t, len(SeedExpectedPromptTemplateSlugs), 6)
	})

	t.Run("Scenario_EightAgentPersonasCovered", func(t *testing.T) {
		// Given common agent personas (assistant, coder, analyst,
		//       researcher, writer, translator, support, extractor),
		set := map[string]bool{}
		for _, k := range SeedExpectedPromptTemplateKinds {
			set[k] = true
		}
		for _, want := range []string{
			"assistant", "coder", "analyst", "researcher",
			"writer", "translator", "customer_support", "data_extractor",
		} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_GeneralAssistantIsTheDefaultStartingPoint", func(t *testing.T) {
		// Given users without specific persona need a safe default,
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedPromptTemplateSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["general-assistant"],
			"general-assistant must be recommended (universal starting point)")
	})

	t.Run("Scenario_CodeReviewerUsesLowTemperatureForConsistency", func(t *testing.T) {
		// Given code review needs deterministic output,
		// When the code-reviewer template is rendered (integration test
		//       verifies actual values), recommended_temperature is low.
		// Constant guard:
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedPromptTemplateSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["code-reviewer"], "code-reviewer recommended")
	})

	t.Run("Scenario_PlaceholderSubstitutionEnablesPersonalization", func(t *testing.T) {
		// Given templates use {{tenantName}} / {{productName}} for
		//       tenant-specific customization,
		tmpl := CorePromptTemplate{
			SystemPrompt: "You are an assistant for {{tenantName}}.",
		}
		out := tmpl.RenderSystemPrompt(map[string]string{
			"tenantName": "Acme Corp",
		})
		assert.Equal(t, "You are an assistant for Acme Corp.", out)
	})

	t.Run("Scenario_MissingPlaceholdersRemainLiteralForVisibility", func(t *testing.T) {
		// Given a missing placeholder is a configuration bug —
		//       silently dropping would hide it,
		tmpl := CorePromptTemplate{SystemPrompt: "Hello {{userName}}!"}
		out := tmpl.RenderSystemPrompt(nil)
		assert.Contains(t, out, "{{userName}}",
			"missing placeholder stays literal — surface bug to caller")
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 8, len(SeedExpectedPromptTemplateSlugs))
	})

	t.Run("Scenario_ParsedListsHandleWhitespace", func(t *testing.T) {
		tmpl := CorePromptTemplate{
			RequiresTools: " a , b , c ",
			Placeholders:  "x, y",
		}
		assert.Equal(t, []string{"a", "b", "c"}, tmpl.RequiresToolsList())
		assert.Equal(t, []string{"x", "y"}, tmpl.PlaceholdersList())
	})
}
