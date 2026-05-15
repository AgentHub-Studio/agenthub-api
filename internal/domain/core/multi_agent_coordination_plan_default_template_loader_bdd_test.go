package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreMACPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksCoordinationPlanFromCatalog", func(t *testing.T) {
		// Given fresh tenants must construct SUB-011 CoordinationPlans
		// for multi-subagent orchestration but inventing strategy +
		// failure_policy + parallelism combinations is error-prone,
		// When admin opens coordination-plan onboarding,
		// Then 5 recommended templates surface across strategy variety.
		assert.Equal(t, 5, len(SeedRecommendedMACPDTemplateSlugs))
	})

	t.Run("Scenario_SequentialPipelineForLinearWorkflows", func(t *testing.T) {
		// Given a linear workflow (ETL: extract → transform → load),
		// When admin uses sequential-pipeline,
		// Then strategy=sequential, abort_on_failure (one broken step
		// kills the chain), parallelism=1.
		assert.Contains(t, SeedExpectedMACPDTemplateSlugs, "sequential-pipeline")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewMACPDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["sequential-pipeline"])
	})

	t.Run("Scenario_ParallelFanoutForIndependentResearch", func(t *testing.T) {
		// Given 5 independent searches/analyses,
		// When admin uses parallel-fanout,
		// Then strategy=parallel, continue_on_failure (one bad branch
		// doesn't kill batch), parallelism=5.
		assert.Contains(t, SeedExpectedMACPDTemplateSlugs, "parallel-fanout")
	})

	t.Run("Scenario_DAGBuildTestDeployRequiresAdminReview", func(t *testing.T) {
		// Given CI/CD diamond DAG (build → test+lint → deploy),
		// When admin uses dag-build-test-deploy,
		// Then strategy=dag, skip_downstream_on_failure, admin review
		// (deployment automation has blast radius).
		assert.Contains(t, SeedExpectedMACPDTemplateSlugs, "dag-build-test-deploy")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewMACPDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["dag-build-test-deploy"])
	})

	t.Run("Scenario_PipelineExtractSummarizeForDataPipelines", func(t *testing.T) {
		// Given a two-stage data pipeline (extract emits → summarizer
		// consumes),
		// When admin uses pipeline-extract-summarize,
		// Then strategy=pipeline, abort_on_failure (failed extract
		// leaves summarizer with nothing).
		assert.Contains(t, SeedExpectedMACPDTemplateSlugs, "pipeline-extract-summarize")
	})

	t.Run("Scenario_DAGWithSkipDownstreamForComplexFlows", func(t *testing.T) {
		// Given general DAG with 6 tasks across 3 stages,
		// When admin uses dag-with-skip-downstream,
		// Then skip_downstream lets independent branches finish.
		assert.Contains(t, SeedExpectedMACPDTemplateSlugs, "dag-with-skip-downstream")
	})

	t.Run("Scenario_StrategyLabelsMatchSUB011EnumByteForByte", func(t *testing.T) {
		// Given SUB-011 MultiAgentStrategy has 4 values,
		// When seed declares target_strategy,
		// Then labels match enum bytes.
		sub011 := []string{"sequential", "parallel", "pipeline", "dag"}
		set := map[string]bool{}
		for _, s := range SeedExpectedMACPDTemplateStrategies {
			set[s] = true
		}
		for _, e := range sub011 {
			assert.True(t, set[e], "SUB-011 strategy %q missing", e)
		}
	})

	t.Run("Scenario_FailurePolicyLabelsMatchSUB011Enum", func(t *testing.T) {
		// Given SUB-011 MultiAgentFailurePolicy has 3 values,
		// When seed declares target_failure_policy,
		// Then labels match enum bytes.
		sub011 := []string{"abort_on_failure", "continue_on_failure", "skip_downstream_on_failure"}
		set := map[string]bool{}
		for _, p := range SeedExpectedMACPDTemplateFailurePolicies {
			set[p] = true
		}
		for _, e := range sub011 {
			assert.True(t, set[e], "SUB-011 failure policy %q missing", e)
		}
	})

	t.Run("Scenario_DAGStrategyImpliesDagDependenciesFlag", func(t *testing.T) {
		// Given dag strategy templates have task dependencies,
		// When admin inspects has_dag_dependencies,
		// Then dag templates → true; sequential/parallel → false.
		// Validated DB-real in integration test.
		assert.Equal(t, 5, SeedExpectedMACPDTemplateRowCount)
	})

	t.Run("Scenario_DAGStrategiesRequireAdminReview", func(t *testing.T) {
		// Given DAG topology can mask responsibility chains,
		// When admin compares admin-review subset,
		// Then both DAG templates are listed; sequential/parallel/pipeline are routine.
		assert.Equal(t, 2, len(SeedAdminReviewMACPDTemplateSlugs))
	})

	t.Run("Scenario_MaxParallelismRespectsBoundedFanOut", func(t *testing.T) {
		// Given max_parallelism caps resource usage,
		// When admin compares templates,
		// Then sequential/pipeline → 1; parallel → 5; dag → 3-4.
		// Validated DB-real in integration test.
		assert.Equal(t, 5, SeedExpectedMACPDTemplateRowCount)
	})
}
