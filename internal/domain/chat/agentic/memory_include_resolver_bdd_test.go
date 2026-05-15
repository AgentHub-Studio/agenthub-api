package agentic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_MemoryIncludeResolver(t *testing.T) {
	t.Run("Scenario_AdminAuthoredMemoryReferencesSharedFacts", func(t *testing.T) {
		// Given admin wants to write "User @include{name} works at @include{company}"
		// instead of duplicating values across many memory entries,
		// When the resolver runs,
		// Then @include markers are replaced with the looked-up values —
		// memory stays DRY across the tenant.
		r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
			"name":    "Alice",
			"company": "AgentHub",
		})
		got, _, err := r.Resolve(context.Background(),
			"User @include{name} works at @include{company}")
		require.NoError(t, err)
		assert.Equal(t, "User Alice works at AgentHub", got)
	})

	t.Run("Scenario_CycleDetectionPreventsInfiniteRecursion", func(t *testing.T) {
		// Given admin authored two memories that reference each other,
		// When resolver expands them,
		// Then cycle detection fires before stack overflow — auditable
		// error names the cycle entry.
		r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
			"a": "calls @include{b}",
			"b": "calls @include{a}",
		})
		_, trace, err := r.Resolve(context.Background(), "@include{a}")
		assert.True(t, errors.Is(err, ErrMemoryIncludeCycle))
		assert.NotEmpty(t, trace.CycleDetected,
			"audit trail names which key triggered cycle")
	})

	t.Run("Scenario_MaxDepthGuardPreventsRunawayExpansion", func(t *testing.T) {
		// Given a deep chain of includes (a→b→c→d→e),
		// And admin sets MaxDepth=2,
		// When resolver tries to expand,
		// Then it aborts at depth 3 — runaway expansion prevented even
		// without cycles.
		r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
			MaxDepth:      2,
			MissingPolicy: MemoryIncludeLeaveAsIs,
		}, map[string]string{
			"a": "@include{b}",
			"b": "@include{c}",
			"c": "@include{d}",
			"d": "deep",
		})
		_, _, err := r.Resolve(context.Background(), "@include{a}")
		assert.True(t, errors.Is(err, ErrMemoryIncludeMaxDepth))
	})

	t.Run("Scenario_MissingKeysLeftAsIsForBackwardCompatibility", func(t *testing.T) {
		// Given a transcript replays text authored when "old_key" was
		// defined but the key has since been removed,
		// When resolver runs with policy=leave_as_is,
		// Then unresolved @include{old_key} stays literal — no silent
		// content removal during replay.
		r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
			MaxDepth:      8,
			MissingPolicy: MemoryIncludeLeaveAsIs,
		}, nil)
		got, trace, err := r.Resolve(context.Background(),
			"Note: @include{old_key} (removed)")
		require.NoError(t, err)
		assert.Contains(t, got, "@include{old_key}")
		assert.Contains(t, trace.MissingKeys, "old_key")
	})

	t.Run("Scenario_StrictModeFailsLoudlyOnMissingIncludes", func(t *testing.T) {
		// Given production tenant uses strict mode to catch typos early,
		// When resolver sees @include{typo} that isn't in lookup,
		// Then resolution fails with ErrMemoryIncludeMissing — typos
		// caught at runtime instead of silently rendering broken text.
		r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
			MaxDepth:      8,
			MissingPolicy: MemoryIncludeErrorOnMissing,
		}, nil)
		_, _, err := r.Resolve(context.Background(), "@include{typo}")
		assert.True(t, errors.Is(err, ErrMemoryIncludeMissing))
	})

	t.Run("Scenario_ReplaceWithEmptyForTemplateFlexibility", func(t *testing.T) {
		// Given a template "Welcome @include{title}! @include{message}"
		// where title may or may not exist,
		// When resolver uses replace_with_empty policy,
		// Then missing keys silently drop, leaving template usable
		// across customers with different field availability.
		r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
			MaxDepth:      8,
			MissingPolicy: MemoryIncludeReplaceWithEmpty,
		}, map[string]string{
			"message": "Have a great day!",
		})
		got, _, err := r.Resolve(context.Background(),
			"Welcome @include{title}! @include{message}")
		require.NoError(t, err)
		assert.Equal(t, "Welcome ! Have a great day!", got)
	})

	t.Run("Scenario_NestedExpansionPreservesAdminAuthoredHierarchy", func(t *testing.T) {
		// Given admin authored hierarchical memory (greeting includes
		// formal_intro which includes company_name),
		// When resolver runs,
		// Then all levels expand correctly — admin can compose memories
		// from reusable building blocks.
		r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
			"greeting":     "Dear @include{formal_intro}",
			"formal_intro": "Customer of @include{company_name}",
			"company_name": "AgentHub Inc.",
		})
		got, _, err := r.Resolve(context.Background(), "@include{greeting}")
		require.NoError(t, err)
		assert.Equal(t, "Dear Customer of AgentHub Inc.", got)
	})

	t.Run("Scenario_TraceProvidesGOV001AuditDataForExpansions", func(t *testing.T) {
		// Given GOV-001 audits which @include values were substituted,
		// When the resolver runs,
		// Then trace.ExpandedKeys lists every key resolved — auditor
		// can reconstruct WHAT the LLM ultimately saw.
		r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
			"a": "value-a",
			"b": "value-b",
		})
		_, trace, err := r.Resolve(context.Background(), "@include{a} and @include{b}")
		require.NoError(t, err)
		assert.Contains(t, trace.ExpandedKeys, "a")
		assert.Contains(t, trace.ExpandedKeys, "b")
	})

	t.Run("Scenario_SetLookupSupportsLiveMemoryUpdates", func(t *testing.T) {
		// Given memory hierarchy (CTX-003) updates values during a session,
		// When CTX-007 resolver is told to use new lookup,
		// Then SetLookup replaces the internal table — next Resolve uses
		// fresh data (no stale resolver after updates).
		r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
			"v": "old",
		})
		got, _, _ := r.Resolve(context.Background(), "@include{v}")
		assert.Equal(t, "old", got)
		r.SetLookup(map[string]string{"v": "new"})
		got, _, _ = r.Resolve(context.Background(), "@include{v}")
		assert.Equal(t, "new", got)
	})

	t.Run("Scenario_DefensiveCopyPreventsExternalLookupMutation", func(t *testing.T) {
		// Given admin passes a lookup map; later mutates the same map,
		// When resolver still resolves,
		// Then output uses ORIGINAL values — defensive copy at construct
		// time prevents subtle bugs from shared-map mutation.
		external := map[string]string{"x": "original"}
		r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), external)
		external["x"] = "mutated"
		got, _, _ := r.Resolve(context.Background(), "@include{x}")
		assert.Equal(t, "original", got, "external mutation does not leak in")
	})
}
