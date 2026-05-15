package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FEAT029 BDD — §3.1 Design Space Question Registry
// arXiv:2604.14228v1 §3.1 "Design Questions and Running Example"
//
// Each scenario is a self-contained Given/When/Then narrative that validates
// a structural property of the §3.1 four recurring design questions.

func TestFEAT029_BDD_DesignSpaceQuestion(t *testing.T) {

	t.Run("Scenario_FourDesignQuestionsExactlyAsDefined", func(t *testing.T) {
		// Given §3.1 of arXiv:2604.14228v1 identifies exactly four recurring
		// design questions every production coding agent must answer,
		// When the DesignSpaceQuestionRegistry is constructed,
		// Then it contains exactly four profiles with the canonical slugs.
		r := NewDesignSpaceQuestionRegistry()
		all := r.AllQuestions()

		assert.Len(t, all, 4,
			"§3.1 defines exactly four recurring design questions")
		assert.True(t, r.IsValidSlug(DesignQuestionWhereReasoningLives))
		assert.True(t, r.IsValidSlug(DesignQuestionHowManyEngines))
		assert.True(t, r.IsValidSlug(DesignQuestionDefaultSafetyPosture))
		assert.True(t, r.IsValidSlug(DesignQuestionBindingResourceConstraint))
	})

	t.Run("Scenario_EachQuestionHasPDFSection31Citation", func(t *testing.T) {
		// Given all four questions are introduced in §3.1 of the paper,
		// When a consumer inspects any profile's PDFSection field,
		// Then it must always equal "3.1" — no question references a different section.
		r := NewDesignSpaceQuestionRegistry()
		for _, q := range r.AllQuestions() {
			assert.Equal(t, "3.1", q.PDFSection,
				"question %q PDFSection must be '3.1'", q.Slug)
		}
	})

	t.Run("Scenario_WhereReasoningLivesNamesQueryLoopAndRatios", func(t *testing.T) {
		// Given §3.1 states ~1.6% AI decision logic / 98.4% harness, citing query.ts,
		// When the 'where_reasoning_lives' profile is retrieved,
		// Then the ClaudeAnswer must mention both the 1.6% and 98.4% ratios,
		// And the first SourceEvidence entry must reference query.ts.
		r := NewDesignSpaceQuestionRegistry()
		p, ok := r.FindDesignSpaceQuestionBySlug(DesignQuestionWhereReasoningLives)
		require.True(t, ok)
		assert.Contains(t, p.ClaudeAnswer, "1.6%")
		assert.Contains(t, p.ClaudeAnswer, "98.4%")
		require.NotEmpty(t, p.SourceEvidence)
		assert.Contains(t, p.SourceEvidence[0], "query.ts")
	})

	t.Run("Scenario_DefaultSafetyPostureDocumentsAlternativeSystems", func(t *testing.T) {
		// Given §3.1 contrasts Claude's deny-first posture with SWE-Agent/OpenHands
		// (container isolation) and Aider (git-based rollback),
		// When the 'default_safety_posture' alternatives are inspected,
		// Then both named alternative systems must appear and use type 'container_isolation'.
		r := NewDesignSpaceQuestionRegistry()
		p, ok := r.FindDesignSpaceQuestionBySlug(DesignQuestionDefaultSafetyPosture)
		require.True(t, ok)
		systemNames := make([]string, len(p.Alternatives))
		for i, a := range p.Alternatives {
			systemNames[i] = a.SystemName
			assert.Equal(t, "container_isolation", a.AlternativeType,
				"all safety posture alternatives must be container_isolation type")
		}
		assert.Contains(t, systemNames, "SWE-Agent / OpenHands")
		assert.Contains(t, systemNames, "Aider")
	})

	t.Run("Scenario_GroundingPrinciplesAreValidTableOneIds", func(t *testing.T) {
		// Given all grounding principles must be drawn from Table 1 of the paper,
		// When all four profiles' GroundingPrinciples are inspected,
		// Then every listed principle ID must exist in the DesignPrincipleRegistry.
		r := NewDesignSpaceQuestionRegistry()
		pr := NewDesignPrincipleRegistry()
		for _, q := range r.AllQuestions() {
			for _, pid := range q.GroundingPrinciples {
				_, ok := pr.Profile(pid)
				assert.True(t, ok,
					"question %q cites unknown principle %q", q.Slug, pid)
			}
		}
	})

	t.Run("Scenario_TotalAlternativeCountIsSeven", func(t *testing.T) {
		// Given §3.1 names the following alternatives across the four questions:
		//   Q1 (where reasoning): Devin, LangGraph (2)
		//   Q2 (how many engines): mode-specific engine pattern (1)
		//   Q3 (safety posture): SWE-Agent/OpenHands, Aider (2)
		//   Q4 (binding resource): compute-budget, working-memory (2)
		// Total = 7
		// When TotalAlternativeCount() is called,
		// Then it must equal 7.
		r := NewDesignSpaceQuestionRegistry()
		assert.Equal(t, 7, r.TotalAlternativeCount(),
			"§3.1 documents 7 named alternative approaches across four questions")
	})

	t.Run("Scenario_QuestionOrderIsStrictlyAscendingOneThroughFour", func(t *testing.T) {
		// Given §3.1 introduces the four questions in a fixed narrative order,
		// When all profiles are retrieved and their QuestionOrder values collected,
		// Then the orders must form the strictly ascending sequence 1, 2, 3, 4.
		r := NewDesignSpaceQuestionRegistry()
		all := r.AllQuestions()
		orders := make([]int, len(all))
		for i, q := range all {
			orders[i] = q.QuestionOrder
		}
		for i := 0; i < len(orders); i++ {
			assert.Equal(t, i+1, orders[i],
				"question at index %d must have QuestionOrder %d", i, i+1)
		}
	})

	t.Run("Scenario_ScaffoldingReasoningAlternativesNamesDevinAndLangGraph", func(t *testing.T) {
		// Given §3.1 cites Devin (planning in scaffolding) and LangGraph
		// (state-graph routing) as alternatives for question 1,
		// When AlternativesByType("scaffolding_reasoning") is called,
		// Then exactly Devin and LangGraph must be returned.
		r := NewDesignSpaceQuestionRegistry()
		alts := r.AlternativesByType("scaffolding_reasoning")
		require.Len(t, alts, 2)
		names := []string{alts[0].SystemName, alts[1].SystemName}
		assert.Contains(t, names, "Devin")
		assert.Contains(t, names, "LangGraph")
	})

	t.Run("Scenario_BindingResourceConstraintCitesCompactionPipelineEvidence", func(t *testing.T) {
		// Given §3.1 states five context-reduction strategies execute before every
		// model call, citing query.ts:365-453 as the source,
		// When the 'binding_resource_constraint' profile is retrieved,
		// Then the ClaudeAnswer must mention both the five-layer structure and
		// the source line range, and SourceEvidence must reference compact.ts.
		r := NewDesignSpaceQuestionRegistry()
		p, ok := r.FindDesignSpaceQuestionBySlug(DesignQuestionBindingResourceConstraint)
		require.True(t, ok)
		assert.Contains(t, p.ClaudeAnswer, "query.ts:365-453")
		found := false
		for _, e := range p.SourceEvidence {
			if strings.Contains(e, "compact.ts") {
				found = true
				break
			}
		}
		assert.True(t, found, "SourceEvidence must include compact.ts for the auto-compact shaper")
	})
}
