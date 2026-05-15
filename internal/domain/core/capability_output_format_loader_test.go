package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability output format seed constants (migration 000115).
// These run without a database and guard against accidental constant drift.

func TestSeedOutputFormatCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedOutputFormatCount,
		"migration 000115 seeds exactly 9 capability output format rows (three per capability agent)")
}

func TestSeedOutputFormatAgentCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedOutputFormatAgentCount,
		"SeedOutputFormatAgentCount must be 3 — researcher, analyst, planner")
}

func TestSeedResearcherOutputFormatCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherOutputFormatCount,
		"SeedResearcherOutputFormatCount must be 3 — default_format + citation_style + summary_position")
}

func TestSeedAnalystOutputFormatCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAnalystOutputFormatCount,
		"SeedAnalystOutputFormatCount must be 3 — default_format + number_format + uncertainty_notation")
}

func TestSeedPlannerOutputFormatCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedPlannerOutputFormatCount,
		"SeedPlannerOutputFormatCount must be 3 — default_format + step_numbering + code_blocks")
}

func TestSeedPerAgentOutputFormatCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherOutputFormatCount + SeedAnalystOutputFormatCount + SeedPlannerOutputFormatCount
	assert.Equal(t, SeedOutputFormatCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherOutputFormatCount, SeedAnalystOutputFormatCount, SeedPlannerOutputFormatCount,
		SeedOutputFormatCount)
}

func TestSeedOutputFormatCount_EqualsAgentCountTimesThree(t *testing.T) {
	assert.Equal(t, SeedOutputFormatAgentCount*3, SeedOutputFormatCount,
		"total format count must equal agent count × 3 (each agent has exactly 3 format rows)")
}

// Format key constant tests.

func TestSeedFormatKeyDefault_Value(t *testing.T) {
	assert.Equal(t, "default_format", SeedFormatKeyDefault,
		"SeedFormatKeyDefault must equal \"default_format\"")
}

func TestSeedFormatKeyCitation_Value(t *testing.T) {
	assert.Equal(t, "citation_style", SeedFormatKeyCitation,
		"SeedFormatKeyCitation must equal \"citation_style\"")
}

func TestSeedFormatKeySummaryPosition_Value(t *testing.T) {
	assert.Equal(t, "summary_position", SeedFormatKeySummaryPosition,
		"SeedFormatKeySummaryPosition must equal \"summary_position\"")
}

func TestSeedFormatKeyNumberFormat_Value(t *testing.T) {
	assert.Equal(t, "number_format", SeedFormatKeyNumberFormat,
		"SeedFormatKeyNumberFormat must equal \"number_format\"")
}

func TestSeedFormatKeyUncertainty_Value(t *testing.T) {
	assert.Equal(t, "uncertainty_notation", SeedFormatKeyUncertainty,
		"SeedFormatKeyUncertainty must equal \"uncertainty_notation\"")
}

func TestSeedFormatKeyStepNumbering_Value(t *testing.T) {
	assert.Equal(t, "step_numbering", SeedFormatKeyStepNumbering,
		"SeedFormatKeyStepNumbering must equal \"step_numbering\"")
}

func TestSeedFormatKeyCodeBlocks_Value(t *testing.T) {
	assert.Equal(t, "code_blocks", SeedFormatKeyCodeBlocks,
		"SeedFormatKeyCodeBlocks must equal \"code_blocks\"")
}

func TestSeedFormatKeys_AreDistinct(t *testing.T) {
	keys := []string{
		SeedFormatKeyDefault,
		SeedFormatKeyCitation,
		SeedFormatKeySummaryPosition,
		SeedFormatKeyNumberFormat,
		SeedFormatKeyUncertainty,
		SeedFormatKeyStepNumbering,
		SeedFormatKeyCodeBlocks,
	}
	seen := map[string]struct{}{}
	for _, k := range keys {
		assert.NotEmpty(t, k, "every format key constant must be non-empty")
		seen[k] = struct{}{}
	}
	assert.Len(t, seen, 7,
		"there must be exactly 7 distinct format key constants")
}

// Per-agent default format value tests.

func TestSeedResearcherDefaultFormat_IsMarkdown(t *testing.T) {
	assert.Equal(t, "markdown", SeedResearcherDefaultFormat,
		"SeedResearcherDefaultFormat must equal \"markdown\"")
	assert.NotEmpty(t, SeedResearcherDefaultFormat,
		"SeedResearcherDefaultFormat must not be empty")
}

func TestSeedAnalystDefaultFormat_IsStructured(t *testing.T) {
	assert.Equal(t, "structured", SeedAnalystDefaultFormat,
		"SeedAnalystDefaultFormat must equal \"structured\"")
	assert.NotEmpty(t, SeedAnalystDefaultFormat,
		"SeedAnalystDefaultFormat must not be empty")
}

func TestSeedPlannerDefaultFormat_IsChecklist(t *testing.T) {
	assert.Equal(t, "checklist", SeedPlannerDefaultFormat,
		"SeedPlannerDefaultFormat must equal \"checklist\"")
	assert.NotEmpty(t, SeedPlannerDefaultFormat,
		"SeedPlannerDefaultFormat must not be empty")
}

func TestSeedDefaultFormats_AreDistinct(t *testing.T) {
	formats := []string{
		SeedResearcherDefaultFormat,
		SeedAnalystDefaultFormat,
		SeedPlannerDefaultFormat,
	}
	seen := map[string]struct{}{}
	for _, f := range formats {
		seen[f] = struct{}{}
	}
	assert.Len(t, seen, 3,
		"researcher (%q), analyst (%q), planner (%q) must have distinct default formats",
		SeedResearcherDefaultFormat, SeedAnalystDefaultFormat, SeedPlannerDefaultFormat)
}

// Display order constant tests.

func TestSeedOutputFormatDisplayOrderFirst_IsOne(t *testing.T) {
	assert.Equal(t, 1, SeedOutputFormatDisplayOrderFirst,
		"SeedOutputFormatDisplayOrderFirst must be 1 (primary format setting)")
}

func TestSeedOutputFormatDisplayOrderSecond_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedOutputFormatDisplayOrderSecond,
		"SeedOutputFormatDisplayOrderSecond must be 2 (secondary format setting)")
}

func TestSeedOutputFormatDisplayOrderThird_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedOutputFormatDisplayOrderThird,
		"SeedOutputFormatDisplayOrderThird must be 3 (tertiary format setting)")
}

func TestSeedOutputFormatDisplayOrders_AreSequential(t *testing.T) {
	assert.Less(t, SeedOutputFormatDisplayOrderFirst, SeedOutputFormatDisplayOrderSecond,
		"first display_order must be less than second")
	assert.Less(t, SeedOutputFormatDisplayOrderSecond, SeedOutputFormatDisplayOrderThird,
		"second display_order must be less than third")
	assert.Equal(t, SeedOutputFormatDisplayOrderFirst+1, SeedOutputFormatDisplayOrderSecond,
		"display orders must be strictly sequential: second = first + 1")
	assert.Equal(t, SeedOutputFormatDisplayOrderSecond+1, SeedOutputFormatDisplayOrderThird,
		"display orders must be strictly sequential: third = second + 1")
}
