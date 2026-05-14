package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreEvidenceKitSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsComplianceEvidenceKitCatalog", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(SeedExpectedComplianceEvidenceKitSlugs), 6)
	})

	t.Run("Scenario_AllEightComplianceProfilesCovered", func(t *testing.T) {
		set := map[string]bool{}
		for _, p := range SeedExpectedComplianceEvidenceKitProfiles {
			set[p] = true
		}
		// 6 regulated + 2 generic.
		for _, want := range []string{
			"gdpr", "hipaa", "sox", "pci_dss", "soc2", "iso27001",
			"generic_audit", "generic_security",
		} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_RegulatedKitsRequireAdminSignoffForOngoingObligation", func(t *testing.T) {
		// Given enabling GDPR/HIPAA/SOX/PCI/ISO27001 evidence collection
		//       commits the tenant to compliance reporting,
		set := map[string]bool{}
		for _, s := range SeedAdminSignoffComplianceEvidenceKitSlugs {
			set[s] = true
		}
		for _, regulated := range []string{
			"gdpr-quarterly-audit", "hipaa-monthly-phi-access",
			"sox-quarterly-financial-controls",
			"pci-dss-quarterly-cardholder", "iso27001-annual-isms",
		} {
			assert.True(t, set[regulated],
				"regulated %q must require admin signoff", regulated)
		}
	})

	t.Run("Scenario_GenericKitsAreSafeOneClickRecommended", func(t *testing.T) {
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedComplianceEvidenceKitSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["generic-monthly-audit"], "generic monthly safe baseline")
		assert.True(t, recSet["generic-weekly-security"], "generic weekly security review")
	})

	t.Run("Scenario_SOC2IsRecommendedAsLowFrictionTrustBaseline", func(t *testing.T) {
		// SOC2 doesn't require admin signoff (Type II is auditor-driven,
		// not regulator-imposed) — it's the low-friction recommended kit
		// for any SaaS company.
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedComplianceEvidenceKitSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["soc2-quarterly-trust-criteria"])
	})

	t.Run("Scenario_RecommendedExcludesAllAdminSignoffKits", func(t *testing.T) {
		// One-click constraint.
		adminSet := map[string]bool{}
		for _, a := range SeedAdminSignoffComplianceEvidenceKitSlugs {
			adminSet[a] = true
		}
		for _, r := range SeedRecommendedComplianceEvidenceKitSlugs {
			assert.False(t, adminSet[r],
				"recommended %q must be one-click (no signoff required)", r)
		}
	})

	t.Run("Scenario_AdminSignoffCoversOnlyRegulatedProfiles", func(t *testing.T) {
		for _, s := range SeedAdminSignoffComplianceEvidenceKitSlugs {
			assert.False(t, strings.HasPrefix(s, "generic-"),
				"admin signoff is for regulated profiles only — not generic")
		}
	})

	t.Run("Scenario_ExportFormatsCoverMajorRegulatorPreferences", func(t *testing.T) {
		// Given regulators prefer different formats (csv for grep,
		//       json for tooling, pdf for archive, xlsx for finance),
		set := map[string]bool{}
		for _, f := range SeedExpectedComplianceEvidenceKitFormats {
			set[f] = true
		}
		for _, want := range []string{"csv", "json", "pdf", "xlsx"} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_KitNamesEncodeFrequencyAndProfileForReadability", func(t *testing.T) {
		// Given operators see "gdpr-quarterly-X" / "hipaa-monthly-X",
		validPrefixes := []string{
			"gdpr-", "hipaa-", "sox-", "pci-dss-", "soc2-", "iso27001-", "generic-",
		}
		for _, s := range SeedExpectedComplianceEvidenceKitSlugs {
			matches := false
			for _, p := range validPrefixes {
				if strings.HasPrefix(s, p) {
					matches = true
					break
				}
			}
			assert.True(t, matches,
				"slug %q must encode profile prefix", s)
		}
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 8, len(SeedExpectedComplianceEvidenceKitSlugs))
	})
}
