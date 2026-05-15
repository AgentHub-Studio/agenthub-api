package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreEvidenceKit_SlugsNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedComplianceEvidenceKitSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreEvidenceKit_SlugsCanonicalCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedComplianceEvidenceKitSlugs))
}

func TestCoreEvidenceKit_ProfilesCount(t *testing.T) {
	// 8 profiles: 6 regulated + 2 generic.
	assert.Equal(t, 8, len(SeedExpectedComplianceEvidenceKitProfiles))
}

func TestCoreEvidenceKit_ProfilesCoverMajorFrameworks(t *testing.T) {
	allowed := map[string]bool{}
	for _, p := range SeedExpectedComplianceEvidenceKitProfiles {
		allowed[p] = true
	}
	for _, want := range []string{
		"gdpr", "hipaa", "sox", "pci_dss", "soc2", "iso27001",
		"generic_audit", "generic_security",
	} {
		assert.True(t, allowed[want])
	}
}

func TestCoreEvidenceKit_ExportFormatsAllowedSet(t *testing.T) {
	allowed := map[string]bool{}
	for _, f := range SeedExpectedComplianceEvidenceKitFormats {
		allowed[f] = true
	}
	for _, want := range []string{"csv", "json", "pdf", "xlsx"} {
		assert.True(t, allowed[want])
	}
}

func TestCoreEvidenceKit_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedComplianceEvidenceKitSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedComplianceEvidenceKitSlugs {
		assert.True(t, seedSet[r])
	}
}

func TestCoreEvidenceKit_AdminSignoffAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedComplianceEvidenceKitSlugs {
		seedSet[s] = true
	}
	for _, a := range SeedAdminSignoffComplianceEvidenceKitSlugs {
		assert.True(t, seedSet[a])
	}
}

func TestCoreEvidenceKit_AdminSignoffCoversRegulatedProfilesOnly(t *testing.T) {
	// Only regulated profiles need admin signoff (compliance contract).
	for _, s := range SeedAdminSignoffComplianceEvidenceKitSlugs {
		isGeneric := strings.HasPrefix(s, "generic-")
		assert.False(t, isGeneric,
			"admin-signoff slug %q must NOT be generic (only regulated needs signoff)", s)
	}
}

func TestCoreEvidenceKit_RecommendedExcludesAdminSignoff(t *testing.T) {
	// Recommended = one-click safe; admin-signoff = explicit opt-in.
	adminSet := map[string]bool{}
	for _, a := range SeedAdminSignoffComplianceEvidenceKitSlugs {
		adminSet[a] = true
	}
	for _, r := range SeedRecommendedComplianceEvidenceKitSlugs {
		assert.False(t, adminSet[r],
			"recommended %q must not require admin signoff (one-click constraint)", r)
	}
}

func TestCoreEvidenceKit_ExportFormatsList_Parses(t *testing.T) {
	k := CoreComplianceEvidenceKit{ExportFormats: "csv,json,pdf"}
	assert.Equal(t, []string{"csv", "json", "pdf"}, k.ExportFormatsList())
}

func TestCoreEvidenceKit_SlugsKebabCase(t *testing.T) {
	for _, s := range SeedExpectedComplianceEvidenceKitSlugs {
		assert.False(t, strings.Contains(s, "_"),
			"slug %q must use kebab-case", s)
	}
}
