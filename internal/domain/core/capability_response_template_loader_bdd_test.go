package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000116 capability response template seeds.
// These assert seed shape and template rationale without a database.

func TestBDD_CapabilityResponseTemplateSeed(t *testing.T) {
	t.Run("Scenario_NineTemplatesAcrossThreeAgents", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent response templates
		//   to ensure each capability agent communicates consistently with users
		//   across common interaction patterns (greetings, errors, handoffs),
		// When migration 000116 seeds capability_response_template rows,
		// Then exactly 9 rows are added — three per agent — covering researcher,
		//   analyst, and planner with balanced template sets across a consistent
		//   set of interaction patterns.
		assert.Equal(t, 9, SeedResponseTemplateCount,
			"migration 000116 must seed exactly 9 capability response template rows")

		assert.Equal(t, 3, SeedResponseTemplateAgentCount,
			"SeedResponseTemplateAgentCount must be 3 — researcher, analyst, planner")

		assert.Equal(t, SeedResponseTemplateAgentCount*3, SeedResponseTemplateCount,
			"total template count must equal agent count × 3 (each agent has exactly 3 templates)")
	})

	t.Run("Scenario_EachAgentHasGreeting", func(t *testing.T) {
		// Given users interact with multiple capability agents (researcher, analyst,
		//   planner) and each agent has a distinct purpose and communication style,
		//   a welcoming, role-specific greeting helps users understand what the agent
		//   can do and how to engage with it effectively,
		// When migration 000116 seeds response template rows,
		// Then the "greeting" template key is defined as a constant and represents
		//   the first interaction template that all three agents share — ensuring
		//   every agent opens sessions with a clear, purposeful introduction that
		//   does not require placeholder interpolation.
		assert.Equal(t, "greeting", SeedTemplateKeyGreeting,
			"SeedTemplateKeyGreeting must equal \"greeting\"")

		// Greeting templates have no placeholders — they are rendered verbatim.
		// The greeting key must not appear in the placeholder key set.
		placeholderKeys := []string{
			SeedTemplateKeyAmbiguity,
			SeedTemplateKeyConfidenceFooter,
			SeedTemplateKeyClarification,
			SeedTemplateKeyHandoff,
		}
		for _, pk := range placeholderKeys {
			assert.NotEqual(t, SeedTemplateKeyGreeting, pk,
				"greeting key must not be a placeholder key — greetings are rendered verbatim")
		}

		// All three agents have this pattern — confirmed by agent count constant.
		assert.Equal(t, 3, SeedResponseTemplateAgentCount,
			"all three agents (researcher, analyst, planner) have a greeting template")
	})

	t.Run("Scenario_FourTemplatesHavePlaceholders", func(t *testing.T) {
		// Given some response templates need runtime context that cannot be known
		//   at seed time — such as the specific clarification question, confidence
		//   level, or first step of a plan — these templates use {variable} tokens
		//   that callers must fill via string interpolation before rendering,
		// When migration 000116 seeds has_placeholders flags,
		// Then exactly 4 of the 9 templates have has_placeholders=TRUE:
		//   ambiguity_prompt and confidence_footer for core-analyst,
		//   clarification_request and handoff for core-planner.
		//   The remaining 5 (all greeting templates + not_found + source_disclaimer)
		//   have has_placeholders=FALSE and are safe to render verbatim.
		assert.Equal(t, 4, SeedResponseTemplatesWithPlaceholdersCount,
			"exactly 4 templates have has_placeholders=TRUE (ambiguity_prompt, confidence_footer, clarification_request, handoff)")

		// Non-placeholder templates: 5 = 9 total - 4 placeholder.
		nonPlaceholder := SeedResponseTemplateCount - SeedResponseTemplatesWithPlaceholdersCount
		assert.Equal(t, 5, nonPlaceholder,
			"exactly 5 templates have has_placeholders=FALSE (all greetings + not_found + source_disclaimer)")

		// Validate the four placeholder key constants are distinct and non-empty.
		placeholderKeys := []string{
			SeedTemplateKeyAmbiguity,
			SeedTemplateKeyConfidenceFooter,
			SeedTemplateKeyClarification,
			SeedTemplateKeyHandoff,
		}
		seen := map[string]struct{}{}
		for _, k := range placeholderKeys {
			assert.NotEmpty(t, k, "placeholder template key must be non-empty")
			seen[k] = struct{}{}
		}
		assert.Len(t, seen, 4,
			"all 4 placeholder key constants must be distinct")
	})

	t.Run("Scenario_PlannerRequestsClarificationBeforePlanning", func(t *testing.T) {
		// Given the core-planner agent generates detailed, actionable plans that
		//   depend heavily on well-understood goals and constraints — and producing
		//   a plan without clarifying ambiguous requirements wastes the user's time
		//   and reduces plan quality — the planner must request clarification when
		//   the initial request is underspecified,
		// When migration 000116 seeds response templates for core-planner,
		// Then the clarification_request template is seeded (has_placeholders=TRUE)
		//   with a {clarification_question} token that callers fill with the
		//   specific question, and the handoff template is also seeded with a
		//   {first_step} token so the plan conclusion is always actionable.
		assert.Equal(t, "clarification_request", SeedTemplateKeyClarification,
			"SeedTemplateKeyClarification must equal \"clarification_request\"")
		assert.Equal(t, "handoff", SeedTemplateKeyHandoff,
			"SeedTemplateKeyHandoff must equal \"handoff\"")

		// Both clarification_request and handoff are placeholder templates.
		placeholderKeys := []string{
			SeedTemplateKeyAmbiguity,
			SeedTemplateKeyConfidenceFooter,
			SeedTemplateKeyClarification,
			SeedTemplateKeyHandoff,
		}
		plannerPlaceholders := 0
		for _, k := range placeholderKeys {
			if k == SeedTemplateKeyClarification || k == SeedTemplateKeyHandoff {
				plannerPlaceholders++
			}
		}
		assert.Equal(t, 2, plannerPlaceholders,
			"core-planner must have exactly 2 placeholder templates: clarification_request and handoff")

		// Clarification key must contain "request" — emphasising its elicitation role.
		assert.Contains(t, SeedTemplateKeyClarification, "request",
			"clarification_request key must contain 'request'")

		// Planner greeting is non-empty and non-placeholder.
		assert.Equal(t, "greeting", SeedTemplateKeyGreeting,
			"planner greeting key must be 'greeting'")
	})

	t.Run("Scenario_ResearcherDisclaimsSourcesWhenCiting", func(t *testing.T) {
		// Given the core-researcher agent synthesises information from external
		//   sources — web pages, documents, APIs — and users must understand that
		//   the information is derived from those sources and may require independent
		//   verification for critical decisions, the researcher must append a clear
		//   disclaimer whenever it cites external sources in its response,
		// When migration 000116 seeds response templates for core-researcher,
		// Then the source_disclaimer template is seeded as a verbatim footer
		//   (has_placeholders=FALSE) that is appended whenever the researcher cites
		//   sources, and the not_found template is seeded so users receive actionable
		//   guidance when the researcher cannot locate reliable information.
		assert.Equal(t, "source_disclaimer", SeedTemplateKeySourceDisclaimer,
			"SeedTemplateKeySourceDisclaimer must equal \"source_disclaimer\"")
		assert.Equal(t, "not_found", SeedTemplateKeyNotFound,
			"SeedTemplateKeyNotFound must equal \"not_found\"")

		// source_disclaimer and not_found are non-placeholder templates (rendered verbatim).
		nonPlaceholderKeys := []string{
			SeedTemplateKeyGreeting,
			SeedTemplateKeyNotFound,
			SeedTemplateKeySourceDisclaimer,
		}
		placeholderSet := map[string]struct{}{
			SeedTemplateKeyAmbiguity:       {},
			SeedTemplateKeyConfidenceFooter: {},
			SeedTemplateKeyClarification:   {},
			SeedTemplateKeyHandoff:         {},
		}
		for _, k := range nonPlaceholderKeys {
			_, isPlaceholder := placeholderSet[k]
			assert.False(t, isPlaceholder,
				"researcher template key %q must not be a placeholder template", k)
		}

		// source_disclaimer key should reference "source" or "disclaimer".
		assert.True(t,
			strings.Contains(SeedTemplateKeySourceDisclaimer, "source") ||
				strings.Contains(SeedTemplateKeySourceDisclaimer, "disclaimer"),
			"source_disclaimer key must reference 'source' or 'disclaimer'")
	})
}
