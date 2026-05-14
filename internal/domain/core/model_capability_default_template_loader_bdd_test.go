package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for ModelCapabilityDefaultTemplate seed.
// Model capability templates catalog the Anthropic Claude model lineup,
// exposing context window size, max output tokens, family, and recommended
// use cases to the AgentHub model selector.

func TestBDD_AhCoreModelCapabilitySeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesThreeModelEntries", func(t *testing.T) {
		// Given a new tenant needs to select a Claude model for their agents
		// When the model capability catalog is loaded
		// Then exactly 3 models are available (haiku, sonnet, opus families)
		assert.Equal(t, 3, SeedExpectedModelCapRowCount)
		assert.Equal(t, 3, len(SeedExpectedModelCapModelIDs))
	})

	t.Run("Scenario_ThreeClaudeFamiliesAreRepresented", func(t *testing.T) {
		// Given the Claude model lineup spans haiku/sonnet/opus capability tiers
		// When the model family taxonomy is inspected
		// Then all three families are present in the seed
		assert.Equal(t, 3, len(SeedModelCapFamilies))
		familySet := map[string]bool{}
		for _, f := range SeedModelCapFamilies {
			familySet[f] = true
		}
		assert.True(t, familySet["haiku"])
		assert.True(t, familySet["sonnet"])
		assert.True(t, familySet["opus"])
	})

	t.Run("Scenario_AllModelIDsContainFamilyName", func(t *testing.T) {
		// Given model IDs follow the claude-{family}-{version} naming convention
		// When each seeded model ID is inspected
		// Then every model ID contains at least one family name as a substring
		familySet := map[string]bool{}
		for _, f := range SeedModelCapFamilies {
			familySet[f] = true
		}
		for _, id := range SeedExpectedModelCapModelIDs {
			found := false
			for family := range familySet {
				if len(id) >= len(family) {
					for i := 0; i <= len(id)-len(family); i++ {
						if id[i:i+len(family)] == family {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			assert.True(t, found, "model ID %q does not contain a known family name", id)
		}
	})

	t.Run("Scenario_AllModelsShareThe200KContextWindow", func(t *testing.T) {
		// Given Anthropic Claude 4.x models share a 200,000-token context window
		// When the context window constant is inspected
		// Then the seeded value is exactly 200,000
		assert.Equal(t, 200_000, SeedModelCapMaxContextWindow)
	})

	t.Run("Scenario_FamilyNamesMustBeLowerAlpha", func(t *testing.T) {
		// Given family names should be simple lowercase identifiers
		// When each family name is validated against the regex
		// Then all family names match ^[a-z]+$
		for _, f := range SeedModelCapFamilies {
			assert.True(t, SeedModelCapFamilyRE.MatchString(f),
				"family %q violates lowercase-alpha pattern", f)
		}
	})

	t.Run("Scenario_EachFamilyRepresentedByExactlyOneModel", func(t *testing.T) {
		// Given each Claude family tier maps to a single canonical model in the seed
		// When the family count is compared to the total model count
		// Then the number of families equals the number of models
		assert.Equal(t, len(SeedModelCapFamilies), SeedExpectedModelCapRowCount)
	})
}
