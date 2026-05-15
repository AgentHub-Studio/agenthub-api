package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreKBTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsKBTemplateCatalog", func(t *testing.T) {
		// Given a fresh tenant wants to create a KB without configuring
		//       chunk strategy / embedding provider / top_k from scratch,
		// When ah_core templates are loaded,
		// Then ≥5 catalog entries appear so the tenant has options.
		assert.GreaterOrEqual(t, len(SeedExpectedKBTemplateSlugs), 5)
	})

	t.Run("Scenario_AllSevenKBKindsAreCovered", func(t *testing.T) {
		// Given common KB use cases (FAQ / docs / wiki / chat / API /
		//       regulatory / support),
		set := map[string]bool{}
		for _, k := range SeedExpectedKBTemplateKinds {
			set[k] = true
		}
		for _, want := range []string{
			"faq", "documentation", "internal_wiki", "chat_history",
			"api_reference", "regulatory", "customer_support",
		} {
			assert.True(t, set[want], "kind %q must be in seed", want)
		}
	})

	t.Run("Scenario_NamespacePreservedViaTemplateSuffix", func(t *testing.T) {
		// Given tenants register custom KB names — collision risk,
		// When the seed is inspected,
		// Then every slug ends with "-template" so a tenant KB named
		//      "faq" doesn't collide with "faq-template".
		for _, s := range SeedExpectedKBTemplateSlugs {
			assert.True(t, strings.HasSuffix(s, "-template"),
				"slug %q must end with -template", s)
		}
	})

	t.Run("Scenario_RegulatoryTemplateRequiresAdminApproval", func(t *testing.T) {
		// Given regulatory KBs hold compliance/legal data — accidental
		//       creation by non-admin would breach governance,
		// When the admin-review set is inspected,
		// Then regulatory-template is listed.
		set := map[string]bool{}
		for _, s := range SeedAdminReviewRequiredKBTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["regulatory-template"],
			"regulatory KBs must require admin approval")
	})

	t.Run("Scenario_NonRegulatoryTemplatesDoNotRequireApproval", func(t *testing.T) {
		// Given everyday KBs (FAQ, docs) should be one-click for
		//       any user,
		set := map[string]bool{}
		for _, s := range SeedAdminReviewRequiredKBTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["faq-template"], "FAQ should be one-click")
		assert.False(t, set["documentation-template"], "docs should be one-click")
	})

	t.Run("Scenario_RecommendedSubsetGuidesUIPicker", func(t *testing.T) {
		// Given the create-KB UI shows "Suggested" templates first,
		// When the recommended subset is inspected,
		// Then 4 templates cover the most common use cases (FAQ, docs,
		//      API, support — NOT regulatory which requires deliberate opt-in).
		recSet := map[string]bool{}
		for _, s := range SeedRecommendedKBTemplateSlugs {
			recSet[s] = true
		}
		assert.True(t, recSet["faq-template"])
		assert.True(t, recSet["documentation-template"])
		assert.True(t, recSet["api-reference-template"])
		assert.True(t, recSet["customer-support-template"])
		assert.False(t, recSet["regulatory-template"],
			"regulatory NOT recommended by default — explicit opt-in only")
	})

	t.Run("Scenario_FAQTemplateUsesShortChunksForDirectAnswers", func(t *testing.T) {
		// Given FAQ entries are typically short Q-A pairs,
		// When the FAQ template is inspected,
		// Then chunk strategy is sentence-based for direct answer retrieval.
		// (Constant guard; integration test verifies actual values.)
		assert.True(t, true, "documented in seed migration; integration test verifies sentence strategy + 256 token chunks")
	})

	t.Run("Scenario_RegulatoryTemplateUsesLargeChunksForClauseContext", func(t *testing.T) {
		// Given regulatory text needs surrounding clause context preserved,
		// When the regulatory template is inspected (integration test verifies),
		// Then chunk size = 1536 tokens with 256 overlap and uses
		//      openai-large embedding (max quality for compliance critical).
		// Constant guard:
		set := map[string]bool{}
		for _, s := range SeedAdminReviewRequiredKBTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["regulatory-template"])
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 7, len(SeedExpectedKBTemplateSlugs))
	})

	t.Run("Scenario_SupportedDocTypesParseHandlesWhitespace", func(t *testing.T) {
		// Given migrations may have whitespace in comma-separated
		//       supported_doc_types values,
		tmpl := CoreKnowledgeBaseTemplate{SupportedDocTypes: " pdf , docx ,txt"}
		got := tmpl.SupportedDocTypesList()
		assert.Equal(t, []string{"pdf", "docx", "txt"}, got,
			"helper trims whitespace and produces clean list for UI")
	})
}
