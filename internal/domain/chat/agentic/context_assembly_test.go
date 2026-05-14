package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextAssemblySequence_HasNineSources(t *testing.T) {
	assert.Equal(t, 9, len(ContextAssemblySequence))
}

func TestIsContextAssemblySequentiallyOrdered_ReturnsTrue(t *testing.T) {
	assert.True(t, IsContextAssemblySequentiallyOrdered())
}

func TestContextAssemblySequence_AllSourceOrdersDistinct(t *testing.T) {
	seen := map[int]ContextAssemblySource{}
	for _, src := range ContextAssemblySequence {
		p := contextAssemblySourceProfiles[src]
		_, exists := seen[p.SourceOrder]
		assert.False(t, exists, "duplicate SourceOrder %d for source %q", p.SourceOrder, src)
		seen[p.SourceOrder] = src
	}
}

func TestContextAssemblySequence_OrdersOneToNine(t *testing.T) {
	for i, src := range ContextAssemblySequence {
		p := contextAssemblySourceProfiles[src]
		assert.Equal(t, i+1, p.SourceOrder, "source %q should have order %d", src, i+1)
	}
}

func TestContextAssemblySourceRegistry_Profile_KnownSource(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	p, ok := r.Profile(ContextSourceAutoMemory)
	require.True(t, ok)
	assert.Equal(t, ContextSourceAutoMemory, p.Source)
	assert.Equal(t, 5, p.SourceOrder)
	assert.Equal(t, ContextDomainMemory, p.Domain)
	assert.False(t, p.IsMemoized)
	assert.True(t, p.IsAsynchronous)
	assert.False(t, p.IsAlwaysIncluded)
}

func TestContextAssemblySourceRegistry_Profile_UnknownSource(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	_, ok := r.Profile("not_a_real_source")
	assert.False(t, ok)
}

func TestContextAssemblySourceRegistry_AllSources_Length(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	assert.Equal(t, 9, len(r.AllSources()))
}

func TestContextAssemblySourceRegistry_AllSources_DefensiveCopy(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	sources := r.AllSources()
	original := sources[0]
	sources[0] = "mutated"
	assert.Equal(t, original, r.AllSources()[0], "AllSources must return a defensive copy")
}

func TestContextAssemblySourceRegistry_AlwaysIncludedSources(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	always := r.AlwaysIncludedSources()
	expected := []ContextAssemblySource{
		ContextSourceSystemPrompt,
		ContextSourceEnvironmentInfo,
		ContextSourceClaudeMDHierarchy,
		ContextSourceToolMetadata,
		ContextSourceConversationHistory,
		ContextSourceToolResults,
	}
	assert.Equal(t, expected, always)
}

func TestContextAssemblySourceRegistry_AsyncSources(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	async := r.AsyncSources()
	expected := []ContextAssemblySource{
		ContextSourceAutoMemory,
		ContextSourceToolMetadata,
	}
	assert.Equal(t, expected, async)
}

func TestContextAssemblySourceRegistry_MemoizedSources(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	memoized := r.MemoizedSources()
	expected := []ContextAssemblySource{
		ContextSourceEnvironmentInfo,
		ContextSourceClaudeMDHierarchy,
	}
	assert.Equal(t, expected, memoized)
}

func TestContextAssemblySourceRegistry_SourcesInDomain_InstructionFiles(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	sources := r.SourcesInDomain(ContextDomainInstructionFiles)
	expected := []ContextAssemblySource{
		ContextSourceClaudeMDHierarchy,
		ContextSourcePathScopedRules,
	}
	assert.Equal(t, expected, sources)
}

func TestContextAssemblySourceRegistry_SourcesInDomain_Conversation(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	sources := r.SourcesInDomain(ContextDomainConversation)
	expected := []ContextAssemblySource{
		ContextSourceConversationHistory,
		ContextSourceToolResults,
		ContextSourceCompactSummaries,
	}
	assert.Equal(t, expected, sources)
}

func TestContextAssemblySourceRegistry_SourcesInDomain_Unknown(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	sources := r.SourcesInDomain("not_a_domain")
	assert.Empty(t, sources)
}

func TestContextAssemblySourceProfiles_SixDomainsPartitionAllNine(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	domains := []ContextAssemblyDomain{
		ContextDomainPromptConstruction,
		ContextDomainPlatformContext,
		ContextDomainInstructionFiles,
		ContextDomainMemory,
		ContextDomainToolDefinitions,
		ContextDomainConversation,
	}
	seen := map[ContextAssemblySource]bool{}
	total := 0
	for _, domain := range domains {
		for _, src := range r.SourcesInDomain(domain) {
			assert.False(t, seen[src], "source %q in multiple domains", src)
			seen[src] = true
			total++
		}
	}
	assert.Equal(t, 9, total)
}

func TestContextAssemblySourceProfiles_SystemPromptIsFirstAlwaysIncluded(t *testing.T) {
	p := contextAssemblySourceProfiles[ContextSourceSystemPrompt]
	assert.Equal(t, 1, p.SourceOrder)
	assert.True(t, p.IsAlwaysIncluded)
	assert.False(t, p.IsMemoized)
	assert.False(t, p.IsAsynchronous)
}

func TestContextAssemblySourceProfiles_EnvironmentInfoIsMemoized(t *testing.T) {
	p := contextAssemblySourceProfiles[ContextSourceEnvironmentInfo]
	assert.Equal(t, 2, p.SourceOrder)
	assert.True(t, p.IsMemoized, "environment_info is memoized once per session")
}

func TestContextAssemblySourceProfiles_PathScopedRulesIsConditional(t *testing.T) {
	p := contextAssemblySourceProfiles[ContextSourcePathScopedRules]
	assert.False(t, p.IsAlwaysIncluded, "path_scoped_rules are loaded lazily by directory match")
}

func TestContextAssemblySourceProfiles_CompactSummariesIsConditional(t *testing.T) {
	p := contextAssemblySourceProfiles[ContextSourceCompactSummaries]
	assert.Equal(t, 9, p.SourceOrder, "compact_summaries is last in assembly order")
	assert.False(t, p.IsAlwaysIncluded, "compact_summaries only present after compaction")
}

func TestContextAssemblySourceProfiles_ConversationDomainHasThreeSources(t *testing.T) {
	r := NewContextAssemblySourceRegistry()
	assert.Equal(t, 3, len(r.SourcesInDomain(ContextDomainConversation)))
}
