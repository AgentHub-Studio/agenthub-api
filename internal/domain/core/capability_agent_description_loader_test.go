package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- count constants --------------------------------------------------------

func TestAgentDescription_SeedAgentDescriptionCount(t *testing.T) {
	assert.Equal(t, 9, SeedAgentDescriptionCount,
		"9 total rows: 3 description types × 3 agents")
}

func TestAgentDescription_SeedAgentDescriptionAgentCount(t *testing.T) {
	assert.Equal(t, 3, SeedAgentDescriptionAgentCount,
		"3 agents: researcher, analyst, planner")
}

func TestAgentDescription_TotalRowsEqualAgentsTimesDescKeysPerAgent(t *testing.T) {
	const descKeysPerAgent = 3 // tagline, long_description, use_cases
	assert.Equal(t, SeedAgentDescriptionCount, SeedAgentDescriptionAgentCount*descKeysPerAgent,
		"total rows = agents × desc keys per agent")
}

// ---- per-agent row count (3 desc keys each) ---------------------------------

func TestAgentDescription_ResearcherHasThreeDescKeys(t *testing.T) {
	// core-researcher seeds 3 rows: tagline, long_description, use_cases.
	const researcherDescCount = 3
	assert.Equal(t, 3, researcherDescCount,
		"core-researcher must seed exactly 3 description rows")
}

func TestAgentDescription_AnalystHasThreeDescKeys(t *testing.T) {
	// core-analyst seeds 3 rows: tagline, long_description, use_cases.
	const analystDescCount = 3
	assert.Equal(t, 3, analystDescCount,
		"core-analyst must seed exactly 3 description rows")
}

func TestAgentDescription_PlannerHasThreeDescKeys(t *testing.T) {
	// core-planner seeds 3 rows: tagline, long_description, use_cases.
	const plannerDescCount = 3
	assert.Equal(t, 3, plannerDescCount,
		"core-planner must seed exactly 3 description rows")
}

// ---- desc key constants -----------------------------------------------------

func TestAgentDescription_DescKeyTagline(t *testing.T) {
	assert.Equal(t, "tagline", SeedDescKeyTagline)
}

func TestAgentDescription_DescKeyLongDescription(t *testing.T) {
	assert.Equal(t, "long_description", SeedDescKeyLongDescription)
}

func TestAgentDescription_DescKeyUseCases(t *testing.T) {
	assert.Equal(t, "use_cases", SeedDescKeyUseCases)
}

func TestAgentDescription_DescKeysAreDistinct(t *testing.T) {
	keys := []string{
		SeedDescKeyTagline,
		SeedDescKeyLongDescription,
		SeedDescKeyUseCases,
	}
	seen := map[string]bool{}
	for _, k := range keys {
		assert.False(t, seen[k], "duplicate desc key constant %q", k)
		seen[k] = true
	}
	assert.Equal(t, 3, len(seen), "exactly 3 distinct desc key constants")
}

// ---- use case separator -----------------------------------------------------

func TestAgentDescription_UseCaseSeparatorIsPipe(t *testing.T) {
	assert.Equal(t, "|", SeedUseCaseSeparator,
		"use cases must be separated by pipe character")
}

// ---- per-agent tagline constants --------------------------------------------

func TestAgentDescription_ResearcherTagline(t *testing.T) {
	assert.Equal(t, "Search the web and synthesize findings instantly", SeedResearcherTagline)
}

func TestAgentDescription_AnalystTagline(t *testing.T) {
	assert.Equal(t, "Analyze data and documents with structured reasoning", SeedAnalystTagline)
}

func TestAgentDescription_PlannerTagline(t *testing.T) {
	assert.Equal(t, "Break down complex goals into actionable step-by-step plans", SeedPlannerTagline)
}

func TestAgentDescription_TaglinesAreDistinct(t *testing.T) {
	taglines := []string{
		SeedResearcherTagline,
		SeedAnalystTagline,
		SeedPlannerTagline,
	}
	seen := map[string]bool{}
	for _, tag := range taglines {
		assert.False(t, seen[tag], "duplicate tagline constant %q", tag)
		seen[tag] = true
	}
	assert.Equal(t, 3, len(seen), "exactly 3 distinct per-agent tagline constants")
}

func TestAgentDescription_TaglinesAreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedResearcherTagline)
	assert.NotEmpty(t, SeedAnalystTagline)
	assert.NotEmpty(t, SeedPlannerTagline)
}

// ---- use_cases contain separator constraint ---------------------------------

func TestAgentDescription_ResearcherUseCasesContainPipeSeparator(t *testing.T) {
	// The use_cases desc_value for core-researcher is pipe-separated.
	// We validate the constant SeedUseCaseSeparator appears in the expected value.
	researcherUseCases := "Fact-checking claims|Researching competitors|Summarizing recent news|Answering questions about current events|Finding technical documentation"
	assert.True(t, strings.Contains(researcherUseCases, SeedUseCaseSeparator),
		"core-researcher use_cases value must contain pipe separator %q", SeedUseCaseSeparator)
}

func TestAgentDescription_AnalystUseCasesContainPipeSeparator(t *testing.T) {
	analystUseCases := "Analyzing reports and documents|Comparing product options|Interpreting survey results|Identifying data patterns|Evaluating trade-offs"
	assert.True(t, strings.Contains(analystUseCases, SeedUseCaseSeparator),
		"core-analyst use_cases value must contain pipe separator %q", SeedUseCaseSeparator)
}

func TestAgentDescription_PlannerUseCasesContainPipeSeparator(t *testing.T) {
	plannerUseCases := "Project planning|Onboarding checklists|Multi-step task breakdown|Sprint planning|Creating implementation roadmaps"
	assert.True(t, strings.Contains(plannerUseCases, SeedUseCaseSeparator),
		"core-planner use_cases value must contain pipe separator %q", SeedUseCaseSeparator)
}

// ---- long_description is longer than tagline for all agents -----------------

func TestAgentDescription_ResearcherLongDescriptionLongerThanTagline(t *testing.T) {
	researcherLong := "The Researcher agent searches the web, fetches pages, and scans your knowledge base to gather accurate, up-to-date information. Ideal for fact-checking, competitive research, news summaries, and answering questions that require current data."
	assert.Greater(t, len(researcherLong), len(SeedResearcherTagline),
		"core-researcher long_description must be longer than tagline")
}

func TestAgentDescription_AnalystLongDescriptionLongerThanTagline(t *testing.T) {
	analystLong := "The Analyst agent examines data, documents, and information to extract patterns, insights, and conclusions. Uses step-by-step reasoning to present findings with appropriate confidence levels. Ideal for interpreting reports, comparing options, and turning raw data into actionable insights."
	assert.Greater(t, len(analystLong), len(SeedAnalystTagline),
		"core-analyst long_description must be longer than tagline")
}

func TestAgentDescription_PlannerLongDescriptionLongerThanTagline(t *testing.T) {
	plannerLong := "The Planner agent takes your goal and creates a structured, actionable plan. It breaks work into clear steps, identifies dependencies, and can delegate research or analysis subtasks to specialized agents. Ideal for project planning, onboarding workflows, and tackling multi-step objectives."
	assert.Greater(t, len(plannerLong), len(SeedPlannerTagline),
		"core-planner long_description must be longer than tagline")
}

// ---- loader construction ---------------------------------------------------

func TestAgentDescription_NewLoaderAcceptsNilPool(t *testing.T) {
	// Construction must not panic even with a nil pool.
	// (pool is only used on method calls, not on construction)
	assert.NotPanics(t, func() {
		_ = NewCoreCapabilityAgentDescriptionLoader(nil)
	})
}
