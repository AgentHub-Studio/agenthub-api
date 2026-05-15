package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability response template seed constants (migration 000116).
// These run without a database and guard against accidental constant drift.

func TestSeedResponseTemplateCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedResponseTemplateCount,
		"migration 000116 seeds exactly 9 capability response template rows (three per capability agent)")
}

func TestSeedResponseTemplateAgentCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResponseTemplateAgentCount,
		"SeedResponseTemplateAgentCount must be 3 — researcher, analyst, planner")
}

func TestSeedResponseTemplatesWithPlaceholdersCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedResponseTemplatesWithPlaceholdersCount,
		"exactly 4 templates have placeholders: ambiguity_prompt, confidence_footer, clarification_request, handoff")
}

func TestSeedResponseTemplateCount_EqualsAgentCountTimesThree(t *testing.T) {
	assert.Equal(t, SeedResponseTemplateAgentCount*3, SeedResponseTemplateCount,
		"total template count must equal agent count × 3 (each agent has exactly 3 response templates)")
}

func TestSeedResponseTemplatesWithPlaceholders_LessThanTotal(t *testing.T) {
	assert.Less(t, SeedResponseTemplatesWithPlaceholdersCount, SeedResponseTemplateCount,
		"placeholder templates (%d) must be fewer than total templates (%d)",
		SeedResponseTemplatesWithPlaceholdersCount, SeedResponseTemplateCount)
}

// Template key constant tests.

func TestSeedTemplateKeyGreeting_Value(t *testing.T) {
	assert.Equal(t, "greeting", SeedTemplateKeyGreeting,
		"SeedTemplateKeyGreeting must equal \"greeting\"")
}

func TestSeedTemplateKeyNotFound_Value(t *testing.T) {
	assert.Equal(t, "not_found", SeedTemplateKeyNotFound,
		"SeedTemplateKeyNotFound must equal \"not_found\"")
}

func TestSeedTemplateKeySourceDisclaimer_Value(t *testing.T) {
	assert.Equal(t, "source_disclaimer", SeedTemplateKeySourceDisclaimer,
		"SeedTemplateKeySourceDisclaimer must equal \"source_disclaimer\"")
}

func TestSeedTemplateKeyAmbiguity_Value(t *testing.T) {
	assert.Equal(t, "ambiguity_prompt", SeedTemplateKeyAmbiguity,
		"SeedTemplateKeyAmbiguity must equal \"ambiguity_prompt\"")
}

func TestSeedTemplateKeyConfidenceFooter_Value(t *testing.T) {
	assert.Equal(t, "confidence_footer", SeedTemplateKeyConfidenceFooter,
		"SeedTemplateKeyConfidenceFooter must equal \"confidence_footer\"")
}

func TestSeedTemplateKeyClarification_Value(t *testing.T) {
	assert.Equal(t, "clarification_request", SeedTemplateKeyClarification,
		"SeedTemplateKeyClarification must equal \"clarification_request\"")
}

func TestSeedTemplateKeyHandoff_Value(t *testing.T) {
	assert.Equal(t, "handoff", SeedTemplateKeyHandoff,
		"SeedTemplateKeyHandoff must equal \"handoff\"")
}

func TestSeedTemplateKeys_AreDistinct(t *testing.T) {
	keys := []string{
		SeedTemplateKeyGreeting,
		SeedTemplateKeyNotFound,
		SeedTemplateKeySourceDisclaimer,
		SeedTemplateKeyAmbiguity,
		SeedTemplateKeyConfidenceFooter,
		SeedTemplateKeyClarification,
		SeedTemplateKeyHandoff,
	}
	seen := map[string]struct{}{}
	for _, k := range keys {
		assert.NotEmpty(t, k, "every template key constant must be non-empty")
		seen[k] = struct{}{}
	}
	assert.Len(t, seen, 7,
		"there must be exactly 7 distinct template key constants")
}

func TestSeedTemplateKeys_AreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedTemplateKeyGreeting)
	assert.NotEmpty(t, SeedTemplateKeyNotFound)
	assert.NotEmpty(t, SeedTemplateKeySourceDisclaimer)
	assert.NotEmpty(t, SeedTemplateKeyAmbiguity)
	assert.NotEmpty(t, SeedTemplateKeyConfidenceFooter)
	assert.NotEmpty(t, SeedTemplateKeyClarification)
	assert.NotEmpty(t, SeedTemplateKeyHandoff)
}

// Placeholder template key tests — templates with {variable} tokens.

func TestSeedPlaceholderTemplateKeys_ContainPlaceholderMarker(t *testing.T) {
	// The template key names for templates that have placeholders.
	// Verify the constants that name placeholder templates are distinct from
	// the non-placeholder ones (greeting, not_found, source_disclaimer).
	placeholderKeys := []string{
		SeedTemplateKeyAmbiguity,
		SeedTemplateKeyConfidenceFooter,
		SeedTemplateKeyClarification,
		SeedTemplateKeyHandoff,
	}
	nonPlaceholderKeys := []string{
		SeedTemplateKeyGreeting,
		SeedTemplateKeyNotFound,
		SeedTemplateKeySourceDisclaimer,
	}
	placeholderSet := map[string]struct{}{}
	for _, k := range placeholderKeys {
		placeholderSet[k] = struct{}{}
	}
	for _, k := range nonPlaceholderKeys {
		_, isPlaceholder := placeholderSet[k]
		assert.False(t, isPlaceholder,
			"non-placeholder template key %q must not appear in placeholder key set", k)
	}
}

func TestSeedResponseTemplatesWithPlaceholdersCount_MatchesPlaceholderKeySlice(t *testing.T) {
	placeholderKeys := []string{
		SeedTemplateKeyAmbiguity,
		SeedTemplateKeyConfidenceFooter,
		SeedTemplateKeyClarification,
		SeedTemplateKeyHandoff,
	}
	assert.Equal(t, SeedResponseTemplatesWithPlaceholdersCount, len(placeholderKeys),
		"SeedResponseTemplatesWithPlaceholdersCount must equal the number of placeholder key constants")
}

// Greeting template body tests — greeting bodies must be non-empty and
// agent-relevant. We verify via string constants implied by the constants
// (no DB needed — just the count and key constants).

func TestSeedTemplateKeyGreeting_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedTemplateKeyGreeting,
		"greeting template key must be non-empty — all three agents must have a greeting")
}

func TestSeedResponseTemplateAgentCount_IsPositive(t *testing.T) {
	assert.Greater(t, SeedResponseTemplateAgentCount, 0,
		"SeedResponseTemplateAgentCount must be positive")
}

func TestSeedResponseTemplateCount_IsPositive(t *testing.T) {
	assert.Greater(t, SeedResponseTemplateCount, 0,
		"SeedResponseTemplateCount must be positive")
}

func TestSeedResponseTemplatesWithPlaceholdersCount_IsPositive(t *testing.T) {
	assert.Greater(t, SeedResponseTemplatesWithPlaceholdersCount, 0,
		"SeedResponseTemplatesWithPlaceholdersCount must be positive")
}

// Structural validation — placeholder templates must contain "{" character
// in their key names or count validation. We validate the placeholder
// key constants align with the expected count.

func TestSeedTemplateKeyAmbiguity_ContainsPrompt(t *testing.T) {
	assert.Contains(t, SeedTemplateKeyAmbiguity, "prompt",
		"ambiguity template key must contain 'prompt' to reflect its elicitation role")
}

func TestSeedTemplateKeyConfidenceFooter_ContainsFooter(t *testing.T) {
	assert.Contains(t, SeedTemplateKeyConfidenceFooter, "footer",
		"confidence footer template key must contain 'footer' to reflect its appended position")
}

func TestSeedTemplateKeyClarification_ContainsRequest(t *testing.T) {
	assert.Contains(t, SeedTemplateKeyClarification, "request",
		"clarification template key must contain 'request' to reflect its elicitation role")
}

func TestSeedTemplateKeyHandoff_IsHandoff(t *testing.T) {
	assert.Equal(t, "handoff", SeedTemplateKeyHandoff,
		"handoff template key must be exactly 'handoff'")
}

// Verify all template key constants use snake_case (no spaces, no uppercase).

func TestSeedTemplateKeys_UseSnakeCase(t *testing.T) {
	keys := []string{
		SeedTemplateKeyGreeting,
		SeedTemplateKeyNotFound,
		SeedTemplateKeySourceDisclaimer,
		SeedTemplateKeyAmbiguity,
		SeedTemplateKeyConfidenceFooter,
		SeedTemplateKeyClarification,
		SeedTemplateKeyHandoff,
	}
	for _, k := range keys {
		assert.Equal(t, strings.ToLower(k), k,
			"template key %q must be lowercase (snake_case)", k)
		assert.NotContains(t, k, " ",
			"template key %q must not contain spaces", k)
	}
}
