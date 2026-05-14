package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedCapabilityToolCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityToolCount, len(SeedCapabilityToolSlugs))
}

func TestSeedCapabilitySkillCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilitySkillCount, len(SeedCapabilitySkillSlugs))
}

func TestSeedCapabilityToolSlugs_ContainsWebSearch(t *testing.T) {
	assert.Contains(t, SeedCapabilityToolSlugs, "core-web-search")
}

func TestSeedCapabilityToolSlugs_ContainsWebFetch(t *testing.T) {
	assert.Contains(t, SeedCapabilityToolSlugs, "core-web-fetch")
}

func TestSeedCapabilityToolSlugs_ContainsDocSearch(t *testing.T) {
	assert.Contains(t, SeedCapabilityToolSlugs, "core-doc-search")
}

func TestSeedCapabilityToolSlugs_ContainsSubagentRun(t *testing.T) {
	assert.Contains(t, SeedCapabilityToolSlugs, "core-subagent-run")
}

func TestSeedCapabilitySkillSlugs_ContainsWebResearch(t *testing.T) {
	assert.Contains(t, SeedCapabilitySkillSlugs, "core-web-research")
}

func TestSeedCapabilitySkillSlugs_ContainsDocAnalysis(t *testing.T) {
	assert.Contains(t, SeedCapabilitySkillSlugs, "core-doc-analysis")
}

func TestSeedCapabilitySkillSlugs_ContainsTaskWorkflow(t *testing.T) {
	assert.Contains(t, SeedCapabilitySkillSlugs, "core-task-workflow")
}

func TestSeedCapabilityToolSlugs_AllUseCorePrfix(t *testing.T) {
	for _, slug := range SeedCapabilityToolSlugs {
		assert.True(t, strings.HasPrefix(slug, "core-"), "slug %q must start with core-", slug)
	}
}

func TestSeedCapabilitySkillSlugs_AllUseCorePrefix(t *testing.T) {
	for _, slug := range SeedCapabilitySkillSlugs {
		assert.True(t, strings.HasPrefix(slug, "core-"), "slug %q must start with core-", slug)
	}
}

func TestSeedDocumentSearchToolSlug_InCapabilityToolSlugs(t *testing.T) {
	found := false
	for _, slug := range SeedCapabilityToolSlugs {
		if slug == SeedDocumentSearchToolSlug {
			found = true
			break
		}
	}
	assert.True(t, found, "SeedDocumentSearchToolSlug must be in SeedCapabilityToolSlugs")
}

func TestSeedCapabilitySkillBindingCount_IsSixTotal(t *testing.T) {
	assert.Equal(t, 6, SeedCapabilitySkillBindingCount)
}

func TestSeedCapabilitySkillBindingCount_MatchesSumOfPerSkillCounts(t *testing.T) {
	perSkill := len(SeedWebResearchToolSlugs) + len(SeedDocAnalysisToolSlugs) + len(SeedTaskWorkflowToolSlugs)
	assert.Equal(t, SeedCapabilitySkillBindingCount, perSkill)
}

func TestSeedWebResearchToolSlugs_BothInCapabilityTools(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedCapabilityToolSlugs {
		all[s] = true
	}
	for _, s := range SeedWebResearchToolSlugs {
		assert.True(t, all[s], "web-research tool %q not in capability tool list", s)
	}
}

func TestSeedDocAnalysisToolSlugs_BothInCapabilityTools(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedCapabilityToolSlugs {
		all[s] = true
	}
	for _, s := range SeedDocAnalysisToolSlugs {
		assert.True(t, all[s], "doc-analysis tool %q not in capability tool list", s)
	}
}

func TestSeedTaskWorkflowToolSlugs_BothInCapabilityTools(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedCapabilityToolSlugs {
		all[s] = true
	}
	for _, s := range SeedTaskWorkflowToolSlugs {
		assert.True(t, all[s], "task-workflow tool %q not in capability tool list", s)
	}
}

func TestSeedCapabilityToolTypes_ContainsHTTP(t *testing.T) {
	assert.Contains(t, SeedCapabilityToolTypes, "HTTP")
}

func TestSeedCapabilityToolTypes_ContainsDocumentSearch(t *testing.T) {
	assert.Contains(t, SeedCapabilityToolTypes, "DOCUMENT_SEARCH")
}
