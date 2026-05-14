package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for Table 6 context-management approach taxonomy (arXiv:2604.14228v1, §13.2).

func TestBDD_ContextManagementApproachRegistry(t *testing.T) {
	t.Run("Scenario_FiveApproachesMatchTable6RowCount", func(t *testing.T) {
		// Given Table 6 defines five context-management strategies
		// When all approaches are listed from the registry
		// Then exactly five profiles are returned in Table 6 order
		r := NewContextManagementApproachRegistry()
		all := r.AllApproaches()
		assert.Equal(t, 5, len(all))
		assert.Equal(t, ContextMgmtSimpleTruncation, all[0].Approach)
		assert.Equal(t, ContextMgmtGraduatedCompaction, all[4].Approach)
	})

	t.Run("Scenario_OnlyGraduatedCompactionHasVeryFineGranularity", func(t *testing.T) {
		// Given Table 6 marks graduated compaction as the only "Very fine" entry
		// When all approaches at very_fine granularity are queried
		// Then exactly one approach is returned and it is graduated_compaction
		r := NewContextManagementApproachRegistry()
		vf := r.ApproachesAtGranularity(ContextMgmtGranularityVeryFine)
		assert.Equal(t, 1, len(vf))
		assert.Equal(t, ContextMgmtGraduatedCompaction, vf[0].Approach)
	})

	t.Run("Scenario_TwoCoarseApproachesMatchTable6", func(t *testing.T) {
		// Given Table 6 marks simple_truncation and single_summarization as Coarse
		// When approaches at coarse granularity are queried
		// Then exactly two approaches are returned
		r := NewContextManagementApproachRegistry()
		coarse := r.ApproachesAtGranularity(ContextMgmtGranularityCoarse)
		assert.Equal(t, 2, len(coarse))
	})

	t.Run("Scenario_AgentHubUsesGraduatedCompactionFromSection73", func(t *testing.T) {
		// Given AgentHub adapts Claude Code's five-stage compaction pipeline (§7.3)
		// When the AgentHub approach is retrieved
		// Then it returns graduated_compaction with very_fine granularity
		r := NewContextManagementApproachRegistry()
		p := r.AgentHubApproach()
		assert.Equal(t, ContextMgmtGraduatedCompaction, p.Approach)
		assert.True(t, p.IsAgentHubApproach)
	})

	t.Run("Scenario_InvalidApproachIDRejected", func(t *testing.T) {
		// Given only the five Table 6 slugs are valid
		// When an unrecognised identifier is passed
		// Then IsValidApproach returns false and Profile returns false
		r := NewContextManagementApproachRegistry()
		assert.False(t, r.IsValidApproach("not_a_real_approach"))
		_, ok := r.Profile("not_a_real_approach")
		assert.False(t, ok)
	})
}
