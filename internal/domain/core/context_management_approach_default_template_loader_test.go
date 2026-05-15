package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedContextManagementApproach_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 5, SeedExpectedContextManagementApproachRowCount)
}

func TestSeedContextManagementApproach_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedContextManagementApproachRowCount, len(SeedExpectedContextManagementApproachSlugs))
}

func TestSeedContextManagementApproach_SlugsContainSimpleTruncation(t *testing.T) {
	assert.Contains(t, SeedExpectedContextManagementApproachSlugs, "simple_truncation")
}

func TestSeedContextManagementApproach_SlugsContainSlidingWindow(t *testing.T) {
	assert.Contains(t, SeedExpectedContextManagementApproachSlugs, "sliding_window")
}

func TestSeedContextManagementApproach_SlugsContainRAG(t *testing.T) {
	assert.Contains(t, SeedExpectedContextManagementApproachSlugs, "rag")
}

func TestSeedContextManagementApproach_SlugsContainSingleSummarization(t *testing.T) {
	assert.Contains(t, SeedExpectedContextManagementApproachSlugs, "single_summarization")
}

func TestSeedContextManagementApproach_SlugsContainGraduatedCompaction(t *testing.T) {
	assert.Contains(t, SeedExpectedContextManagementApproachSlugs, "graduated_compaction")
}

func TestSeedContextManagementApproach_AgentHubApproachIsGraduatedCompaction(t *testing.T) {
	assert.Equal(t, "graduated_compaction", SeedContextManagementAgentHubApproachSlug)
	assert.Contains(t, SeedExpectedContextManagementApproachSlugs, SeedContextManagementAgentHubApproachSlug)
}

func TestSeedContextManagementApproach_TwoCoarseApproaches(t *testing.T) {
	assert.Equal(t, 2, len(SeedContextManagementCoarseApproachSlugs))
	assert.Contains(t, SeedContextManagementCoarseApproachSlugs, "simple_truncation")
	assert.Contains(t, SeedContextManagementCoarseApproachSlugs, "single_summarization")
}

func TestSeedContextManagementApproach_FourGranularityValues(t *testing.T) {
	assert.Equal(t, 4, len(SeedContextManagementGranularityValues))
	assert.Contains(t, SeedContextManagementGranularityValues, "coarse")
	assert.Contains(t, SeedContextManagementGranularityValues, "medium")
	assert.Contains(t, SeedContextManagementGranularityValues, "fine")
	assert.Contains(t, SeedContextManagementGranularityValues, "very_fine")
}

func TestSeedContextManagementApproach_CoarseSlugsAreInCanonicalList(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedContextManagementApproachSlugs {
		all[s] = true
	}
	for _, s := range SeedContextManagementCoarseApproachSlugs {
		assert.True(t, all[s], "coarse slug %q not in canonical list", s)
	}
}
