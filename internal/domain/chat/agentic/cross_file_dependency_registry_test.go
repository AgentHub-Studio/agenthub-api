package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCrossFileDependencyRegistry() *CrossFileDependencyRegistry {
	return NewCrossFileDependencyRegistry()
}

func TestCrossFileDependencyRegistry_NewReturnsNonNil(t *testing.T) {
	require.NotNil(t, newCrossFileDependencyRegistry())
}

func TestCrossFileDependencyRegistry_CountEqualsSeedCount(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	assert.Equal(t, SeedCrossFileDependencyCount, r.Count())
}

func TestCrossFileDependencyRegistry_CountIsSix(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	assert.Equal(t, 6, r.Count())
}

func TestCrossFileDependencyRegistry_SeedIDsLengthMatchesCount(t *testing.T) {
	assert.Len(t, SeedCrossFileDependencyIDs, SeedCrossFileDependencyCount)
}

func TestCrossFileDependencyRegistry_SeedIDsOrder(t *testing.T) {
	expected := []string{
		"query_engine_delegates_query",
		"query_imports_tool_services",
		"query_imports_compact_services",
		"query_engine_imports_memdir",
		"permission_types_break_import_cycle",
		"context_cached_claudemd_breaks_cycle",
	}
	assert.Equal(t, expected, SeedCrossFileDependencyIDs)
}

func TestCrossFileDependencyRegistry_AllReturnsSix(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	assert.Len(t, r.All(), 6)
}

func TestCrossFileDependencyRegistry_AllReturnsDefensiveCopy(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	first := r.All()
	first[0].DependencyID = "mutated"
	second := r.All()
	assert.Equal(t, "query_engine_delegates_query", second[0].DependencyID)
}

func TestCrossFileDependencyRegistry_FindByIDHit(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	p, ok := r.FindByID("query_engine_delegates_query")
	require.True(t, ok)
	assert.Equal(t, "QueryEngine.ts", p.Source)
	assert.Equal(t, "query.ts", p.Target)
	assert.Equal(t, CrossFileDependencyKindDelegation, p.Kind)
}

func TestCrossFileDependencyRegistry_FindByIDMiss(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	p, ok := r.FindByID("missing")
	assert.False(t, ok)
	assert.Nil(t, p)
}

func TestCrossFileDependencyRegistry_IsValidTrueForSeeds(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	for _, id := range SeedCrossFileDependencyIDs {
		assert.True(t, r.IsValid(id), "expected valid dependency ID %q", id)
	}
}

func TestCrossFileDependencyRegistry_IsValidFalseForUnknown(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	assert.False(t, r.IsValid("unknown_dependency"))
}

func TestCrossFileDependencyRegistry_DependenciesFromQuery(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.DependenciesFrom("query.ts")
	require.Len(t, deps, 2)
	targets := map[string]bool{}
	for _, dep := range deps {
		targets[dep.Target] = true
	}
	assert.True(t, targets["services/tools/"])
	assert.True(t, targets["services/compact/"])
}

func TestCrossFileDependencyRegistry_DependenciesFromQueryEngine(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.DependenciesFrom("QueryEngine.ts")
	require.Len(t, deps, 2)
	assert.Contains(t, []string{deps[0].Target, deps[1].Target}, "query.ts")
	assert.Contains(t, []string{deps[0].Target, deps[1].Target}, "memdir/")
}

func TestCrossFileDependencyRegistry_DependenciesToQuery(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.DependenciesTo("query.ts")
	require.Len(t, deps, 1)
	assert.Equal(t, "query_engine_delegates_query", deps[0].DependencyID)
}

func TestCrossFileDependencyRegistry_DependenciesByKindServiceImport(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.DependenciesByKind(CrossFileDependencyKindServiceImport)
	require.Len(t, deps, 2)
	for _, dep := range deps {
		assert.Equal(t, "query.ts", dep.Source)
	}
}

func TestCrossFileDependencyRegistry_DependenciesByKindCycleBreaker(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.DependenciesByKind(CrossFileDependencyKindCycleBreaker)
	assert.Len(t, deps, 2)
}

func TestCrossFileDependencyRegistry_CycleBreakers(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	breakers := r.CycleBreakers()
	require.Len(t, breakers, 2)
	for _, breaker := range breakers {
		assert.True(t, breaker.IsCycleBreaker)
		assert.NotEmpty(t, breaker.AvoidedCycle)
	}
}

func TestCrossFileDependencyRegistry_RuntimeCriticalDependencies(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	critical := r.RuntimeCriticalDependencies()
	require.Len(t, critical, 4)
	for _, dep := range critical {
		assert.True(t, dep.IsRuntimeCritical)
		assert.False(t, dep.IsCycleBreaker)
	}
}

func TestCrossFileDependencyRegistry_CoreLoopDependencies(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.CoreLoopDependencies()
	assert.Len(t, deps, 4)
}

func TestCrossFileDependencyRegistry_DependenciesTouchingQueryEngine(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.DependenciesTouching("QueryEngine.ts")
	assert.Len(t, deps, 2)
}

func TestCrossFileDependencyRegistry_DependenciesTouchingPermissionTypes(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	deps := r.DependenciesTouching("types/permissions.ts")
	require.Len(t, deps, 1)
	assert.True(t, deps[0].IsCycleBreaker)
}

func TestCrossFileDependencyRegistry_HasNoSelfDependencies(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	ok, violations := r.HasNoSelfDependencies()
	assert.True(t, ok)
	assert.Empty(t, violations)
}

func TestCrossFileDependencyRegistry_CycleBreakersHaveAvoidedCycle(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	ok, violations := r.CycleBreakersHaveAvoidedCycle()
	assert.True(t, ok)
	assert.Empty(t, violations)
}

func TestCrossFileDependencyRegistry_QueryEngineDelegatesToQuery(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	assert.True(t, r.QueryEngineDelegatesToQuery())
}

func TestCrossFileDependencyRegistry_AllProfilesHavePDFSection(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	ok, violations := r.AllProfilesHavePDFSection()
	assert.True(t, ok)
	assert.Empty(t, violations)
}

func TestCrossFileDependencyRegistry_AllProfilesHaveEvidence(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	for _, dep := range r.All() {
		assert.NotEmpty(t, dep.Evidence, "dependency %q must have evidence", dep.DependencyID)
	}
}

func TestCrossFileDependencyRegistry_AllProfilesHaveResponsibility(t *testing.T) {
	r := newCrossFileDependencyRegistry()
	for _, dep := range r.All() {
		assert.NotEmpty(t, dep.Responsibility, "dependency %q must have responsibility", dep.DependencyID)
	}
}

func TestCrossFileDependencyRegistry_KindConstants(t *testing.T) {
	assert.Equal(t, CrossFileDependencyKind("delegation"), CrossFileDependencyKindDelegation)
	assert.Equal(t, CrossFileDependencyKind("service_import"), CrossFileDependencyKindServiceImport)
	assert.Equal(t, CrossFileDependencyKind("memory_assembly"), CrossFileDependencyKindMemoryAssembly)
	assert.Equal(t, CrossFileDependencyKind("cycle_breaker"), CrossFileDependencyKindCycleBreaker)
}
