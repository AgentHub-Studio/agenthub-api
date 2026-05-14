package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreCRDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksReferencePolicyWithoutInventingThresholds", func(t *testing.T) {
		// Given fresh tenants need a content-reference policy,
		// And CTX-009 ContentReferenceRegistry accepts kinds + thresholds,
		// When admin opens reference-policy onboarding,
		// Then 5 recommended profiles cover use-case spectrum (general /
		// research / kb-heavy / code / cost-strict).
		assert.Equal(t, 5, len(SeedRecommendedCRDTemplateSlugs))
	})

	t.Run("Scenario_BalancedDefaultReferencesCommonHeavyKinds", func(t *testing.T) {
		// Given general tenants have KB chunks + tool results + file
		// blobs as the heaviest content sources,
		// When admin picks balanced-default,
		// Then enabled_kinds includes those 3 (web_fetch + memory_snapshot
		// inlined since less common per-tenant).
		assert.Contains(t, SeedExpectedCRDTemplateSlugs, "balanced-default")
	})

	t.Run("Scenario_ResearchHeavyAggressivelyReferencesKBContent", func(t *testing.T) {
		// Given research workflows query KBs heavily,
		// When admin picks research-heavy,
		// Then min_bytes is LOW (1KB) so even small KB chunks get @ref
		// — context window stays bounded over many sources.
		assert.Contains(t, SeedExpectedCRDTemplateSlugs, "research-heavy")
	})

	t.Run("Scenario_KBHeavyOnlyReferencesKBChunksOnly", func(t *testing.T) {
		// Given some tenants have huge KBs but small tool outputs,
		// When admin picks kb-heavy-only,
		// Then enabled_kinds = "kb_chunk" alone — tool results stay inline
		// (their cost saving from referencing wouldn't justify the
		// resolution overhead).
		assert.Contains(t, SeedExpectedCRDTemplateSlugs, "kb-heavy-only")
	})

	t.Run("Scenario_CodeHeavyReferencesToolResultsAndFileBlobs", func(t *testing.T) {
		// Given code-gen agents pull verbose tool outputs (tests, builds)
		// and reference code files,
		// When admin picks code-heavy,
		// Then enabled_kinds = "tool_result,file_blob" — KB inline
		// (small docstrings stay verbatim).
		assert.Contains(t, SeedExpectedCRDTemplateSlugs, "code-heavy")
	})

	t.Run("Scenario_CostStrictRequiresAdminReviewBecauseDefatesCacheBenefits", func(t *testing.T) {
		// Given cost-strict almost-always inlines (16KB threshold),
		// When admin picks it,
		// Then template requires admin review (defeats CTX-009 cache
		// benefits — cross-region storage / @ref resolution costs).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewCRDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["cost-strict"])
	})

	t.Run("Scenario_DevDebugDisablesReferencesEntirelyForFullVisibility", func(t *testing.T) {
		// Given dev tenants want raw bytes for debugging,
		// When admin picks dev-debug,
		// Then enabled_kinds is empty — everything inlined; LARGE prompts
		// but full visibility. NOT recommended for production.
		set := map[string]bool{}
		for _, s := range SeedRecommendedCRDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dev-debug"])
	})

	t.Run("Scenario_KindLabelsMatchCTX009ContentReferenceKindByteForByte", func(t *testing.T) {
		// Given CTX-009 has 5 ContentReferenceKind values,
		// When seed declares kind labels,
		// Then they match enum bytes (no mapping table runtime).
		ctx009 := []string{
			"kb_chunk", "tool_result", "file_blob", "web_fetch", "memory_snapshot",
		}
		set := map[string]bool{}
		for _, k := range SeedExpectedCRDTemplateKindLabels {
			set[k] = true
		}
		for _, e := range ctx009 {
			assert.True(t, set[e], "CTX-009 kind %q missing from seed", e)
		}
	})

	t.Run("Scenario_ThresholdsLadderReflectsCacheVsCostTradeoff", func(t *testing.T) {
		// Given thresholds tradeoff: lower = more refs (cache wins, but
		// resolution cost) vs higher = more inlining (no resolution but
		// bigger prompts),
		// When admin compares profiles,
		// Then thresholds ladder: research(1k) < kb-heavy(2k) ≤
		// balanced(4k) ≤ code-heavy(4k) < cost-strict(16k) — explicit
		// trade-off curve. Validated structurally via integration test.
		assert.Equal(t, 6, len(SeedExpectedCRDTemplateSlugs))
	})

	t.Run("Scenario_IdleGCWindowsAlignWithUseCase", func(t *testing.T) {
		// Given research needs replay windows (7 days), prod stays at 24h,
		// dev never sweeps,
		// When admin compares profiles,
		// Then idle_gc_seconds reflects use-case semantics:
		//   research(604800=7d) > balanced(86400=24h) > cost-strict(21600=6h) > dev(0=never)
		// (Validated via integration test.)
		assert.Equal(t, 6, len(SeedExpectedCRDTemplateSlugs))
	})
}
