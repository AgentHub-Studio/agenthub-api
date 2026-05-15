package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FEAT029 — §3.1 Design Space Question Registry unit tests
// arXiv:2604.14228v1 §3.1 "Design Questions and Running Example"

func TestFEAT029_SeedCount(t *testing.T) {
	assert.Equal(t, 4, SeedDesignSpaceQuestionCount,
		"§3.1 defines exactly four recurring design questions")
}

func TestFEAT029_SeedSlugsLength(t *testing.T) {
	assert.Len(t, SeedDesignSpaceQuestionSlugs, SeedDesignSpaceQuestionCount,
		"SeedDesignSpaceQuestionSlugs must have exactly SeedDesignSpaceQuestionCount entries")
}

func TestFEAT029_SeedSlugsContainAllFour(t *testing.T) {
	slugSet := make(map[DesignSpaceQuestionSlug]bool)
	for _, s := range SeedDesignSpaceQuestionSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[DesignQuestionWhereReasoningLives])
	assert.True(t, slugSet[DesignQuestionHowManyEngines])
	assert.True(t, slugSet[DesignQuestionDefaultSafetyPosture])
	assert.True(t, slugSet[DesignQuestionBindingResourceConstraint])
}

func TestFEAT029_RegistryConstructs(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	require.NotNil(t, r)
}

func TestFEAT029_AllQuestionsReturnsFour(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	all := r.AllQuestions()
	assert.Len(t, all, SeedDesignSpaceQuestionCount)
}

func TestFEAT029_AllQuestionsIsDefensiveCopy(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	a := r.AllQuestions()
	b := r.AllQuestions()
	// Mutating one copy must not affect the registry
	a[0].QuestionText = "mutated"
	b2 := r.AllQuestions()
	assert.NotEqual(t, "mutated", b2[0].QuestionText)
	_ = b
}

func TestFEAT029_FindBySlugSuccess(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, slug := range SeedDesignSpaceQuestionSlugs {
		p, ok := r.FindDesignSpaceQuestionBySlug(slug)
		assert.True(t, ok, "slug %q must be found", slug)
		require.NotNil(t, p)
		assert.Equal(t, slug, p.Slug)
	}
}

func TestFEAT029_FindBySlugUnknown(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	p, ok := r.FindDesignSpaceQuestionBySlug("nonexistent_slug")
	assert.False(t, ok)
	assert.Nil(t, p)
}

func TestFEAT029_IsValidSlugKnown(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, slug := range SeedDesignSpaceQuestionSlugs {
		assert.True(t, r.IsValidSlug(slug), "slug %q must be valid", slug)
	}
}

func TestFEAT029_IsValidSlugUnknown(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	assert.False(t, r.IsValidSlug("bogus"))
}

func TestFEAT029_QuestionOrdersAreOneToFour(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	orders := make(map[int]bool)
	for _, q := range r.AllQuestions() {
		orders[q.QuestionOrder] = true
	}
	for i := 1; i <= SeedDesignSpaceQuestionCount; i++ {
		assert.True(t, orders[i], "QuestionOrder %d must exist", i)
	}
}

func TestFEAT029_QuestionByOrderSuccess(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for i := 1; i <= SeedDesignSpaceQuestionCount; i++ {
		p, ok := r.QuestionByOrder(i)
		assert.True(t, ok, "order %d must be found", i)
		require.NotNil(t, p)
		assert.Equal(t, i, p.QuestionOrder)
	}
}

func TestFEAT029_QuestionByOrderOutOfRange(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	p, ok := r.QuestionByOrder(0)
	assert.False(t, ok)
	assert.Nil(t, p)

	p, ok = r.QuestionByOrder(99)
	assert.False(t, ok)
	assert.Nil(t, p)
}

func TestFEAT029_AllQuestionsHavePDFSection31(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, q := range r.AllQuestions() {
		assert.Equal(t, "3.1", q.PDFSection,
			"question %q must cite §3.1", q.Slug)
	}
}

func TestFEAT029_AllQuestionsHaveNonEmptyText(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, q := range r.AllQuestions() {
		assert.NotEmpty(t, q.QuestionText, "question %q must have QuestionText", q.Slug)
	}
}

func TestFEAT029_AllQuestionsHaveClaudeAnswer(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, q := range r.AllQuestions() {
		assert.NotEmpty(t, q.ClaudeAnswer, "question %q must have ClaudeAnswer", q.Slug)
	}
}

func TestFEAT029_AllQuestionsHaveSourceEvidence(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, q := range r.AllQuestions() {
		assert.NotEmpty(t, q.SourceEvidence,
			"question %q must have at least one SourceEvidence entry", q.Slug)
	}
}

func TestFEAT029_AllQuestionsHaveGroundingPrinciples(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, q := range r.AllQuestions() {
		assert.NotEmpty(t, q.GroundingPrinciples,
			"question %q must be grounded in at least one Table 1 principle", q.Slug)
	}
}

func TestFEAT029_AllQuestionsHaveAlternatives(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, q := range r.AllQuestions() {
		assert.NotEmpty(t, q.Alternatives,
			"question %q must document at least one alternative approach", q.Slug)
	}
}

func TestFEAT029_AllQuestionsHaveTradeoffAccepted(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	for _, q := range r.AllQuestions() {
		assert.NotEmpty(t, q.TradeoffAccepted,
			"question %q must describe the tradeoff accepted", q.Slug)
	}
}

func TestFEAT029_TotalAlternativeCount(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	count := r.TotalAlternativeCount()
	// Questions 1 has 2, question 2 has 1, question 3 has 2, question 4 has 2 = 7 total
	assert.Equal(t, 7, count, "§3.1 documents 7 named alternative approaches across the four questions")
}

func TestFEAT029_AllAlternativesFlat(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	alts := r.AllAlternatives()
	assert.Len(t, alts, r.TotalAlternativeCount())
}

func TestFEAT029_AlternativesByTypeScaffoldingReasoning(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	alts := r.AlternativesByType("scaffolding_reasoning")
	// Devin + LangGraph (question 1)
	assert.Len(t, alts, 2, "two scaffolding_reasoning alternatives (Devin, LangGraph)")
	names := make([]string, len(alts))
	for i, a := range alts {
		names[i] = a.SystemName
	}
	assert.Contains(t, names, "Devin")
	assert.Contains(t, names, "LangGraph")
}

func TestFEAT029_AlternativesByTypeAlternateResource(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	alts := r.AlternativesByType("alternate_resource")
	// compute-budget + working-memory (question 4)
	assert.Len(t, alts, 2, "two alternate_resource alternatives")
}

func TestFEAT029_AlternativesByTypeContainerIsolation(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	alts := r.AlternativesByType("container_isolation")
	// SWE-Agent/OpenHands + Aider (question 3)
	assert.Len(t, alts, 2)
}

func TestFEAT029_QuestionsGroundedInMinimalScaffolding(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	qs := r.QuestionsGroundedIn(PrincipleMinimalScaffoldingMaximalHarness)
	// Questions 1 (where reasoning) and 2 (how many engines) both cite this principle
	assert.GreaterOrEqual(t, len(qs), 2,
		"PrincipleMinimalScaffoldingMaximalHarness grounds questions 1 and 2")
}

func TestFEAT029_QuestionsGroundedInContextAsScarceResource(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	qs := r.QuestionsGroundedIn(PrincipleContextAsScarceResource)
	assert.GreaterOrEqual(t, len(qs), 1,
		"binding resource constraint question cites PrincipleContextAsScarceResource")
}

func TestFEAT029_QuestionsGroundedInUnusedPrinciple(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	qs := r.QuestionsGroundedIn("nonexistent_principle")
	assert.Empty(t, qs)
}

func TestFEAT029_WhereReasoningLivesProfile(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	p, ok := r.FindDesignSpaceQuestionBySlug(DesignQuestionWhereReasoningLives)
	require.True(t, ok)
	assert.Equal(t, 1, p.QuestionOrder)
	assert.Contains(t, p.ClaudeAnswer, "1.6%")
	assert.Contains(t, p.ClaudeAnswer, "98.4%")
	assert.Contains(t, p.SourceEvidence[0], "query.ts")
}

func TestFEAT029_HowManyEnginesProfile(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	p, ok := r.FindDesignSpaceQuestionBySlug(DesignQuestionHowManyEngines)
	require.True(t, ok)
	assert.Equal(t, 2, p.QuestionOrder)
	assert.Contains(t, p.ClaudeAnswer, "queryLoop()")
}

func TestFEAT029_DefaultSafetyPostureProfile(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	p, ok := r.FindDesignSpaceQuestionBySlug(DesignQuestionDefaultSafetyPosture)
	require.True(t, ok)
	assert.Equal(t, 3, p.QuestionOrder)
	assert.Contains(t, p.ClaudeAnswer, "deny-first")
	// The 93% approval-rate finding from Hughes,2026 appears in TradeoffAccepted
	assert.Contains(t, p.TradeoffAccepted, "93%")
}

func TestFEAT029_BindingResourceConstraintProfile(t *testing.T) {
	r := NewDesignSpaceQuestionRegistry()
	p, ok := r.FindDesignSpaceQuestionBySlug(DesignQuestionBindingResourceConstraint)
	require.True(t, ok)
	assert.Equal(t, 4, p.QuestionOrder)
	assert.Contains(t, p.ClaudeAnswer, "200K")
	assert.Contains(t, p.ClaudeAnswer, "query.ts:365-453")
}
