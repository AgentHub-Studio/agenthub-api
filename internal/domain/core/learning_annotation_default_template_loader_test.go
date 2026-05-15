package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreLATTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedLATTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedLATTemplateRowCount)
}

func TestCoreLATTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedLATTemplateSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreLATTemplate_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedLATTemplateSlugs {
		assert.True(t, LearningAnnotationTemplateSeedSlugRE.MatchString(s),
			"slug %q must match kebab regex", s)
	}
}

func TestCoreLATTemplate_AudiencePartitionExhaustive(t *testing.T) {
	combined := append([]string{}, SeedBeginnerLATTemplateSlugs...)
	combined = append(combined, SeedIntermediateLATTemplateSlugs...)
	combined = append(combined, SeedAdvancedLATTemplateSlugs...)
	assert.ElementsMatch(t, SeedExpectedLATTemplateSlugs, combined,
		"beginner ∪ intermediate ∪ advanced must equal all slugs")
}

func TestCoreLATTemplate_BeginnerCount(t *testing.T) {
	assert.Equal(t, 2, len(SeedBeginnerLATTemplateSlugs))
}

func TestCoreLATTemplate_IntermediateCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedIntermediateLATTemplateSlugs))
}

func TestCoreLATTemplate_AdvancedCount(t *testing.T) {
	assert.Equal(t, 1, len(SeedAdvancedLATTemplateSlugs))
}

func TestCoreLATTemplate_KindsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"pattern": true, "anti_pattern": true, "tip": true,
		"optimization": true, "knowledge_gap": true,
	}
	for _, k := range SeedExpectedLATTemplateKinds {
		assert.True(t, expected[k], "kind %q outside closed set", k)
	}
	assert.Equal(t, 5, len(SeedExpectedLATTemplateKinds))
}

func TestCoreLATTemplate_KindsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedLATTemplateKinds {
		assert.False(t, seen[k], "duplicate kind %q", k)
		seen[k] = true
	}
}

func TestCoreLATTemplate_AudiencesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"beginner": true, "intermediate": true, "advanced": true,
	}
	for _, a := range SeedExpectedLATTemplateAudiences {
		assert.True(t, expected[a], "audience %q outside closed set", a)
	}
	assert.Equal(t, 3, len(SeedExpectedLATTemplateAudiences))
}

func TestCoreLATTemplate_AudiencesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range SeedExpectedLATTemplateAudiences {
		assert.False(t, seen[a], "duplicate audience %q", a)
		seen[a] = true
	}
}

func TestCoreLATTemplate_AudienceSlugsAreSubsetOfAll(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedLATTemplateSlugs {
		all[s] = true
	}
	for _, s := range SeedBeginnerLATTemplateSlugs {
		assert.True(t, all[s], "beginner slug %q not in all slugs", s)
	}
	for _, s := range SeedIntermediateLATTemplateSlugs {
		assert.True(t, all[s], "intermediate slug %q not in all slugs", s)
	}
	for _, s := range SeedAdvancedLATTemplateSlugs {
		assert.True(t, all[s], "advanced slug %q not in all slugs", s)
	}
}

func TestCoreLATTemplate_AudienceGroupsAreDisjoint(t *testing.T) {
	begSet := map[string]bool{}
	for _, s := range SeedBeginnerLATTemplateSlugs {
		begSet[s] = true
	}
	for _, s := range SeedIntermediateLATTemplateSlugs {
		assert.False(t, begSet[s], "slug %q in both beginner and intermediate", s)
	}
	for _, s := range SeedAdvancedLATTemplateSlugs {
		assert.False(t, begSet[s], "slug %q in both beginner and advanced", s)
	}
}

func TestCoreLATTemplate_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedLATTemplateRowCount, len(SeedExpectedLATTemplateSlugs))
}
