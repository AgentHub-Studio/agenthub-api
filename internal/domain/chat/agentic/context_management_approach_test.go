package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextManagementApproachRegistry_FiveApproachesFromTable6(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	assert.Equal(t, 5, len(r.AllApproaches()))
}

func TestContextManagementApproachRegistry_ProfileSimpleTruncation(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	p, ok := r.Profile(ContextMgmtSimpleTruncation)
	assert.True(t, ok)
	assert.Equal(t, ContextMgmtGranularityCoarse, p.Granularity)
	assert.Contains(t, p.Mechanism, "oldest")
}

func TestContextManagementApproachRegistry_ProfileSlidingWindow(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	p, ok := r.Profile(ContextMgmtSlidingWindow)
	assert.True(t, ok)
	assert.Equal(t, ContextMgmtGranularityMedium, p.Granularity)
	assert.Contains(t, p.Mechanism, "recent")
}

func TestContextManagementApproachRegistry_ProfileRAG(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	p, ok := r.Profile(ContextMgmtRAG)
	assert.True(t, ok)
	assert.Equal(t, ContextMgmtGranularityFine, p.Granularity)
	assert.Contains(t, p.Mechanism, "relevant")
}

func TestContextManagementApproachRegistry_ProfileSingleSummarization(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	p, ok := r.Profile(ContextMgmtSingleSummarization)
	assert.True(t, ok)
	assert.Equal(t, ContextMgmtGranularityCoarse, p.Granularity)
	assert.Contains(t, p.Mechanism, "compress")
}

func TestContextManagementApproachRegistry_ProfileGraduatedCompaction(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	p, ok := r.Profile(ContextMgmtGraduatedCompaction)
	assert.True(t, ok)
	assert.Equal(t, ContextMgmtGranularityVeryFine, p.Granularity)
	assert.True(t, p.IsAgentHubApproach)
	assert.Contains(t, p.Mechanism, "pipeline")
}

func TestContextManagementApproachRegistry_UnknownApproachReturnsFalse(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	_, ok := r.Profile("not_real")
	assert.False(t, ok)
}

func TestContextManagementApproachRegistry_IsValidApproach_KnownTrue(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	assert.True(t, r.IsValidApproach(ContextMgmtGraduatedCompaction))
}

func TestContextManagementApproachRegistry_IsValidApproach_UnknownFalse(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	assert.False(t, r.IsValidApproach("invented"))
}

func TestContextManagementApproachRegistry_TwoCoarseApproaches(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	coarse := r.ApproachesAtGranularity(ContextMgmtGranularityCoarse)
	assert.Equal(t, 2, len(coarse))
	slugs := []ContextManagementApproach{coarse[0].Approach, coarse[1].Approach}
	assert.Contains(t, slugs, ContextMgmtSimpleTruncation)
	assert.Contains(t, slugs, ContextMgmtSingleSummarization)
}

func TestContextManagementApproachRegistry_OnlyGraduatedCompactionIsVeryFine(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	vf := r.ApproachesAtGranularity(ContextMgmtGranularityVeryFine)
	assert.Equal(t, 1, len(vf))
	assert.Equal(t, ContextMgmtGraduatedCompaction, vf[0].Approach)
}

func TestContextManagementApproachRegistry_AgentHubApproachIsGraduatedCompaction(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	p := r.AgentHubApproach()
	assert.Equal(t, ContextMgmtGraduatedCompaction, p.Approach)
	assert.Equal(t, ContextMgmtGranularityVeryFine, p.Granularity)
}

func TestContextManagementApproachRegistry_OnlyOneAgentHubApproach(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	count := 0
	for _, p := range r.AllApproaches() {
		if p.IsAgentHubApproach {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestContextManagementApproachRegistry_AllApproachesHaveNonEmptyMechanism(t *testing.T) {
	r := NewContextManagementApproachRegistry()
	for _, p := range r.AllApproaches() {
		assert.NotEmpty(t, p.Mechanism, "approach %s has empty mechanism", p.Approach)
	}
}
