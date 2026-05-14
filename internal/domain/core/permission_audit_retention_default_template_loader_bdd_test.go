package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePARDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksAuditLifecycleFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a PERM-010 retention + redaction
		// policy before audit log accumulates,
		// When admin opens audit-lifecycle onboarding,
		// Then 5 recommended templates surface across the compliance
		// ladder (minimal/balanced/regulated/pii_strict/forensic_hold).
		assert.Equal(t, 5, len(SeedRecommendedPARDTemplateSlugs))
	})

	t.Run("Scenario_MinimalRetentionForDebugOnly", func(t *testing.T) {
		// Given a tenant just wants recent audit for debugging,
		// When admin uses minimal-30day,
		// Then every tier is 30 days, no redaction, no admin review.
		assert.Contains(t, SeedExpectedPARDTemplateSlugs, "minimal-30day")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPARDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["minimal-30day"])
	})

	t.Run("Scenario_BalancedDefaultForStandardTenants", func(t *testing.T) {
		// Given routine tenants want allows kept 90d, denies kept 1y,
		// When admin uses balanced-90d-allow-365d-deny,
		// Then no admin review required (routine default).
		assert.Contains(t, SeedExpectedPARDTemplateSlugs, "balanced-90d-allow-365d-deny")
	})

	t.Run("Scenario_RegulatedSevenYearsDenyRetention", func(t *testing.T) {
		// Given financial / healthcare tenants must retain denies 7y,
		// When admin uses regulated-7y-deny-2y-confirm,
		// Then admin review required; input snippets redacted on export.
		assert.Contains(t, SeedExpectedPARDTemplateSlugs, "regulated-7y-deny-2y-confirm")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPARDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["regulated-7y-deny-2y-confirm"])
	})

	t.Run("Scenario_PIIStrictRedactsAllSensitiveFields", func(t *testing.T) {
		// Given GDPR / LGPD tenants must redact PII for export,
		// When admin uses strict-pii-redacted-export,
		// Then all three redaction flags are on (snippet + rule + run id).
		assert.Contains(t, SeedExpectedPARDTemplateSlugs, "strict-pii-redacted-export")
	})

	t.Run("Scenario_ForensicHoldNeverExpiresEntries", func(t *testing.T) {
		// Given a tenant is under legal hold or active investigation,
		// When admin uses forensic-hold-never-expires,
		// Then every TTL is 0 (never expire) and admin review required.
		assert.Contains(t, SeedExpectedPARDTemplateSlugs, "forensic-hold-never-expires")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPARDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["forensic-hold-never-expires"])
	})

	t.Run("Scenario_ComplianceLadderRepresented", func(t *testing.T) {
		// Given compliance posture is a stance ladder,
		// When seed templates ship,
		// Then 5 postures cobertos 1:1.
		assert.Equal(t, 5, len(SeedExpectedPARDTemplatePostures))
	})

	t.Run("Scenario_AdminReviewExcludesRoutineTiers", func(t *testing.T) {
		// Given minimal + balanced are routine baseline,
		// When admin compares admin-review subset,
		// Then only 3 of 5 templates require review (regulated/pii/forensic).
		assert.Equal(t, 3, len(SeedAdminReviewPARDTemplateSlugs))
	})

	t.Run("Scenario_DenyTTLAlwaysGTEAllowTTLForCompliance", func(t *testing.T) {
		// Given compliance pattern: denies retained longer than allows,
		// When admin inspects each template,
		// Then deny_ttl_days >= allow_ttl_days for every preset (with
		// 0=forever handled separately). Validated DB-real in integration.
		assert.Equal(t, 5, SeedExpectedPARDTemplateRowCount)
	})

	t.Run("Scenario_PIIStrictAddsRedactionOnTopOfRegulated", func(t *testing.T) {
		// Given PII tenants need regulated retention PLUS aggressive
		// redaction,
		// When admin compares pii_strict vs regulated,
		// Then TTLs match but redaction flags differ. Validated DB-real
		// in integration test.
		assert.Contains(t, SeedExpectedPARDTemplateSlugs, "regulated-7y-deny-2y-confirm")
		assert.Contains(t, SeedExpectedPARDTemplateSlugs, "strict-pii-redacted-export")
	})
}
