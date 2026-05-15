package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HUMAN-001 — Human review guidance BDD.
//
// PDF arXiv:2604.14228v1 §11 (humans need attention direction — raw
// diffs are noise); §6.1 (agent outputs need stable structure for review).
//
// These scenarios validate the contract: bounded categories, severity-
// ordered display, multi-format rendering, attention-directing semantics.

func TestBDD_HumanReviewGuidance(t *testing.T) {

	t.Run("Scenario_AgentDirectsReviewerToCriticalSecurityChange", func(t *testing.T) {
		// Given an agent fixed an auth bug,
		// When it produces review guidance,
		// Then the security focus point leads — reviewer doesn't have
		//      to scan the diff to know "look here first".
		g := NewReviewGuidance("PR-42", "code_diff",
			"Refactor JWT validation to support new issuer").
			AddSecurityFocus("internal/auth/jwt.go:42-58",
				"JWT signature validation rewritten — new code path under default config",
				"Verify alg=RS256 enforcement still applies; check fallback path")
		assert.True(t, g.HasCriticalFocus())
		assert.Equal(t, ReviewCategorySecurity, g.FocusPoints[0].Category)
	})

	t.Run("Scenario_NoFocusPointsMeansFullReviewNotZeroReview", func(t *testing.T) {
		// Given the agent produced an artifact but identified no
		//       specific high-attention areas,
		// When the markdown is rendered,
		// Then it explicitly says "review the full artifact" — reviewer
		//      knows the agent ran review and didn't find concentration
		//      points (NOT that review didn't run).
		g := NewReviewGuidance("doc-7", "doc_edit", "Fix typo in README")
		out := g.Markdown()
		assert.Contains(t, out, "No specific focus points",
			"silence is dangerous — explicit zero-focus message required")
	})

	t.Run("Scenario_BoundedCategoriesPreventDriftAcrossDeploys", func(t *testing.T) {
		// Given dashboards aggregate by category,
		// When the bounded set is inspected,
		// Then exactly 6 categories exist with stable wire strings.
		expected := map[string]bool{
			"security":          true,
			"correctness":       true,
			"style":             true,
			"performance":       true,
			"test_coverage":     true,
			"policy_compliance": true,
		}
		for _, c := range AllReviewCategories() {
			assert.True(t, expected[string(c)],
				"category %q not in stable set", c)
		}
		assert.Equal(t, 6, len(AllReviewCategories()))
	})

	t.Run("Scenario_PolicyComplianceFocusEscalatesToCritical", func(t *testing.T) {
		// Given a change touches a regulated surface (audit retention,
		//       PII handling, encryption config),
		// When AddPolicyComplianceFocus is called,
		// Then severity is critical — compliance is non-negotiable.
		g := NewReviewGuidance("config-3", "config_change", "Lower audit retention").
			AddPolicyComplianceFocus("config/audit.yaml#retention",
				"Retention lowered from 365 to 30 — verify GDPR floor",
				"Reject if tenant is GDPR-regulated")
		assert.Equal(t, ReviewSeverityCritical, g.FocusPoints[0].Severity)
		assert.True(t, g.HasCriticalFocus())
	})

	t.Run("Scenario_StyleAndCoverageDefaultToInfoSeverity", func(t *testing.T) {
		// Given style + coverage changes are usually low-attention,
		g := NewReviewGuidance("x", "y", "z").
			AddStyleFocus("naming.go", "rename for clarity", "").
			AddTestCoverageFocus("auth_test.go", "1 test removed", "verify still covered")
		assert.Equal(t, ReviewSeverityInfo, g.FocusPoints[0].Severity)
		assert.Equal(t, ReviewSeverityInfo, g.FocusPoints[1].Severity)
		assert.False(t, g.HasCriticalFocus())
	})

	t.Run("Scenario_SortBySeverityLeadsCriticalFirstForRendering", func(t *testing.T) {
		// Given a mixed-severity guidance,
		// When rendered after sort,
		// Then reviewer sees critical first.
		g := NewReviewGuidance("PR", "code_diff", "mixed").
			AddStyleFocus("a", "rename", "").
			AddPerformanceFocus("b", "N+1", "").
			AddSecurityFocus("c", "auth", "")
		g.SortBySeverity()
		assert.Equal(t, ReviewSeverityCritical, g.FocusPoints[0].Severity)
	})

	t.Run("Scenario_HistogramByCategoryGuidesSeniorReviewerRouting", func(t *testing.T) {
		// Given oncall code-review routing: "if histogram[security]>0,
		//       route to security team",
		g := NewReviewGuidance("x", "y", "z").
			AddSecurityFocus("auth.go", "x", "").
			AddSecurityFocus("session.go", "x", "").
			AddPerformanceFocus("query.go", "x", "")
		hist := g.CountByCategory()
		assert.Equal(t, 2, hist[ReviewCategorySecurity])
		assert.Equal(t, 1, hist[ReviewCategoryPerformance])
		assert.Equal(t, 0, hist[ReviewCategoryStyle],
			"unused category MUST appear with 0 — stable axes")
	})

	t.Run("Scenario_MultipleRendererFormatsForDifferentSurfaces", func(t *testing.T) {
		// Given guidance flows to: (a) terminal (PlainText),
		//       (b) chat UI / PR description (Markdown), (c) audit (JSON),
		g := NewReviewGuidance("PR-42", "code_diff", "auth fix").
			AddSecurityFocus("auth.go:42", "JWT", "verify alg")

		plain := g.PlainText()
		md := g.Markdown()
		jsonStr, err := g.JSON()
		require.NoError(t, err)

		// Plain text: single-line, [REVIEW] tag.
		assert.Contains(t, plain, "[REVIEW]")
		assert.True(t, strings.Count(plain, "\n") <= 1)

		// Markdown: multi-line, ## header, focus list.
		assert.Contains(t, md, "## Review Guidance:")
		assert.Contains(t, md, "[critical] [security]")
		assert.Contains(t, md, "Action: verify alg")

		// JSON: stable wire keys.
		assert.Contains(t, jsonStr, `"artifactId":"PR-42"`)
		assert.Contains(t, jsonStr, `"focusPoints"`)
		assert.Contains(t, jsonStr, `"category":"security"`)
	})

	t.Run("Scenario_LongReasonsAreBoundedToPreventLogSpam", func(t *testing.T) {
		// Given user-controlled / agent-generated reasons may be long,
		long := strings.Repeat("x", 1500)
		g := NewReviewGuidance("x", "y", "z").
			AddFocus(ReviewFocusPoint{
				Category: ReviewCategorySecurity, Severity: ReviewSeverityCritical, Reason: long,
			})
		assert.Equal(t, 500, len(g.FocusPoints[0].Reason),
			"reasons capped at 500 chars to keep audit logs sane")
	})

	t.Run("Scenario_PlainTextSurfacesCriticalCountForGrep", func(t *testing.T) {
		// Given oncall greps logs for "X critical",
		// When PlainText is rendered,
		// Then the critical count is in the line so grep finds it.
		g := NewReviewGuidance("x", "y", "z").
			AddSecurityFocus("a", "x", "").
			AddSecurityFocus("b", "x", "").
			AddStyleFocus("c", "x", "")
		out := g.PlainText()
		assert.Contains(t, out, "3 focus points, 2 critical")
	})

	t.Run("Scenario_ArtifactIDIsRequiredForCorrelationToTracing", func(t *testing.T) {
		// Given the audit log correlates guidance to artifact via
		//       ArtifactID (PR number, agent run ID, doc version),
		// When NewReviewGuidance is called,
		// Then ArtifactID round-trips through JSON as the join key.
		g := NewReviewGuidance("agent-run-uuid-123", "agent_definition", "creating spec agent")
		jsonStr, _ := g.JSON()
		assert.Contains(t, jsonStr, `"artifactId":"agent-run-uuid-123"`,
			"ArtifactID is the join key — must be stable on the wire")
	})

	t.Run("Scenario_InvalidCategoriesAreSilentlyDroppedNotPropagated", func(t *testing.T) {
		// Given a caller-bug typo'd a category,
		// When AddFocus is called,
		// Then nothing appears in the guidance — caller bug surfaces
		//      via tests (IsValidReviewCategory) not via log spam.
		g := NewReviewGuidance("x", "y", "z").
			AddFocus(ReviewFocusPoint{
				Category: ReviewCategory("securty"), // typo
				Severity: ReviewSeverityCritical,
				Reason:   "x",
			})
		assert.Empty(t, g.FocusPoints,
			"typo'd category dropped — guidance stays clean")
	})

	t.Run("Scenario_AllSixCategoriesHaveStableWireStrings", func(t *testing.T) {
		// Wire-stable assertion (UI / dashboards / audit bind to these strings).
		expected := []ReviewCategory{
			ReviewCategorySecurity,
			ReviewCategoryCorrectness,
			ReviewCategoryStyle,
			ReviewCategoryPerformance,
			ReviewCategoryTestCoverage,
			ReviewCategoryPolicyCompliance,
		}
		assert.Equal(t, expected, AllReviewCategories(),
			"order + values are wire contract")
	})
}
