package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedQueryPipelineStep_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 9, SeedExpectedQueryPipelineStepRowCount)
}

func TestSeedQueryPipelineStep_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedQueryPipelineStepRowCount, len(SeedExpectedQueryPipelineStepSlugs))
}

func TestSeedQueryPipelineStep_SlugsContainSettingsResolution(t *testing.T) {
	assert.Contains(t, SeedExpectedQueryPipelineStepSlugs, "settings_resolution")
}

func TestSeedQueryPipelineStep_SlugsContainModelCall(t *testing.T) {
	assert.Contains(t, SeedExpectedQueryPipelineStepSlugs, "model_call")
}

func TestSeedQueryPipelineStep_SlugsContainPermissionGate(t *testing.T) {
	assert.Contains(t, SeedExpectedQueryPipelineStepSlugs, "permission_gate")
}

func TestSeedQueryPipelineStep_SlugsContainStopCondition(t *testing.T) {
	assert.Contains(t, SeedExpectedQueryPipelineStepSlugs, "stop_condition")
}

func TestSeedQueryPipelineStep_FivePhasesNamed(t *testing.T) {
	assert.Equal(t, 5, len(SeedQueryPipelinePhases))
	assert.Contains(t, SeedQueryPipelinePhases, "setup")
	assert.Contains(t, SeedQueryPipelinePhases, "context")
	assert.Contains(t, SeedQueryPipelinePhases, "reasoning")
	assert.Contains(t, SeedQueryPipelinePhases, "execution")
	assert.Contains(t, SeedQueryPipelinePhases, "termination")
}

func TestSeedQueryPipelineStep_BlockingStepsFourSlugs(t *testing.T) {
	assert.Equal(t, 4, len(SeedQueryPipelineBlockingStepSlugs))
}

func TestSeedQueryPipelineStep_RetryableStepsTwoSlugs(t *testing.T) {
	assert.Equal(t, 2, len(SeedQueryPipelineRetryableStepSlugs))
	assert.Contains(t, SeedQueryPipelineRetryableStepSlugs, "model_call")
	assert.Contains(t, SeedQueryPipelineRetryableStepSlugs, "tool_execution")
}

func TestSeedQueryPipelineStep_BlockingSlugsAreSubsetOfAllSlugs(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedQueryPipelineStepSlugs {
		all[s] = true
	}
	for _, s := range SeedQueryPipelineBlockingStepSlugs {
		assert.True(t, all[s], "blocking slug %q not in all-slugs list", s)
	}
}
