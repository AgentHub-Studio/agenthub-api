package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for LearningAnnotationDefaultTemplate seed.
// Learning annotations are contextual tips, patterns, and anti-patterns
// surfaced to tenant admins based on audience level and kind classification.

func TestBDD_AhCoreLearningAnnotationSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesSixAnnotations", func(t *testing.T) {
		// Given a new tenant initializes the learning system
		// When the annotation catalog is loaded
		// Then exactly 6 learning annotations are seeded covering patterns,
		// anti-patterns, tips, and optimizations
		assert.Equal(t, 6, SeedExpectedLATTemplateRowCount)
		assert.Equal(t, 6, len(SeedExpectedLATTemplateSlugs))
	})

	t.Run("Scenario_ThreeAudienceLevelsAreRepresented", func(t *testing.T) {
		// Given the learning system targets beginner, intermediate, and advanced users
		// When the audience taxonomy is inspected
		// Then all three levels appear in the closed set
		assert.Equal(t, 3, len(SeedExpectedLATTemplateAudiences))
		audienceSet := map[string]bool{}
		for _, a := range SeedExpectedLATTemplateAudiences {
			audienceSet[a] = true
		}
		assert.True(t, audienceSet["beginner"])
		assert.True(t, audienceSet["intermediate"])
		assert.True(t, audienceSet["advanced"])
	})

	t.Run("Scenario_BeginnerAnnotationsTargetOnboardingPatterns", func(t *testing.T) {
		// Given new users need guidance on tool binding and system prompt basics
		// When beginner-audience annotations are listed
		// Then agent-tool-binding-pattern and no-system-prompt-antipattern are present
		assert.Equal(t, 2, len(SeedBeginnerLATTemplateSlugs))
		assert.Contains(t, SeedBeginnerLATTemplateSlugs, "agent-tool-binding-pattern")
		assert.Contains(t, SeedBeginnerLATTemplateSlugs, "no-system-prompt-antipattern")
	})

	t.Run("Scenario_AnnotationKindSetIsClosed", func(t *testing.T) {
		// Given annotation kinds form a controlled vocabulary
		// When all kind values are inspected
		// Then exactly pattern/anti_pattern/tip/optimization/knowledge_gap are valid
		assert.Equal(t, 5, len(SeedExpectedLATTemplateKinds))
		kindSet := map[string]bool{}
		for _, k := range SeedExpectedLATTemplateKinds {
			kindSet[k] = true
		}
		assert.True(t, kindSet["pattern"])
		assert.True(t, kindSet["anti_pattern"])
		assert.True(t, kindSet["tip"])
		assert.True(t, kindSet["optimization"])
		assert.True(t, kindSet["knowledge_gap"])
	})

	t.Run("Scenario_AdvancedAnnotationCoversEscalationWorkflow", func(t *testing.T) {
		// Given advanced users need guidance on complex orchestration patterns
		// When advanced-audience annotations are listed
		// Then escalation-workflow-pattern is present as the advanced-level entry
		assert.Equal(t, 1, len(SeedAdvancedLATTemplateSlugs))
		assert.Contains(t, SeedAdvancedLATTemplateSlugs, "escalation-workflow-pattern")
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// When each annotation slug is validated
		// Then every slug matches the ^[a-z0-9][a-z0-9-]*[a-z0-9]$ pattern
		for _, s := range SeedExpectedLATTemplateSlugs {
			assert.True(t, LearningAnnotationTemplateSeedSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})

	t.Run("Scenario_AudienceSubsetsPartitionFullCatalog", func(t *testing.T) {
		// Given beginner + intermediate + advanced subsets must cover all annotations
		// When the union of all audience subsets is computed
		// Then the total count equals the full catalog size
		total := len(SeedBeginnerLATTemplateSlugs) +
			len(SeedIntermediateLATTemplateSlugs) +
			len(SeedAdvancedLATTemplateSlugs)
		assert.Equal(t, SeedExpectedLATTemplateRowCount, total)
	})
}
