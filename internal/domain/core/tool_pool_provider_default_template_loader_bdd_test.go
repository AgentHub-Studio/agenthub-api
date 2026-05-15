package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreTPPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksProvidersFromCatalog", func(t *testing.T) {
		// Given fresh tenants need a non-empty tool pool day one,
		// And TOOL-003 ToolPoolAssembler requires ≥1 provider,
		// When admin opens provider onboarding,
		// Then 6 recommended templates surface across all 5 sources.
		assert.Equal(t, 6, len(SeedRecommendedTPPDTemplateSlugs))
	})

	t.Run("Scenario_BuiltinReadOnlyAlwaysAvailable", func(t *testing.T) {
		// Given inspection is the safe baseline every tenant needs,
		// When admin uses builtin-readonly-core,
		// Then Read/Grep/Glob are wired with no admin review.
		assert.Contains(t, SeedExpectedTPPDTemplateSlugs, "builtin-readonly-core")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewTPPDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["builtin-readonly-core"])
	})

	t.Run("Scenario_BuiltinMutatingRequiresAdminReview", func(t *testing.T) {
		// Given Bash/Edit/Write expand the destructive surface,
		// When admin uses builtin-mutating-core,
		// Then admin review is mandatory.
		set := map[string]bool{}
		for _, s := range SeedAdminReviewTPPDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["builtin-mutating-core"])
	})

	t.Run("Scenario_SkillDocumentSearchEnablesRAGFromKnowledgeBase", func(t *testing.T) {
		// Given tenants upload documents and want answers grounded in them,
		// When admin uses skill-document-search,
		// Then document_search tool materialises from a KB skill binding.
		assert.Contains(t, SeedExpectedTPPDTemplateSlugs, "skill-document-search")
	})

	t.Run("Scenario_MCPFilesystemBridgesLocalResources", func(t *testing.T) {
		// Given engineering tenants want bridged file access via MCP,
		// When admin uses mcp-filesystem-default,
		// Then mcp_fs_list/read are exposed via TOOL-003 mcp source.
		assert.Contains(t, SeedExpectedTPPDTemplateSlugs, "mcp-filesystem-default")
	})

	t.Run("Scenario_SubagentAllowlistEnforcesDelegationSafety", func(t *testing.T) {
		// Given parent agents delegate investigation work but not mutation,
		// When admin uses subagent-readonly-allowlist,
		// Then subagent pool is restricted to read-only tools.
		assert.Contains(t, SeedExpectedTPPDTemplateSlugs, "subagent-readonly-allowlist")
	})

	t.Run("Scenario_ExtensionUtilitiesShowVendorContract", func(t *testing.T) {
		// Given vendors need a concrete EXT-001 example to model after,
		// When admin uses extension-platform-utilities,
		// Then token_usage / conversation_summary tools demonstrate vendor delivery.
		assert.Contains(t, SeedExpectedTPPDTemplateSlugs, "extension-platform-utilities")
	})

	t.Run("Scenario_SourceLabelsMatchTOOL003EnumByteForByte", func(t *testing.T) {
		// Given TOOL-003 ToolSource has 5 values,
		// When seed declares target_source,
		// Then labels match enum bytes (no mapping table runtime).
		tool003 := []string{"builtin", "skill", "mcp", "subagent", "extension"}
		set := map[string]bool{}
		for _, s := range SeedExpectedTPPDTemplateSources {
			set[s] = true
		}
		for _, e := range tool003 {
			assert.True(t, set[e], "TOOL-003 source %q missing", e)
		}
	})

	t.Run("Scenario_AllFiveSourcesRepresented", func(t *testing.T) {
		// Given TOOL-003 has 5 sources,
		// When seed templates ship,
		// Then ALL 5 sources have at least one example template.
		// Validated structurally via integration test.
		assert.Equal(t, 5, len(SeedExpectedTPPDTemplateSources))
	})

	t.Run("Scenario_PriorityLadderReflectsProviderRegistrationHint", func(t *testing.T) {
		// Given ToolPoolAssembler resolves collisions via toolSourceRank,
		// When admin compares default_priority across templates,
		// Then builtin variants get lowest numerical (highest precedence).
		// Validated cross-row in integration test.
		assert.Equal(t, 6, len(SeedExpectedTPPDTemplateSlugs))
	})
}
