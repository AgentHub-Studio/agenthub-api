package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePolicyTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsComplianceTemplateCatalog", func(t *testing.T) {
		// Given a fresh tenant needs to enforce GDPR / HIPAA / SOX /
		//       PCI / SOC2 / ISO27001 without writing rules from scratch,
		// When ah_core templates are loaded,
		// Then ≥6 catalog entries appear so the tenant has options.
		assert.GreaterOrEqual(t, len(SeedExpectedPolicyTemplateSlugs), 6)
	})

	t.Run("Scenario_AllSevenComplianceProfilesAreCovered", func(t *testing.T) {
		// Given enterprise customers need at least the 6 major frameworks
		//       (GDPR / HIPAA / SOX / PCI-DSS / SOC2 / ISO27001) plus
		//       generic safety baseline,
		set := map[string]bool{}
		for _, p := range SeedExpectedPolicyTemplateComplianceProfiles {
			set[p] = true
		}
		for _, want := range []string{
			"generic_safety", "gdpr", "hipaa", "sox", "pci_dss", "soc2", "iso27001",
		} {
			assert.True(t, set[want], "profile %q must be in seed", want)
		}
	})

	t.Run("Scenario_GenericSafetyIsTheRecommendedBaseline", func(t *testing.T) {
		// Given every tenant needs at least baseline shell/DDL/force-push
		//       blocking — even non-regulated SaaS,
		// When the recommended subset is inspected,
		// Then generic-safety is recommended.
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedPolicyTemplateSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["generic-safety"],
			"generic-safety = universal recommended baseline")
	})

	t.Run("Scenario_RegulatedProfilesRequireAdminApprovalBeforeEnabling", func(t *testing.T) {
		// Given enabling GDPR/HIPAA/SOX/PCI/ISO27001 commits the tenant
		//       to legal/compliance obligations — accidental enable by
		//       a non-admin would be a governance failure,
		adminSet := map[string]bool{}
		for _, s := range SeedAdminReviewRequiredPolicyTemplateSlugs {
			adminSet[s] = true
		}
		for _, regulated := range []string{
			"gdpr-strict", "hipaa-strict", "sox-financial",
			"pci-dss-cardholder", "iso27001-info-security",
		} {
			assert.True(t, adminSet[regulated],
				"regulated %q must require admin approval", regulated)
		}
	})

	t.Run("Scenario_RecommendedSetExcludesAllAdminReviewTemplates", func(t *testing.T) {
		// Given recommended templates are one-click enable,
		// When they're inspected,
		// Then NONE of them require admin review (one-click constraint).
		adminSet := map[string]bool{}
		for _, a := range SeedAdminReviewRequiredPolicyTemplateSlugs {
			adminSet[a] = true
		}
		for _, r := range SeedRecommendedPolicyTemplateSlugs {
			assert.False(t, adminSet[r],
				"recommended %q must be one-click — cannot also require admin review", r)
		}
	})

	t.Run("Scenario_StaticDenyIsTheDefaultEngineForCompliance", func(t *testing.T) {
		// Given GOV-002 PolicyEngine has 3 implementations + chained,
		//       static_deny is the simplest and most-auditable for
		//       compliance contexts,
		// When the engine_kinds are inspected,
		// Then static_deny is in the allowed set.
		set := map[string]bool{}
		for _, k := range SeedExpectedPolicyTemplateEngineKinds {
			set[k] = true
		}
		assert.True(t, set["static_deny"],
			"static_deny is the default engine for compliance templates")
	})

	t.Run("Scenario_SlugsAreKebabCaseForURLSafety", func(t *testing.T) {
		// Given slugs may appear in URLs / config files,
		for _, s := range SeedExpectedPolicyTemplateSlugs {
			assert.False(t, strings.Contains(s, "_"),
				"slug %q must use kebab-case (no underscores)", s)
		}
	})

	t.Run("Scenario_AuditObligationsAlwaysIncludedForRegulatedTemplates", func(t *testing.T) {
		// Given audit_log obligation is mandatory for compliance
		//       (no compliance without audit trail),
		// When a regulated template is constructed,
		// Then audit_log appears in its obligations list.
		// (Constant guard — integration test verifies actual DB rows.)
		// Just the list shape contract:
		tmpl := CorePolicyEngineTemplate{
			Obligations: "audit_log, evidence_header, gdpr_consent_check",
		}
		obs := tmpl.ObligationsList()
		assert.Contains(t, obs, "audit_log")
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 7, len(SeedExpectedPolicyTemplateSlugs))
	})

	t.Run("Scenario_ParseListHelpersHandleWhitespaceAndEmpty", func(t *testing.T) {
		// Given migrations can have whitespace in comma-separated lists,
		assert.Equal(t, []string{"a", "b"}, parseCommaList(" a , b "))
		assert.Nil(t, parseCommaList(""))
		assert.Equal(t, []string{"x"}, parseCommaList("x,,"))
	})
}
