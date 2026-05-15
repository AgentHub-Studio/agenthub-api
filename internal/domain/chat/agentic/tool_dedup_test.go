package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolDedup_IsValidPolicy(t *testing.T) {
	for _, p := range allToolDedupPolicies {
		assert.True(t, IsValidToolDedupPolicy(p))
	}
	assert.False(t, IsValidToolDedupPolicy(ToolDedupPolicy("nope")))
	assert.False(t, IsValidToolDedupPolicy(ToolDedupPolicy("")))
}

func TestToolDedup_ParseVersionedNameUnversioned(t *testing.T) {
	v, err := ParseVersionedToolName("Read")
	require.NoError(t, err)
	assert.Equal(t, "Read", v.Name)
	assert.Empty(t, v.Version)
	assert.Equal(t, "Read", v.String())
}

func TestToolDedup_ParseVersionedNameSemver(t *testing.T) {
	v, err := ParseVersionedToolName("vector_search@1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "vector_search", v.Name)
	assert.Equal(t, "1.2.3", v.Version)
	assert.Equal(t, "vector_search@1.2.3", v.String())
}

func TestToolDedup_ParsePartialSemver(t *testing.T) {
	v, err := ParseVersionedToolName("foo@1.2")
	require.NoError(t, err)
	assert.Equal(t, "1.2", v.Version)
}

func TestToolDedup_ParseRejectsEmpty(t *testing.T) {
	_, err := ParseVersionedToolName("")
	assert.ErrorIs(t, err, ErrToolDedupEmptyName)
	_, err = ParseVersionedToolName("   ")
	assert.ErrorIs(t, err, ErrToolDedupEmptyName)
}

func TestToolDedup_ParseRejectsBadName(t *testing.T) {
	_, err := ParseVersionedToolName("123name")
	assert.ErrorIs(t, err, ErrToolDedupBadName)
	_, err = ParseVersionedToolName("name@bad-version")
	assert.ErrorIs(t, err, ErrToolDedupBadName)
}

func TestToolDedup_CompareSemver(t *testing.T) {
	assert.Equal(t, -1, CompareSemver("1.0.0", "2.0.0"))
	assert.Equal(t, 1, CompareSemver("2.0.0", "1.9.9"))
	assert.Equal(t, 0, CompareSemver("1.2.3", "1.2.3"))
	assert.Equal(t, 1, CompareSemver("1.2", "1.0.0")) // 1.2.0 > 1.0.0
	assert.Equal(t, 0, CompareSemver("", ""))
}

func TestToolDedup_AliasMapResolves(t *testing.T) {
	m := ToolAliasMap{"read_file": "Read", "fileRead": "Read"}
	assert.Equal(t, "Read", m.Canonical("read_file"))
	assert.Equal(t, "Read", m.Canonical("fileRead"))
	assert.Equal(t, "unknown", m.Canonical("unknown"))
	var nilMap ToolAliasMap
	assert.Equal(t, "X", nilMap.Canonical("X"))
}

func TestToolDedup_ConfigValidate(t *testing.T) {
	good := ToolDedupConfig{Policy: ToolDedupPreferSourceRank}
	assert.NoError(t, good.Validate())

	bad := ToolDedupConfig{Policy: ToolDedupPolicy("nope")}
	assert.ErrorIs(t, bad.Validate(), ErrToolDedupBadPolicy)

	badPin := ToolDedupConfig{Policy: ToolDedupPreferPinned, Pins: []ToolVersionPin{{Name: ""}}}
	assert.ErrorIs(t, badPin.Validate(), ErrToolDedupBadPin)

	badPerTool := ToolDedupConfig{Policy: ToolDedupPreferSourceRank,
		PerToolPolicy: map[string]ToolDedupPolicy{"foo": ToolDedupPolicy("nope")}}
	assert.ErrorIs(t, badPerTool.Validate(), ErrToolDedupBadPolicy)
}

func TestToolDedup_NewResolverRejectsBadConfig(t *testing.T) {
	_, err := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPolicy("nope")})
	assert.ErrorIs(t, err, ErrToolDedupBadPolicy)
}

func TestToolDedup_AliasResolution(t *testing.T) {
	r, err := NewToolDedupResolver(ToolDedupConfig{
		Policy:  ToolDedupPreferSourceRank,
		Aliases: ToolAliasMap{"read_file": "Read"},
	})
	require.NoError(t, err)
	in := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "read_file", Source: ToolSourceSkill, ProviderID: "skill-x"},
	}
	kept, results, err := r.Resolve(in)
	require.NoError(t, err)
	require.Equal(t, 1, len(kept))
	assert.Equal(t, ToolSourceBuiltin, kept[0].Source)
	require.Equal(t, 1, len(results))
	assert.Equal(t, "Read", results[0].Canonical)
}

func TestToolDedup_PreferLatestVersion(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPreferLatestVersion})
	in := []ToolPoolEntry{
		{Name: "vector_search@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "vector_search@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "vector_search@1.5.0", Source: ToolSourceSkill, ProviderID: "p"},
	}
	kept, _, err := r.Resolve(in)
	require.NoError(t, err)
	require.Equal(t, 1, len(kept))
	assert.Equal(t, "vector_search@2.0.0", kept[0].Name)
}

func TestToolDedup_KeepAllVersions(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupKeepAllVersions})
	in := []ToolPoolEntry{
		{Name: "vector_search@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "vector_search@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
	}
	kept, results, err := r.Resolve(in)
	require.NoError(t, err)
	assert.Equal(t, 2, len(kept))
	assert.Equal(t, []string{"1.0.0", "2.0.0"}, results[0].Versions)
}

func TestToolDedup_DenyCollisionFails(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupDenyCollision})
	in := []ToolPoolEntry{
		{Name: "x@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "x@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
	}
	_, _, err := r.Resolve(in)
	assert.ErrorIs(t, err, ErrToolDedupVersionConflict)
}

func TestToolDedup_DenyCollisionAllowsSameVersion(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupDenyCollision})
	in := []ToolPoolEntry{
		{Name: "x@1.0.0", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "x@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
	}
	kept, _, err := r.Resolve(in)
	require.NoError(t, err)
	assert.Equal(t, 1, len(kept))
}

func TestToolDedup_PreferPinnedMatch(t *testing.T) {
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
	require.Equal(t, 1, len(kept))
	assert.Equal(t, "vector_search@1.0.0", kept[0].Name)
	assert.Contains(t, results[0].Reason, "matched pin")
}

func TestToolDedup_PreferPinnedFallsBackToSourceRank(t *testing.T) {
	// Pin specifies version 9.9.9 not present in pool — falls back to
	// source-rank precedence (builtin wins skill).
	r, _ := NewToolDedupResolver(ToolDedupConfig{
		Policy: ToolDedupPreferPinned,
		Pins:   []ToolVersionPin{{Name: "x", Version: "9.9.9"}},
	})
	in := []ToolPoolEntry{
		{Name: "x@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "x@2.0.0", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}
	kept, _, err := r.Resolve(in)
	require.NoError(t, err)
	require.Equal(t, 1, len(kept))
	assert.Equal(t, ToolSourceBuiltin, kept[0].Source)
}

func TestToolDedup_PreferSourceRank(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPreferSourceRank})
	in := []ToolPoolEntry{
		{Name: "x", Source: ToolSourceMCP, ProviderID: "p"},
		{Name: "x", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "x", Source: ToolSourceSkill, ProviderID: "p"},
	}
	kept, _, err := r.Resolve(in)
	require.NoError(t, err)
	require.Equal(t, 1, len(kept))
	assert.Equal(t, ToolSourceBuiltin, kept[0].Source)
}

func TestToolDedup_PerToolPolicyOverridesDefault(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{
		Policy:        ToolDedupPreferLatestVersion,
		PerToolPolicy: map[string]ToolDedupPolicy{"x": ToolDedupKeepAllVersions},
	})
	in := []ToolPoolEntry{
		{Name: "x@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "x@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "y@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "y@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
	}
	kept, _, err := r.Resolve(in)
	require.NoError(t, err)
	// x kept both, y kept latest only → 2+1=3.
	assert.Equal(t, 3, len(kept))
}

func TestToolDedup_StatefulToolsBypassDedup(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{
		Policy:        ToolDedupPreferLatestVersion,
		StatefulTools: map[string]bool{"session": true},
	})
	in := []ToolPoolEntry{
		{Name: "session@1.0.0", Source: ToolSourceSkill, ProviderID: "p"},
		{Name: "session@2.0.0", Source: ToolSourceSkill, ProviderID: "p"},
	}
	kept, results, err := r.Resolve(in)
	require.NoError(t, err)
	assert.Equal(t, 2, len(kept))
	assert.Contains(t, results[0].Reason, "stateful")
}

func TestToolDedup_BadEntryNameReturnsError(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPreferSourceRank})
	in := []ToolPoolEntry{{Name: "123bad", Source: ToolSourceBuiltin, ProviderID: "p"}}
	_, _, err := r.Resolve(in)
	assert.ErrorIs(t, err, ErrToolDedupBadName)
}

func TestToolDedup_DroppedEntriesReportedInResults(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPreferSourceRank})
	in := []ToolPoolEntry{
		{Name: "x", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "x", Source: ToolSourceSkill, ProviderID: "skill-shadow"},
	}
	_, results, err := r.Resolve(in)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	assert.Equal(t, 1, len(results[0].Dropped))
	assert.Equal(t, ToolSourceSkill, results[0].Dropped[0].Source)
}

func TestToolDedup_EmptyInputReturnsEmpty(t *testing.T) {
	r, _ := NewToolDedupResolver(ToolDedupConfig{Policy: ToolDedupPreferSourceRank})
	kept, results, err := r.Resolve(nil)
	require.NoError(t, err)
	assert.Empty(t, kept)
	assert.Empty(t, results)
}
