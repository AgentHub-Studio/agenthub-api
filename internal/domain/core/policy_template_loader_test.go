package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePolicyTemplate_SeedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPolicyTemplateSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCorePolicyTemplate_SeedSlugs_AllNonEmpty(t *testing.T) {
	for i, s := range SeedExpectedPolicyTemplateSlugs {
		assert.NotEmpty(t, s, "slug at %d non-empty", i)
	}
}

func TestCorePolicyTemplate_SeedSlugs_CanonicalCount(t *testing.T) {
	// 7 templates: generic + GDPR + HIPAA + SOX + PCI + SOC2 + ISO27001.
	assert.Equal(t, 7, len(SeedExpectedPolicyTemplateSlugs))
}

func TestCorePolicyTemplate_ComplianceProfiles_AreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range SeedExpectedPolicyTemplateComplianceProfiles {
		assert.False(t, seen[p], "duplicate profile %q", p)
		seen[p] = true
	}
}

func TestCorePolicyTemplate_ComplianceProfiles_CoverMajorFrameworks(t *testing.T) {
	allowed := map[string]bool{}
	for _, p := range SeedExpectedPolicyTemplateComplianceProfiles {
		allowed[p] = true
	}
	for _, want := range []string{"gdpr", "hipaa", "sox", "pci_dss", "soc2", "iso27001", "generic_safety"} {
		assert.True(t, allowed[want], "compliance profile %q must be in seed", want)
	}
}

func TestCorePolicyTemplate_EngineKinds_AreClosed(t *testing.T) {
	allowed := map[string]bool{}
	for _, k := range SeedExpectedPolicyTemplateEngineKinds {
		allowed[k] = true
	}
	for _, want := range []string{"static_deny", "limits_backed", "chained"} {
		assert.True(t, allowed[want])
	}
	assert.Equal(t, 3, len(SeedExpectedPolicyTemplateEngineKinds))
}

func TestCorePolicyTemplate_RecommendedAreSafeDefaults(t *testing.T) {
	// Only safe (no admin-review) templates can be recommended for
	// fresh tenants — regulated profiles need explicit opt-in.
	adminSet := map[string]bool{}
	for _, s := range SeedAdminReviewRequiredPolicyTemplateSlugs {
		adminSet[s] = true
	}
	for _, r := range SeedRecommendedPolicyTemplateSlugs {
		assert.False(t, adminSet[r],
			"recommended %q must NOT also require admin review (safe defaults only)", r)
	}
}

func TestCorePolicyTemplate_AdminReviewSlugs_AllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedPolicyTemplateSlugs {
		seedSet[s] = true
	}
	for _, a := range SeedAdminReviewRequiredPolicyTemplateSlugs {
		assert.True(t, seedSet[a], "admin-review %q must be in seed", a)
	}
}

func TestCorePolicyTemplate_AdminReviewCoversAllRegulatoryProfiles(t *testing.T) {
	// Regulated profiles (GDPR/HIPAA/SOX/PCI/ISO27001) MUST require admin review.
	expected := []string{
		"gdpr-strict", "hipaa-strict", "sox-financial",
		"pci-dss-cardholder", "iso27001-info-security",
	}
	got := map[string]bool{}
	for _, s := range SeedAdminReviewRequiredPolicyTemplateSlugs {
		got[s] = true
	}
	for _, e := range expected {
		assert.True(t, got[e], "regulated %q must require admin review", e)
	}
}

func TestCorePolicyTemplate_GenericSafetyIsRecommendedDefault(t *testing.T) {
	// Generic safety is the catch-all baseline — must always be recommended.
	recSet := map[string]bool{}
	for _, r := range SeedRecommendedPolicyTemplateSlugs {
		recSet[r] = true
	}
	assert.True(t, recSet["generic-safety"],
		"generic-safety must be recommended (universal baseline)")
}

func TestCorePolicyTemplate_SlugsFilesystemSafe(t *testing.T) {
	for _, s := range SeedExpectedPolicyTemplateSlugs {
		for _, r := range s {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok, "slug %q invalid char %q", s, r)
		}
		assert.False(t, strings.Contains(s, "_"),
			"slug %q must use kebab-case", s)
	}
}

func TestCorePolicyTemplate_ParseCommaList_HandlesWhitespace(t *testing.T) {
	got := parseCommaList(" a , b ,c , ,d ")
	assert.Equal(t, []string{"a", "b", "c", "d"}, got,
		"trims, drops empty entries")
}

func TestCorePolicyTemplate_ParseCommaList_HandlesEmpty(t *testing.T) {
	assert.Nil(t, parseCommaList(""))
}

func TestCorePolicyTemplate_DenyToolsList_Parses(t *testing.T) {
	tmpl := CorePolicyEngineTemplate{DenyTools: "shell, execute-sql,git-force-push"}
	got := tmpl.DenyToolsList()
	assert.Equal(t, []string{"shell", "execute-sql", "git-force-push"}, got)
}
