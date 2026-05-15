package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedContextAssemblySource_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 9, SeedExpectedContextAssemblySourceRowCount)
}

func TestSeedContextAssemblySource_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedContextAssemblySourceRowCount, len(SeedExpectedContextAssemblySourceSlugs))
}

func TestSeedContextAssemblySource_SlugsContainSystemPrompt(t *testing.T) {
	assert.Contains(t, SeedExpectedContextAssemblySourceSlugs, "system_prompt")
}

func TestSeedContextAssemblySource_SlugsContainAutoMemory(t *testing.T) {
	assert.Contains(t, SeedExpectedContextAssemblySourceSlugs, "auto_memory")
}

func TestSeedContextAssemblySource_SlugsContainCompactSummaries(t *testing.T) {
	assert.Contains(t, SeedExpectedContextAssemblySourceSlugs, "compact_summaries")
}

func TestSeedContextAssemblySource_SixDomainsNamed(t *testing.T) {
	assert.Equal(t, 6, len(SeedContextAssemblySourceDomains))
	assert.Contains(t, SeedContextAssemblySourceDomains, "prompt_construction")
	assert.Contains(t, SeedContextAssemblySourceDomains, "conversation")
	assert.Contains(t, SeedContextAssemblySourceDomains, "memory")
}

func TestSeedContextAssemblySource_AlwaysIncludedHasSixSlugs(t *testing.T) {
	assert.Equal(t, 6, len(SeedContextAssemblyAlwaysIncludedSlugs))
}

func TestSeedContextAssemblySource_AsyncHasTwoSlugs(t *testing.T) {
	assert.Equal(t, 2, len(SeedContextAssemblyAsyncSlugs))
	assert.Contains(t, SeedContextAssemblyAsyncSlugs, "auto_memory")
	assert.Contains(t, SeedContextAssemblyAsyncSlugs, "tool_metadata")
}

func TestSeedContextAssemblySource_MemoizedHasTwoSlugs(t *testing.T) {
	assert.Equal(t, 2, len(SeedContextAssemblyMemoizedSlugs))
}

func TestSeedContextAssemblySource_AlwaysIncludedIsSubsetOfAll(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedContextAssemblySourceSlugs {
		all[s] = true
	}
	for _, s := range SeedContextAssemblyAlwaysIncludedSlugs {
		assert.True(t, all[s], "always-included slug %q not in canonical list", s)
	}
}
