package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreKBTemplate_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedKBTemplateSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreKBTemplate_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, s := range SeedExpectedKBTemplateSlugs {
		assert.NotEmpty(t, s, "slug at %d must be non-empty", i)
	}
}

func TestCoreKBTemplate_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	assert.Equal(t, 7, len(SeedExpectedKBTemplateSlugs),
		"7 KB templates in canonical seed")
}

func TestCoreKBTemplate_AllSlugsEndWithTemplate(t *testing.T) {
	for _, s := range SeedExpectedKBTemplateSlugs {
		assert.True(t, strings.HasSuffix(s, "-template"),
			"slug %q must end with -template suffix (namespace contract)", s)
	}
}

func TestCoreKBTemplate_KindsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedKBTemplateKinds {
		assert.False(t, seen[k], "duplicate kind %q", k)
		seen[k] = true
	}
}

func TestCoreKBTemplate_KindsCanonicalCount(t *testing.T) {
	assert.Equal(t, 7, len(SeedExpectedKBTemplateKinds),
		"7 kinds: faq/docs/wiki/chat/api/regulatory/support")
}

func TestCoreKBTemplate_ChunkStrategiesAreClosed(t *testing.T) {
	allowed := map[string]bool{}
	for _, s := range SeedExpectedKBTemplateChunkStrategies {
		allowed[s] = true
	}
	for _, want := range []string{"fixed_size", "semantic", "sentence", "paragraph"} {
		assert.True(t, allowed[want])
	}
	assert.Equal(t, 4, len(SeedExpectedKBTemplateChunkStrategies))
}

func TestCoreKBTemplate_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedKBTemplateSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedKBTemplateSlugs {
		assert.True(t, seedSet[r], "recommended %q must be in seed", r)
	}
}

func TestCoreKBTemplate_AdminReviewSlugsAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedKBTemplateSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedAdminReviewRequiredKBTemplateSlugs {
		assert.True(t, seedSet[r], "admin-review %q must be in seed", r)
	}
}

func TestCoreKBTemplate_AdminReviewIsRegulatoryOnly(t *testing.T) {
	// Only regulatory KB requires admin review by default.
	assert.Equal(t, []string{"regulatory-template"}, SeedAdminReviewRequiredKBTemplateSlugs,
		"only regulatory needs admin review by default")
}

func TestCoreKBTemplate_SupportedDocTypesList_Parses(t *testing.T) {
	tmpl := CoreKnowledgeBaseTemplate{SupportedDocTypes: "pdf,docx, txt , md"}
	got := tmpl.SupportedDocTypesList()
	assert.Equal(t, []string{"pdf", "docx", "txt", "md"}, got,
		"comma-separated parsed and trimmed")
}

func TestCoreKBTemplate_SupportedDocTypesList_HandlesEmpty(t *testing.T) {
	tmpl := CoreKnowledgeBaseTemplate{SupportedDocTypes: ""}
	assert.Empty(t, tmpl.SupportedDocTypesList())
}
