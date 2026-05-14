package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ToolDedup(t *testing.T) {
	t.Run("Scenario_AliasFromLegacySkillCollapsesToCanonical", func(t *testing.T) {
		// Given a legacy skill exposes "read_file" as an alias for "Read",
		// When the dedup resolver runs,
		// Then the LLM sees one tool ("Read"), not two split-brain ghosts.
		r, _ := NewToolDedupResolver(ToolDedupConfig{
			Policy:  ToolDedupPreferSourceRank,
			Aliases: ToolAliasMap{"read_file": "Read"},
		})
		in := []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "read_file", Source: ToolSourceSkill, ProviderID: "legacy"},
		}
		kept, _, err := r.Resolve(in)
		require.NoError(t, err)
		assert.Equal(t, 1, len(kept))
	})

	t.Run("Scenario_LatestVersionWinsByDefaultForFreshTenant", func(t *testing.T) {
		// Given two versions of vector_search are installed,
		// When the default policy is prefer_latest_version,
		// Then 2.0 wins over 1.0.
		r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPreferLatestVersion})
		in := []ToolPoolEntry{
			{Name: "vector_search@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
			{Name: "vector_search@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		}
		kept, _, err := r.Resolve(in)
		require.NoError(t, err)
		assert.Equal(t, "vector_search@2.0.0", kept[0].Name)
	})

	t.Run("Scenario_AdminPinForcesSpecificVersion", func(t *testing.T) {
		// Given a tenant pinned vector_search@1.0.0 (audit reason: API
		// shape changed in 2.0 and migration is incomplete),
		// When dedup runs with prefer_pinned,
		// Then 1.0.0 wins even though 2.0.0 is also present.
		r, _ := NewToolDedupResolver(ToolDedupConfig{
			Policy: ToolDedupPreferPinned,
			Pins:   []ToolVersionPin{{Name: "vector_search", Version: "1.0.0"}},
		})
		in := []ToolPoolEntry{
			{Name: "vector_search@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
			{Name: "vector_search@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		}
		kept, results, err := r.Resolve(in)
		require.NoError(t, err)
		assert.Equal(t, "vector_search@1.0.0", kept[0].Name)
		assert.Contains(t, results[0].Reason, "matched pin")
	})

	t.Run("Scenario_DenyCollisionStopsAssemblyOnAccidentalDualInstall", func(t *testing.T) {
		// Given two extensions accidentally ship the same tool at
		// different versions and the tenant has zero tolerance for
		// surprise version drift,
		// When dedup runs with deny_collision,
		// Then assembly fails loudly so admin notices.
		r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupDenyCollision})
		in := []ToolPoolEntry{
			{Name: "x@1.0.0", Source: ToolSourceExtension, ProviderID: "ext-a"},
			{Name: "x@2.0.0", Source: ToolSourceExtension, ProviderID: "ext-b"},
		}
		_, _, err := r.Resolve(in)
		assert.ErrorIs(t, err, ErrToolDedupVersionConflict)
	})

	t.Run("Scenario_KeepAllVersionsEnablesMigrationWindow", func(t *testing.T) {
		// Given a tenant is in a migration window where some agents call
		// vector_search@1 and new ones call vector_search@2,
		// When dedup uses keep_all_versions,
		// Then both versions remain visible by their fully-qualified name.
		r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupKeepAllVersions})
		in := []ToolPoolEntry{
			{Name: "vector_search@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
			{Name: "vector_search@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		}
		kept, results, err := r.Resolve(in)
		require.NoError(t, err)
		assert.Equal(t, 2, len(kept))
		assert.Equal(t, []string{"1.0.0", "2.0.0"}, results[0].Versions)
	})

	t.Run("Scenario_StatefulToolBypassesDedup", func(t *testing.T) {
		// Given "session" is a stateful tool (has a backing session state
		// per provider) and dedup would lose context,
		// When dedup runs,
		// Then both entries pass through untouched.
		r, _ := NewToolDedupResolver(ToolDedupConfig{
			Policy:        ToolDedupPreferLatestVersion,
			StatefulTools: map[string]bool{"session": true},
		})
		in := []ToolPoolEntry{
			{Name: "session@1.0.0", Source: ToolSourceSkill, ProviderID: "p1"},
			{Name: "session@2.0.0", Source: ToolSourceSkill, ProviderID: "p2"},
		}
		kept, _, err := r.Resolve(in)
		require.NoError(t, err)
		assert.Equal(t, 2, len(kept))
	})

	t.Run("Scenario_PerToolPolicyOverridesGlobalDefault", func(t *testing.T) {
		// Given the platform default is keep_all_versions but tool "x"
		// must be deduplicated to latest because callers cannot specify
		// version,
		// When dedup runs,
		// Then x is reduced to latest while other tools remain duplicated.
		r, _ := NewToolDedupResolver(ToolDedupConfig{
			Policy:        ToolDedupKeepAllVersions,
			PerToolPolicy: map[string]ToolDedupPolicy{"x": ToolDedupPreferLatestVersion},
		})
		in := []ToolPoolEntry{
			{Name: "x@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
			{Name: "x@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
			{Name: "y@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
			{Name: "y@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		}
		kept, _, err := r.Resolve(in)
		require.NoError(t, err)
		assert.Equal(t, 3, len(kept))
	})

	t.Run("Scenario_DropDecisionsRecordedForSecurityAudit", func(t *testing.T) {
		// Given a security incident asks "why did the LLM not see the
		// extension version of `x`?", the audit must record what was
		// dropped and why.
		r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPreferSourceRank})
		in := []ToolPoolEntry{
			{Name: "x", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "x", Source: ToolSourceExtension, ProviderID: "ext-vendor"},
		}
		_, results, err := r.Resolve(in)
		require.NoError(t, err)
		require.Equal(t, 1, len(results))
		require.Equal(t, 1, len(results[0].Dropped))
		assert.Equal(t, ToolSourceExtension, results[0].Dropped[0].Source)
		assert.Contains(t, results[0].Reason, "builtin")
	})

	t.Run("Scenario_VersionedNameSyntaxParsesAndFormatsBackToSame", func(t *testing.T) {
		// Given a versioned name passes through parser and back,
		// When the LLM sees it again,
		// Then it round-trips byte-for-byte (no string corruption).
		v, err := ParseVersionedToolName("foo@1.2.3")
		require.NoError(t, err)
		assert.Equal(t, "foo@1.2.3", v.String())
	})

	t.Run("Scenario_SemverComparisonAvoidsLexicographicTrap", func(t *testing.T) {
		// Given lexicographic comparison would say "10.0.0" < "2.0.0",
		// When CompareSemver runs,
		// Then numeric comparison correctly picks 10 > 2.
		assert.Equal(t, 1, CompareSemver("10.0.0", "2.0.0"))
	})
}
