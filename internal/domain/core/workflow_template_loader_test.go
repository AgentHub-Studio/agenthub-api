package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreWorkflowTemplate_SlugsNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedWorkflowTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreWorkflowTemplate_SlugsCanonicalCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedWorkflowTemplateSlugs))
}

func TestCoreWorkflowTemplate_KindsCanonicalCount(t *testing.T) {
	// 7 kinds: qa/extraction/review/onboarding/research/monitoring/compliance.
	assert.Equal(t, 7, len(SeedExpectedWorkflowTemplateKinds))
}

func TestCoreWorkflowTemplate_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedWorkflowTemplateSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedWorkflowTemplateSlugs {
		assert.True(t, seedSet[r])
	}
}

func TestCoreWorkflowTemplate_HumanCheckpointAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedWorkflowTemplateSlugs {
		seedSet[s] = true
	}
	for _, h := range SeedHumanCheckpointWorkflowTemplateSlugs {
		assert.True(t, seedSet[h])
	}
}

func TestCoreWorkflowTemplate_ComplianceWorkflowRequiresHumanCheckpoint(t *testing.T) {
	checkpointSet := map[string]bool{}
	for _, s := range SeedHumanCheckpointWorkflowTemplateSlugs {
		checkpointSet[s] = true
	}
	assert.True(t, checkpointSet["compliance-export"],
		"compliance-export must require human checkpoint (audit data export)")
}

func TestCoreWorkflowTemplate_RequiresSkillsList_Parses(t *testing.T) {
	tmpl := CoreWorkflowTemplate{RequiresSkills: "web_search, knowledge_base_search"}
	got := tmpl.RequiresSkillsList()
	assert.Equal(t, []string{"web_search", "knowledge_base_search"}, got)
}

func TestCoreWorkflowTemplate_SlugsKebabCase(t *testing.T) {
	for _, s := range SeedExpectedWorkflowTemplateSlugs {
		assert.False(t, strings.Contains(s, "_"))
	}
}
