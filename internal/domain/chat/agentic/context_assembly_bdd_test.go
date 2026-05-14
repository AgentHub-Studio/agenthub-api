package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for the §7.1 nine-source context window assembly order.

func TestBDD_ContextAssemblySourceRegistry(t *testing.T) {
	t.Run("Scenario_NineSourcesAssembleInFixedOrder", func(t *testing.T) {
		// Given §7.1 specifies nine ordered sources for context window assembly
		// When the sequence is loaded
		// Then exactly 9 sources exist with orders 1..9 and the invariant holds
		assert.Equal(t, 9, len(ContextAssemblySequence))
		assert.True(t, IsContextAssemblySequentiallyOrdered())
	})

	t.Run("Scenario_SystemPromptAlwaysLeadsAssembly", func(t *testing.T) {
		// Given the system prompt is the foundational context for every turn
		// When the assembly sequence is inspected
		// Then system_prompt is first and is always included
		r := NewContextAssemblySourceRegistry()
		first := r.AllSources()[0]
		assert.Equal(t, ContextSourceSystemPrompt, first)
		p, _ := r.Profile(first)
		assert.True(t, p.IsAlwaysIncluded)
	})

	t.Run("Scenario_AutoMemoryAndToolMetadataAreAsynchronous", func(t *testing.T) {
		// Given §7.1 notes auto_memory is prefetched async and tool metadata
		//      includes deferred tool definitions resolved on demand
		// When async sources are queried
		// Then exactly auto_memory and tool_metadata are returned
		r := NewContextAssemblySourceRegistry()
		async := r.AsyncSources()
		assert.Equal(t, 2, len(async))
		assert.Contains(t, async, ContextSourceAutoMemory)
		assert.Contains(t, async, ContextSourceToolMetadata)
	})

	t.Run("Scenario_SixDomainsPartitionAllNineSourcesWithoutOverlap", func(t *testing.T) {
		// Given the taxonomy groups sources by functional role
		// When all domains are enumerated and their sources counted
		// Then the total is exactly 9 with no source in two domains
		r := NewContextAssemblySourceRegistry()
		domains := []ContextAssemblyDomain{
			ContextDomainPromptConstruction, ContextDomainPlatformContext,
			ContextDomainInstructionFiles, ContextDomainMemory,
			ContextDomainToolDefinitions, ContextDomainConversation,
		}
		seen := map[ContextAssemblySource]bool{}
		total := 0
		for _, d := range domains {
			for _, src := range r.SourcesInDomain(d) {
				assert.False(t, seen[src], "source %q in multiple domains", src)
				seen[src] = true
				total++
			}
		}
		assert.Equal(t, 9, total)
	})

	t.Run("Scenario_CompactSummariesIsConditionalAndLast", func(t *testing.T) {
		// Given compact_summaries only exist after a compaction event
		// When the profile for compact_summaries is inspected
		// Then it is order 9, not always included, and in the conversation domain
		r := NewContextAssemblySourceRegistry()
		p, ok := r.Profile(ContextSourceCompactSummaries)
		assert.True(t, ok)
		assert.Equal(t, 9, p.SourceOrder)
		assert.False(t, p.IsAlwaysIncluded)
		assert.Equal(t, ContextDomainConversation, p.Domain)
	})
}
