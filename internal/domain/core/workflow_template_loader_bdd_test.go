package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreWorkflowTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsWorkflowLibrary", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(SeedExpectedWorkflowTemplateSlugs), 6,
			"fresh tenant must have ≥6 ready workflows")
	})

	t.Run("Scenario_AllSevenKindsCovered", func(t *testing.T) {
		set := map[string]bool{}
		for _, k := range SeedExpectedWorkflowTemplateKinds {
			set[k] = true
		}
		for _, want := range []string{
			"qa", "extraction", "review", "onboarding",
			"research", "monitoring", "compliance",
		} {
			assert.True(t, set[want], "kind %q must exist", want)
		}
	})

	t.Run("Scenario_SafeCommonWorkflowsAreRecommended", func(t *testing.T) {
		// Given the 4 most common one-click workflows (FAQ, doc-summary,
		//       code-review, research-brief),
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedWorkflowTemplateSlugs {
			recSet[r] = true
		}
		for _, want := range []string{
			"faq-answer", "document-summary", "code-review", "research-brief",
		} {
			assert.True(t, recSet[want], "%q must be recommended", want)
		}
	})

	t.Run("Scenario_ExpensiveOrSensitiveWorkflowsGateOnHumanCheckpoint", func(t *testing.T) {
		// Given workflows touching production data / financial / compliance
		//       MUST gate via HUMAN-004 understanding checkpoint,
		set := map[string]bool{}
		for _, s := range SeedHumanCheckpointWorkflowTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["code-review"], "code merge gates")
		assert.True(t, set["invoice-processing"], "financial data gates")
		assert.True(t, set["incident-triage"], "production-touching gates")
		assert.True(t, set["compliance-export"], "audit export gates")
	})

	t.Run("Scenario_SafeWorkflowsDoNotRequireCheckpointOverhead", func(t *testing.T) {
		// Given FAQ / doc-summary are safe one-click workflows,
		set := map[string]bool{}
		for _, s := range SeedHumanCheckpointWorkflowTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["faq-answer"], "FAQ doesn't need checkpoint")
		assert.False(t, set["document-summary"], "doc-summary doesn't need checkpoint")
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 8, len(SeedExpectedWorkflowTemplateSlugs))
	})

	t.Run("Scenario_RequiresSkillsListIsParseable", func(t *testing.T) {
		tmpl := CoreWorkflowTemplate{
			RequiresSkills: "knowledge_base_search,document_extract",
		}
		assert.Equal(t, []string{"knowledge_base_search", "document_extract"},
			tmpl.RequiresSkillsList())
	})

	t.Run("Scenario_HumanCheckpointWorkflowsCoverHighRiskCategories", func(t *testing.T) {
		// 4 workflows require checkpoint: review (code merge),
		// extraction (financial), monitoring (incident), compliance (audit).
		assert.Equal(t, 4, len(SeedHumanCheckpointWorkflowTemplateSlugs))
	})
}
