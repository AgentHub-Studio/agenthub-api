package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for FEAT-022 — ComparativeDimensionRegistry (§10 Table 3).
// Each scenario is written in t.Run("Scenario_...") format per project convention.

// TestFEAT022_BDD_RegistryReturnsAllSixDimensions covers the foundational
// expectation that the registry encodes every row of Table 3.
func TestFEAT022_BDD_RegistryReturnsAllSixDimensions(t *testing.T) {
	t.Run("Scenario_NewRegistryHasSixEntries", func(t *testing.T) {
		// Given: the comparative dimension registry is initialised
		dims := NewComparativeDimensionRegistry()

		// Then: it contains exactly six entries (Table 3 has six rows)
		assert.Len(t, dims, 6)
	})

	t.Run("Scenario_EachEntryHasDistinctID", func(t *testing.T) {
		// Given: the full registry
		dims := NewComparativeDimensionRegistry()

		// When: we collect all IDs
		seen := map[ComparativeDimensionID]bool{}
		for _, d := range dims {
			seen[d.ID] = true
		}

		// Then: no two entries share the same ID
		assert.Len(t, seen, len(dims))
	})
}

// TestFEAT022_BDD_FindBySlugResolvesAndRejects exercises the lookup helper.
func TestFEAT022_BDD_FindBySlugResolvesAndRejects(t *testing.T) {
	t.Run("Scenario_KnownSlugResolves", func(t *testing.T) {
		// Given: a known dimension slug
		// When: FindComparativeDimensionBySlug is called
		p, ok := FindComparativeDimensionBySlug(DimMemoryAndContext)

		// Then: the call succeeds and the profile ID matches
		require.True(t, ok)
		assert.Equal(t, DimMemoryAndContext, p.ID)
		assert.NotEmpty(t, p.Label)
	})

	t.Run("Scenario_UnknownSlugReturnsFalse", func(t *testing.T) {
		// Given: an unrecognised dimension slug
		// When: FindComparativeDimensionBySlug is called
		_, ok := FindComparativeDimensionBySlug("nonexistent_dimension")

		// Then: the result signals not-found
		assert.False(t, ok)
	})
}

// TestFEAT022_BDD_SystemAnswerDispatches verifies the two-system dispatch.
func TestFEAT022_BDD_SystemAnswerDispatches(t *testing.T) {
	t.Run("Scenario_ClaudeCodeAnswerIsDistinctFromOpenClaw", func(t *testing.T) {
		// Given: any dimension where the two systems differ
		p, ok := FindComparativeDimensionBySlug(DimTrustModel)
		require.True(t, ok)

		// When: answers are retrieved for each system
		ccAns := p.SystemAnswer(SystemClaudeCode)
		ocAns := p.SystemAnswer(SystemOpenClaw)

		// Then: both are non-empty and differ (the core claim of §10)
		assert.NotEmpty(t, ccAns)
		assert.NotEmpty(t, ocAns)
		assert.NotEqual(t, ccAns, ocAns)
	})

	t.Run("Scenario_UnknownSystemSlugYieldsEmptyString", func(t *testing.T) {
		// Given: any dimension profile
		p, ok := FindComparativeDimensionBySlug(DimAgentRuntime)
		require.True(t, ok)

		// When: an unrecognised system slug is requested
		ans := p.SystemAnswer("some_other_system")

		// Then: an empty string is returned (not a panic or error)
		assert.Empty(t, ans)
	})
}

// TestFEAT022_BDD_ConvergenceAnnotation verifies §10.2 "What the Contrast
// Reveals" convergence notes.
func TestFEAT022_BDD_ConvergenceAnnotation(t *testing.T) {
	t.Run("Scenario_StackableDimensionHasConvergenceNote", func(t *testing.T) {
		// Given: the system_scope dimension (both systems can stack via ACP)
		p, ok := FindComparativeDimensionBySlug(DimSystemScope)
		require.True(t, ok)

		// Then: HasConvergence reports true and the note is non-empty
		assert.True(t, p.HasConvergence())
		assert.NotEmpty(t, p.ConvergenceNote)
	})

	t.Run("Scenario_PureDivergenceDimensionHasNoConvergenceNote", func(t *testing.T) {
		// Given: the trust_model dimension (no common ground identified in §10.1)
		p, ok := FindComparativeDimensionBySlug(DimTrustModel)
		require.True(t, ok)

		// Then: HasConvergence reports false
		assert.False(t, p.HasConvergence())
		assert.Empty(t, p.ConvergenceNote)
	})
}

// TestFEAT022_BDD_ExtensionMechanismCountInAnswer verifies the §10.1 claim that
// Claude Code has exactly four extension mechanisms.
func TestFEAT022_BDD_ExtensionMechanismCountInAnswer(t *testing.T) {
	t.Run("Scenario_FourExtensionMechanismsMentionedInClaudeCodeAnswer", func(t *testing.T) {
		// Given: the extension_architecture dimension
		p, ok := FindComparativeDimensionBySlug(DimExtensionArchitecture)
		require.True(t, ok)

		// When: the Claude Code answer is inspected
		ans := p.SystemAnswer(SystemClaudeCode)

		// Then: all four mechanism names appear (MCP, plugins, skills, hooks)
		mechanisms := []string{"MCP", "plugins", "skills", "hooks"}
		for _, m := range mechanisms {
			assert.Contains(t, ans, m, "expected mechanism %q in ClaudeCodeAnswer", m)
		}
	})
}

// TestFEAT022_BDD_DesignQuestionIsPresent verifies every dimension has a
// well-formed DesignQuestion (ends with "?").
func TestFEAT022_BDD_DesignQuestionIsPresent(t *testing.T) {
	t.Run("Scenario_AllDesignQuestionsEndWithQuestionMark", func(t *testing.T) {
		// Given: the full registry
		dims := NewComparativeDimensionRegistry()

		// Then: every DesignQuestion ends with "?"
		for _, d := range dims {
			assert.True(t,
				len(d.DesignQuestion) > 0 && d.DesignQuestion[len(d.DesignQuestion)-1] == '?',
				"DesignQuestion for %q must end with '?', got: %q", d.ID, d.DesignQuestion)
		}
	})
}
