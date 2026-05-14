package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFEAT024_BDD_PackageStructure validates the package structure registry
// against the narrative descriptions in Appendix A §A.1 and §A.2.
func TestFEAT024_BDD_PackageStructure(t *testing.T) {

	// Scenario 1 — The entry point file is known and discoverable.
	// Given Table 7 in Appendix A.1 lists main.tsx as the entry point at 804 KB,
	// When a caller looks up slug "main_tsx",
	// Then it finds the profile with the correct label, size, and IsEntryPoint=true.
	t.Run("Scenario_EntryPointFileIsDiscoverable", func(t *testing.T) {
		r := NewKeyFileRegistry()

		p, ok := r.FindKeyFileBySlug("main_tsx")

		require.True(t, ok, "main_tsx must be in Table 7 registry")
		assert.Equal(t, "main.tsx", p.Label)
		assert.Equal(t, 804, p.ApproxSizeKB)
		assert.True(t, p.IsEntryPoint, "main.tsx is the application entry point")
		assert.Equal(t, KeyFileLayerEntryAndStartup, p.Layer)
	})

	// Scenario 2 — The core agent loop files are grouped in the same layer.
	// Given query.ts (68 KB) and QueryEngine.ts (47 KB) both belong to the core loop,
	// When a caller filters by KeyFileLayerCoreLoop,
	// Then both files are returned together.
	t.Run("Scenario_CoreLoopFilesGroupedInSameLayer", func(t *testing.T) {
		r := NewKeyFileRegistry()

		coreFiles := r.ByLayer(KeyFileLayerCoreLoop)

		slugs := make(map[string]bool)
		for _, f := range coreFiles {
			slugs[f.Slug] = true
		}
		assert.True(t, slugs["query_ts"], "query.ts must be in core_loop layer")
		assert.True(t, slugs["QueryEngine_ts"], "QueryEngine.ts must be in core_loop layer")
	})

	// Scenario 3 — Large files (no numeric size) are correctly classified.
	// Given mcp/client.ts, compact.ts, AgentTool.tsx, and runAgent.ts are all "Large",
	// When a caller filters by KeyFileSizeLarge,
	// Then at least 4 files are returned.
	t.Run("Scenario_LargeFilesAreClassified", func(t *testing.T) {
		r := NewKeyFileRegistry()

		large := r.BySizeCategory(KeyFileSizeLarge)

		assert.GreaterOrEqual(t, len(large), 4,
			"mcp/client.ts, compact.ts, AgentTool.tsx, runAgent.ts are all Large")
		for _, f := range large {
			assert.Equal(t, 0, f.ApproxSizeKB,
				"large files have no numeric KB value in Table 7")
		}
	})

	// Scenario 4 — Always-included tools are reachable regardless of mode.
	// Given Table 8 lists 8 always-included tools including BashTool and FileReadTool,
	// When a caller filters by ToolAvailAlwaysIncluded,
	// Then bash-tool and file-read-tool are both present.
	t.Run("Scenario_AlwaysIncludedToolsAlwaysPresent", func(t *testing.T) {
		r := NewConditionalToolRegistry()

		always := r.AlwaysIncluded()

		slugs := make(map[string]bool)
		for _, p := range always {
			slugs[p.Slug] = true
		}
		assert.True(t, slugs["bash-tool"], "BashTool must be always-included")
		assert.True(t, slugs["file-read-tool"], "FileReadTool must be always-included")
		assert.GreaterOrEqual(t, len(always), 8)
	})

	// Scenario 5 — Feature-flag gated tools are absent in the default build.
	// Given EnterWorktreeTool requires the "worktree" feature flag,
	// When a caller fetches feature-flag-gated tools,
	// Then enter-worktree-tool is present and its condition references "worktree".
	t.Run("Scenario_FeatureFlagGatedToolReferencesCorrectFlag", func(t *testing.T) {
		r := NewConditionalToolRegistry()

		p, ok := r.FindConditionalToolBySlug("enter-worktree-tool")

		require.True(t, ok)
		assert.Equal(t, ToolAvailFeatureFlag, p.Category)
		assert.Contains(t, p.InclusionCondition, "worktree")
	})

	// Scenario 6 — Null-checked tools depend on runtime capability presence.
	// Given SleepTool, MonitorTool, and RemoteTriggerTool are null-checked,
	// When a caller filters by ToolAvailNullChecked,
	// Then all three appear and each condition mentions "non-nil".
	t.Run("Scenario_NullCheckedToolsMentionNonNilCapability", func(t *testing.T) {
		r := NewConditionalToolRegistry()

		nullChecked := r.NullChecked()

		slugs := make(map[string]bool)
		for _, p := range nullChecked {
			slugs[p.Slug] = true
			assert.Contains(t, p.InclusionCondition, "non-nil",
				"null-checked tool must reference 'non-nil' condition: %s", p.Slug)
		}
		assert.True(t, slugs["sleep-tool"])
		assert.True(t, slugs["monitor-tool"])
		assert.True(t, slugs["remote-trigger-tool"])
	})

	// Scenario 7 — The four categories exhaust the full conditional tool population.
	// Given Table 8 defines exactly four availability categories,
	// When a caller sums the counts across all four categories,
	// Then the total equals SeedConditionalToolCount.
	t.Run("Scenario_FourCategoriesExhaustAllTools", func(t *testing.T) {
		r := NewConditionalToolRegistry()

		total := len(r.AlwaysIncluded()) +
			len(r.EnvironmentGated()) +
			len(r.FeatureFlagGated()) +
			len(r.NullChecked())

		assert.Equal(t, SeedConditionalToolCount, total,
			"four Table 8 categories must cover all %d seeded tools", SeedConditionalToolCount)
	})

	// Scenario 8 — Package size spans from 9 key files to 40+ tool implementations.
	// Given the paper notes 42 tool subdirectories and 86 slash command subdirectories,
	// Given simple mode exposes only 3 tools (Bash, Read, Edit),
	// When a caller inspects always-included tools that carry a MinToolSetSize,
	// Then the minimum is 3 as stated in the paper.
	t.Run("Scenario_SimpleModeMinimumToolSetIs3", func(t *testing.T) {
		r := NewConditionalToolRegistry()

		p, ok := r.FindConditionalToolBySlug("agent-tool")
		require.True(t, ok)

		// The paper states simple mode: Bash, Read, Edit (3 tools).
		// agent-tool carries that note as MinToolSetSize.
		assert.Equal(t, 3, p.MinToolSetSize)
		assert.Equal(t, SeedKeyFileCount, 9,
			"Table 7 documents exactly 9 key files in the package")
	})
}
