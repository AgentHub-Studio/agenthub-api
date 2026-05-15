package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent persona seed constants (migration 000104).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAgentPersonaCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilityAgentPersonaCount,
		"migration 000104 seeds exactly 3 capability agent persona rows")
}

func TestSeedCapabilityAgentPersonaSlugs_HasLengthThree(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentPersonaSlugs, 3,
		"SeedCapabilityAgentPersonaSlugs must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilityAgentPersonaSlugs_AllStartWithCapabilityPrefix(t *testing.T) {
	for _, slug := range SeedCapabilityAgentPersonaSlugs {
		assert.True(t, strings.HasPrefix(slug, "capability-"),
			"persona slug %q must start with 'capability-' (capability-layer namespace contract)", slug)
	}
}

func TestSeedCapabilityAgentPersonaSlugs_AllEndWithPersonaSuffix(t *testing.T) {
	for _, slug := range SeedCapabilityAgentPersonaSlugs {
		assert.True(t, strings.HasSuffix(slug, "-persona"),
			"persona slug %q must end with '-persona' (persona type discriminator)", slug)
	}
}

func TestSeedResearcherPersonaSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentPersonaSlugs, SeedResearcherPersonaSlug,
		"SeedResearcherPersonaSlug must be present in SeedCapabilityAgentPersonaSlugs")
}

func TestSeedAnalystPersonaSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentPersonaSlugs, SeedAnalystPersonaSlug,
		"SeedAnalystPersonaSlug must be present in SeedCapabilityAgentPersonaSlugs")
}

func TestSeedPlannerPersonaSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentPersonaSlugs, SeedPlannerPersonaSlug,
		"SeedPlannerPersonaSlug must be present in SeedCapabilityAgentPersonaSlugs")
}

func TestSeedResearcherTone_IsCurious(t *testing.T) {
	assert.Equal(t, "curious", SeedResearcherTone,
		"SeedResearcherTone must be \"curious\" — curiosity drives thorough information gathering")
}

func TestSeedAnalystTone_IsPrecise(t *testing.T) {
	assert.Equal(t, "precise", SeedAnalystTone,
		"SeedAnalystTone must be \"precise\" — precision ensures evidence-based analysis")
}

func TestSeedPlannerTone_IsPragmatic(t *testing.T) {
	assert.Equal(t, "pragmatic", SeedPlannerTone,
		"SeedPlannerTone must be \"pragmatic\" — pragmatism drives actionable task decomposition")
}

func TestSeedPersonaTones_AreAllDistinct(t *testing.T) {
	tones := []string{
		SeedResearcherTone,
		SeedAnalystTone,
		SeedPlannerTone,
	}
	unique := map[string]struct{}{}
	for _, tone := range tones {
		unique[tone] = struct{}{}
	}
	assert.Len(t, unique, 3,
		"all 3 persona tone constants must be distinct strings — each capability agent has a unique identity")
}

func TestSeedResearcherTraitCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedResearcherTraitCount,
		"SeedResearcherTraitCount must be 4 — researcher persona has 4 trait labels")
}

func TestSeedAnalystTraitCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedAnalystTraitCount,
		"SeedAnalystTraitCount must be 4 — analyst persona has 4 trait labels")
}

func TestSeedPlannerTraitCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedPlannerTraitCount,
		"SeedPlannerTraitCount must be 4 — planner persona has 4 trait labels")
}

func TestSeedTotalTraitCount_IsTwelve(t *testing.T) {
	assert.Equal(t, 12, SeedTotalTraitCount,
		"SeedTotalTraitCount must be 12 — 3 personas × 4 traits each")
}

func TestSeedTotalTraitCount_EqualsPerPersonaSum(t *testing.T) {
	sum := SeedResearcherTraitCount + SeedAnalystTraitCount + SeedPlannerTraitCount
	assert.Equal(t, SeedTotalTraitCount, sum,
		"SeedTotalTraitCount must equal the sum of all per-persona trait counts (%d+%d+%d=%d)",
		SeedResearcherTraitCount, SeedAnalystTraitCount, SeedPlannerTraitCount, sum)
}

func TestSeedCapabilityAgentPersonaSlugs_AllAreDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, slug := range SeedCapabilityAgentPersonaSlugs {
		unique[slug] = struct{}{}
	}
	assert.Len(t, unique, SeedCapabilityAgentPersonaCount,
		"all entries in SeedCapabilityAgentPersonaSlugs must be distinct (no duplicates)")
}

func TestSeedResearcherPersonaSlug_LinksToResearcherAgent(t *testing.T) {
	// The researcher persona slug encodes its agent affiliation in the name:
	// capability-researcher-persona → core-researcher agent.
	assert.Contains(t, SeedResearcherPersonaSlug, "researcher",
		"SeedResearcherPersonaSlug must contain 'researcher' to signal its core-researcher agent affiliation")
}

func TestSeedCapabilityAgentPersonaCount_EqualsSlugSliceLength(t *testing.T) {
	assert.Equal(t, SeedCapabilityAgentPersonaCount, len(SeedCapabilityAgentPersonaSlugs),
		"SeedCapabilityAgentPersonaCount must equal len(SeedCapabilityAgentPersonaSlugs)")
}
