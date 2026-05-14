package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for Appendix A.3 cross-file dependency mapping.
func TestBDD_CrossFileDependencyRegistry(t *testing.T) {
	t.Run("Scenario_QueryEngineDelegatesToSharedQueryLoop", func(t *testing.T) {
		r := NewCrossFileDependencyRegistry()

		dep, ok := r.FindByID("query_engine_delegates_query")

		require.True(t, ok)
		assert.Equal(t, "QueryEngine.ts", dep.Source)
		assert.Equal(t, "query.ts", dep.Target)
		assert.Equal(t, CrossFileDependencyKindDelegation, dep.Kind)
		assert.True(t, dep.IsRuntimeCritical)
	})

	t.Run("Scenario_QueryImportsBothToolAndCompactionServices", func(t *testing.T) {
		r := NewCrossFileDependencyRegistry()

		deps := r.DependenciesFrom("query.ts")

		require.Len(t, deps, 2)
		targets := map[string]bool{}
		for _, dep := range deps {
			targets[dep.Target] = true
			assert.Equal(t, CrossFileDependencyKindServiceImport, dep.Kind)
		}
		assert.True(t, targets["services/tools/"])
		assert.True(t, targets["services/compact/"])
	})

	t.Run("Scenario_QueryEngineUsesMemdirForMemoryAssembly", func(t *testing.T) {
		r := NewCrossFileDependencyRegistry()

		dep, ok := r.FindByID("query_engine_imports_memdir")

		require.True(t, ok)
		assert.Equal(t, CrossFileDependencyKindMemoryAssembly, dep.Kind)
		assert.Equal(t, "memdir/", dep.Target)
		assert.Contains(t, dep.Responsibility, "Memory")
	})

	t.Run("Scenario_CycleBreakersExplainWhichCycleTheyAvoid", func(t *testing.T) {
		r := NewCrossFileDependencyRegistry()

		breakers := r.CycleBreakers()
		ok, violations := r.CycleBreakersHaveAvoidedCycle()

		require.Len(t, breakers, 2)
		assert.True(t, ok)
		assert.Empty(t, violations)
		for _, breaker := range breakers {
			assert.NotEmpty(t, breaker.AvoidedCycle)
			assert.False(t, breaker.IsRuntimeCritical)
		}
	})

	t.Run("Scenario_StructuralInvariantsHoldTogether", func(t *testing.T) {
		r := NewCrossFileDependencyRegistry()

		noSelf, selfViolations := r.HasNoSelfDependencies()
		cycleExplained, cycleViolations := r.CycleBreakersHaveAvoidedCycle()
		sectionsPresent, sectionViolations := r.AllProfilesHavePDFSection()

		assert.True(t, noSelf, "self dependency violations: %v", selfViolations)
		assert.Empty(t, selfViolations)
		assert.True(t, cycleExplained, "cycle breaker violations: %v", cycleViolations)
		assert.Empty(t, cycleViolations)
		assert.True(t, sectionsPresent, "PDF section violations: %v", sectionViolations)
		assert.Empty(t, sectionViolations)
		assert.True(t, r.QueryEngineDelegatesToQuery())
	})
}
