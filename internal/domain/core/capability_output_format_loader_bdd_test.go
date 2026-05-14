package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000115 capability output format seeds.
// These assert seed shape and output format rationale without a database.

func TestBDD_CapabilityOutputFormatSeed(t *testing.T) {
	t.Run("Scenario_NineFormatsAcrossThreeAgents", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent output format
		//   preferences to ensure each capability agent produces consistently
		//   structured and styled responses,
		// When migration 000115 seeds capability_output_format rows,
		// Then exactly 9 rows are added — three per agent — and per-agent counts
		//   sum to the total count constant, covering researcher, analyst, and
		//   planner with balanced format sets.
		assert.Equal(t, 9, SeedOutputFormatCount,
			"migration 000115 must seed exactly 9 capability output format rows")

		sum := SeedResearcherOutputFormatCount + SeedAnalystOutputFormatCount + SeedPlannerOutputFormatCount
		assert.Equal(t, SeedOutputFormatCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherOutputFormatCount, SeedAnalystOutputFormatCount,
			SeedPlannerOutputFormatCount, SeedOutputFormatCount)

		assert.Equal(t, 3, SeedOutputFormatAgentCount,
			"SeedOutputFormatAgentCount must be 3 — researcher, analyst, planner")
	})

	t.Run("Scenario_ThreeFormatsPerAgent", func(t *testing.T) {
		// Given each capability agent has a distinct role (research, analysis,
		//   planning) that requires a focused set of output format preferences,
		// When migration 000115 seeds output format rows,
		// Then each agent receives exactly 3 format rows — one primary
		//   (default_format) and two supplementary — so agents have a complete
		//   but non-overwhelming format specification.
		assert.Equal(t, 3, SeedResearcherOutputFormatCount,
			"core-researcher must have exactly 3 output format rows")
		assert.Equal(t, 3, SeedAnalystOutputFormatCount,
			"core-analyst must have exactly 3 output format rows")
		assert.Equal(t, 3, SeedPlannerOutputFormatCount,
			"core-planner must have exactly 3 output format rows")

		// Per-agent counts are equal — balanced format sets.
		assert.Equal(t, SeedResearcherOutputFormatCount, SeedAnalystOutputFormatCount,
			"researcher and analyst must have equal format row counts (3 each)")
		assert.Equal(t, SeedAnalystOutputFormatCount, SeedPlannerOutputFormatCount,
			"analyst and planner must have equal format row counts (3 each)")
	})

	t.Run("Scenario_ResearcherUsesMarkdownByDefault", func(t *testing.T) {
		// Given the core-researcher agent produces findings, citations, and
		//   summaries that benefit from rich formatting — headers separate topic
		//   sections, bullet lists enumerate sources, and inline code spans
		//   highlight technical terms — making markdown the natural default,
		// When migration 000115 seeds output format rows for core-researcher,
		// Then the default_format key has value "markdown" at display_order 1,
		//   the citation_style key is "inline" for [Source: URL] notation, and
		//   the summary_position key is "top" so executives see the bottom line
		//   before diving into detailed findings.
		assert.Equal(t, "markdown", SeedResearcherDefaultFormat,
			"SeedResearcherDefaultFormat must equal \"markdown\"")

		assert.Equal(t, "default_format", SeedFormatKeyDefault,
			"SeedFormatKeyDefault must equal \"default_format\"")
		assert.Equal(t, "citation_style", SeedFormatKeyCitation,
			"SeedFormatKeyCitation must equal \"citation_style\"")
		assert.Equal(t, "summary_position", SeedFormatKeySummaryPosition,
			"SeedFormatKeySummaryPosition must equal \"summary_position\"")

		// Display order 1 → default_format is always the primary key.
		assert.Equal(t, 1, SeedOutputFormatDisplayOrderFirst,
			"SeedOutputFormatDisplayOrderFirst must be 1 (default_format is always primary)")
	})

	t.Run("Scenario_PlannerUsesChecklistFormat", func(t *testing.T) {
		// Given the core-planner agent generates task lists, implementation plans,
		//   and step-by-step roadmaps that users need to execute directly — and
		//   markdown checklist syntax ([ ] boxes) makes plans immediately
		//   actionable without post-processing,
		// When migration 000115 seeds output format rows for core-planner,
		// Then the default_format key has value "checklist" at display_order 1,
		//   step_numbering is "sequential" for numbered steps (1. 2. 3.), and
		//   code_blocks is "always" so all code snippets are wrapped in fenced
		//   blocks with a language tag — preventing ambiguous inline code.
		assert.Equal(t, "checklist", SeedPlannerDefaultFormat,
			"SeedPlannerDefaultFormat must equal \"checklist\"")

		assert.Equal(t, "step_numbering", SeedFormatKeyStepNumbering,
			"SeedFormatKeyStepNumbering must equal \"step_numbering\"")
		assert.Equal(t, "code_blocks", SeedFormatKeyCodeBlocks,
			"SeedFormatKeyCodeBlocks must equal \"code_blocks\"")

		// Planner default format is distinct from researcher and analyst.
		assert.NotEqual(t, SeedPlannerDefaultFormat, SeedResearcherDefaultFormat,
			"planner default format must differ from researcher")
		assert.NotEqual(t, SeedPlannerDefaultFormat, SeedAnalystDefaultFormat,
			"planner default format must differ from analyst")
	})

	t.Run("Scenario_AnalystUsesStructuredFormat", func(t *testing.T) {
		// Given the core-analyst agent produces structured data analyses,
		//   quantitative assessments, and comparative evaluations — where labeled
		//   headings separate hypothesis, method, data, and conclusion sections —
		//   making "structured" format the clearest default for analytical output,
		// When migration 000115 seeds output format rows for core-analyst,
		// Then the default_format key has value "structured" at display_order 1,
		//   number_format is "grouped" so large numbers use thousands separators
		//   (1,234,567 not 1234567), and uncertainty_notation is "range" so
		//   uncertainty is expressed as 80-90% not a single point estimate.
		assert.Equal(t, "structured", SeedAnalystDefaultFormat,
			"SeedAnalystDefaultFormat must equal \"structured\"")

		assert.Equal(t, "number_format", SeedFormatKeyNumberFormat,
			"SeedFormatKeyNumberFormat must equal \"number_format\"")
		assert.Equal(t, "uncertainty_notation", SeedFormatKeyUncertainty,
			"SeedFormatKeyUncertainty must equal \"uncertainty_notation\"")

		// Analyst default format is distinct from researcher and planner.
		assert.NotEqual(t, SeedAnalystDefaultFormat, SeedResearcherDefaultFormat,
			"analyst default format must differ from researcher")
		assert.NotEqual(t, SeedAnalystDefaultFormat, SeedPlannerDefaultFormat,
			"analyst default format must differ from planner")

		// Display orders are sequential for all agents.
		assert.Equal(t, 1, SeedOutputFormatDisplayOrderFirst)
		assert.Equal(t, 2, SeedOutputFormatDisplayOrderSecond)
		assert.Equal(t, 3, SeedOutputFormatDisplayOrderThird)
	})
}
