package agentic

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReview_CategoryEnumIsBounded(t *testing.T) {
	for _, c := range AllReviewCategories() {
		assert.True(t, IsValidReviewCategory(c))
	}
	assert.False(t, IsValidReviewCategory(ReviewCategory("unknown")))
	assert.False(t, IsValidReviewCategory(""))
}

func TestReview_AllCategoriesCount(t *testing.T) {
	// 6 categories: security/correctness/style/performance/test_coverage/policy_compliance.
	assert.Equal(t, 6, len(AllReviewCategories()))
}

func TestReview_SeverityEnumIsBounded(t *testing.T) {
	for _, s := range []ReviewSeverity{
		ReviewSeverityInfo, ReviewSeverityWarn, ReviewSeverityCritical,
	} {
		assert.True(t, IsValidReviewSeverity(s))
	}
	assert.False(t, IsValidReviewSeverity(ReviewSeverity("urgent")))
}

func TestReview_NewSetsRequiredFields(t *testing.T) {
	g := NewReviewGuidance("PR-42", "code_diff", "auth fix")
	assert.Equal(t, "PR-42", g.ArtifactID)
	assert.Equal(t, "code_diff", g.ArtifactKind)
	assert.Equal(t, "auth fix", g.Summary)
	assert.False(t, g.CreatedAt.IsZero())
	assert.Empty(t, g.FocusPoints)
}

func TestReview_AddFocus_AppendsAndChains(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddFocus(ReviewFocusPoint{
			Category: ReviewCategorySecurity, Location: "a.go:1",
			Severity: ReviewSeverityCritical, Reason: "auth",
		}).
		AddFocus(ReviewFocusPoint{
			Category: ReviewCategoryStyle, Location: "b.go:2",
			Severity: ReviewSeverityInfo, Reason: "rename",
		})
	assert.Len(t, g.FocusPoints, 2)
}

func TestReview_AddFocus_RejectsInvalidCategorySilently(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddFocus(ReviewFocusPoint{
			Category: ReviewCategory("typo"), Severity: ReviewSeverityInfo, Reason: "x",
		})
	assert.Empty(t, g.FocusPoints)
}

func TestReview_AddFocus_RejectsInvalidSeveritySilently(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddFocus(ReviewFocusPoint{
			Category: ReviewCategorySecurity, Severity: ReviewSeverity("urgent"), Reason: "x",
		})
	assert.Empty(t, g.FocusPoints)
}

func TestReview_AddFocus_TruncatesLongReason(t *testing.T) {
	long := strings.Repeat("x", 800)
	g := NewReviewGuidance("x", "y", "z").
		AddFocus(ReviewFocusPoint{
			Category: ReviewCategorySecurity, Severity: ReviewSeverityCritical, Reason: long,
		})
	assert.Len(t, g.FocusPoints, 1)
	assert.Equal(t, 500, len(g.FocusPoints[0].Reason))
	assert.Equal(t, "...", g.FocusPoints[0].Reason[497:])
}

func TestReview_AddFocus_PopulatesAtIfZero(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddFocus(ReviewFocusPoint{
			Category: ReviewCategorySecurity, Severity: ReviewSeverityCritical, Reason: "x",
		})
	assert.False(t, g.FocusPoints[0].At.IsZero())
}

func TestReview_AddSecurityFocus_DefaultsToCritical(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddSecurityFocus("auth.go:42", "JWT signature changed", "verify alg matches issuer config")
	assert.Equal(t, ReviewCategorySecurity, g.FocusPoints[0].Category)
	assert.Equal(t, ReviewSeverityCritical, g.FocusPoints[0].Severity)
	assert.Equal(t, "verify alg matches issuer config", g.FocusPoints[0].RecommendedAction)
}

func TestReview_AddPolicyComplianceFocus_DefaultsToCritical(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddPolicyComplianceFocus("audit.go:10", "retention changed", "verify GDPR floor")
	assert.Equal(t, ReviewSeverityCritical, g.FocusPoints[0].Severity)
}

func TestReview_AddCorrectnessFocus_DefaultsToWarn(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddCorrectnessFocus("loop.go:5", "off-by-one risk", "check loop bounds")
	assert.Equal(t, ReviewSeverityWarn, g.FocusPoints[0].Severity)
}

func TestReview_AddPerformanceFocus_DefaultsToWarn(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddPerformanceFocus("query.go:3", "N+1 risk", "add index or batch")
	assert.Equal(t, ReviewSeverityWarn, g.FocusPoints[0].Severity)
}

func TestReview_AddTestCoverageFocus_DefaultsToInfo(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddTestCoverageFocus("auth_test.go", "test removed", "verify still covered")
	assert.Equal(t, ReviewSeverityInfo, g.FocusPoints[0].Severity)
}

func TestReview_AddStyleFocus_DefaultsToInfo(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddStyleFocus("naming.go:1", "rename", "ok")
	assert.Equal(t, ReviewSeverityInfo, g.FocusPoints[0].Severity)
}

func TestReview_SortBySeverity(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z")
	earliest := time.Now()
	g.AddFocus(ReviewFocusPoint{
		Category: ReviewCategoryStyle, Severity: ReviewSeverityInfo, Reason: "i", At: earliest,
	})
	g.AddFocus(ReviewFocusPoint{
		Category: ReviewCategorySecurity, Severity: ReviewSeverityCritical, Reason: "c", At: earliest.Add(time.Second),
	})
	g.AddFocus(ReviewFocusPoint{
		Category: ReviewCategoryPerformance, Severity: ReviewSeverityWarn, Reason: "w", At: earliest.Add(2 * time.Second),
	})
	g.SortBySeverity()
	assert.Equal(t, ReviewSeverityCritical, g.FocusPoints[0].Severity)
	assert.Equal(t, ReviewSeverityWarn, g.FocusPoints[1].Severity)
	assert.Equal(t, ReviewSeverityInfo, g.FocusPoints[2].Severity)
}

func TestReview_HasCriticalFocus(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").AddStyleFocus("x", "x", "")
	assert.False(t, g.HasCriticalFocus())
	g.AddSecurityFocus("a", "auth", "")
	assert.True(t, g.HasCriticalFocus())
}

func TestReview_CountByCategory_AlwaysAllCategories(t *testing.T) {
	g := NewReviewGuidance("x", "y", "z").
		AddSecurityFocus("a", "x", "").
		AddSecurityFocus("b", "x", "")
	hist := g.CountByCategory()
	for _, c := range AllReviewCategories() {
		_, ok := hist[c]
		assert.True(t, ok, "category %q must always exist", c)
	}
	assert.Equal(t, 2, hist[ReviewCategorySecurity])
	assert.Equal(t, 0, hist[ReviewCategoryStyle])
}

func TestReview_PlainText_Format(t *testing.T) {
	g := NewReviewGuidance("PR-42", "code_diff", "auth fix").
		AddSecurityFocus("a.go:1", "x", "")
	out := g.PlainText()
	assert.Contains(t, out, "[REVIEW]")
	assert.Contains(t, out, "code_diff")
	assert.Contains(t, out, "PR-42")
	assert.Contains(t, out, "1 focus points, 1 critical")
}

func TestReview_Markdown_Format(t *testing.T) {
	g := NewReviewGuidance("PR-42", "code_diff", "auth fix").
		AddSecurityFocus("auth.go:42", "JWT change", "verify alg")
	out := g.Markdown()
	assert.Contains(t, out, "## Review Guidance:")
	assert.Contains(t, out, "code_diff")
	assert.Contains(t, out, "`PR-42`")
	assert.Contains(t, out, "[critical]")
	assert.Contains(t, out, "[security]")
	assert.Contains(t, out, "`auth.go:42`")
	assert.Contains(t, out, "Action: verify alg")
}

func TestReview_Markdown_NoFocusPointsMessage(t *testing.T) {
	g := NewReviewGuidance("PR-1", "doc_edit", "typo fix")
	out := g.Markdown()
	assert.Contains(t, out, "No specific focus points")
}

func TestReview_JSON_RoundTrip(t *testing.T) {
	g := NewReviewGuidance("PR-X", "config_change", "policy update").
		AddPolicyComplianceFocus("config/policy.yaml", "retention 365→30", "verify GDPR")
	jsonStr, err := g.JSON()
	assert.NoError(t, err)
	var rt ReviewGuidance
	assert.NoError(t, json.Unmarshal([]byte(jsonStr), &rt))
	assert.Equal(t, "PR-X", rt.ArtifactID)
	assert.Len(t, rt.FocusPoints, 1)
}
