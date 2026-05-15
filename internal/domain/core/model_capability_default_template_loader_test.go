package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreModelCap_ModelIDsCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedExpectedModelCapModelIDs))
	assert.Equal(t, 3, SeedExpectedModelCapRowCount)
}

func TestCoreModelCap_ModelIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, id := range SeedExpectedModelCapModelIDs {
		assert.False(t, seen[id], "duplicate model id %q", id)
		seen[id] = true
	}
}

func TestCoreModelCap_FamiliesCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedModelCapFamilies))
}

func TestCoreModelCap_FamiliesMatchRegex(t *testing.T) {
	for _, f := range SeedModelCapFamilies {
		assert.True(t, SeedModelCapFamilyRE.MatchString(f), "family %q must be lowercase alpha", f)
	}
}

func TestCoreModelCap_RowCountEqualsModelIDCount(t *testing.T) {
	assert.Equal(t, SeedExpectedModelCapRowCount, len(SeedExpectedModelCapModelIDs))
}

func TestCoreModelCap_MaxContextWindow(t *testing.T) {
	assert.Equal(t, 200_000, SeedModelCapMaxContextWindow)
}

func TestCoreModelCap_SonnetInExpectedIDs(t *testing.T) {
	found := false
	for _, id := range SeedExpectedModelCapModelIDs {
		if id == "claude-sonnet-4-6" {
			found = true
		}
	}
	assert.True(t, found)
}
