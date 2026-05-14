package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreAuditExportFormatTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedAuditExportFormatTemplateSlugs))
	assert.Equal(t, 8, SeedExpectedAuditExportFormatTemplateRowCount)
}

func TestCoreAuditExportFormatTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedAuditExportFormatTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreAuditExportFormatTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedAuditExportFormatTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreAuditExportFormatTemplate_FormatsMatchFutureFiveEnum(t *testing.T) {
	expected := map[string]bool{
		"csv": true, "json": true, "pdf": true, "xlsx": true, "zip_bundle": true,
	}
	for _, f := range SeedExpectedAuditExportFormats {
		assert.True(t, expected[f], "format %q outside FUTURE-005 enum", f)
	}
	assert.Equal(t, 5, len(SeedExpectedAuditExportFormats))
}

func TestCoreAuditExportFormatTemplate_SignatureAlgorithmsMatchFutureFiveEnum(t *testing.T) {
	expected := map[string]bool{
		"sha256": true, "sha256-rsa": true,
		"sha256-ecdsa": true, "ed25519": true,
	}
	for _, s := range SeedExpectedAuditSignatureAlgorithms {
		assert.True(t, expected[s], "signature %q outside FUTURE-005 enum", s)
	}
	assert.Equal(t, 4, len(SeedExpectedAuditSignatureAlgorithms))
}

func TestCoreAuditExportFormatTemplate_ComplianceProfilesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"generic_audit": true, "gdpr": true, "hipaa": true,
		"sox": true, "pci_dss": true, "soc2": true, "iso27001": true,
	}
	for _, p := range SeedExpectedAuditExportComplianceProfiles {
		assert.True(t, expected[p])
	}
}

func TestCoreAuditExportFormatTemplate_RecommendedAndCanonicalAreEqual(t *testing.T) {
	// Each implements a documented audit pattern; tenants opt in per
	// template, not per category.
	assert.ElementsMatch(t,
		SeedExpectedAuditExportFormatTemplateSlugs,
		SeedRecommendedAuditExportFormatTemplateSlugs,
		"all templates must be recommended (each implements a regulated pattern)")
}

func TestCoreAuditExportFormatTemplate_AdminReviewIsRegulatedSubset(t *testing.T) {
	canonical := map[string]bool{}
	for _, s := range SeedExpectedAuditExportFormatTemplateSlugs {
		canonical[s] = true
	}
	for _, a := range SeedAdminReviewAuditExportFormatTemplateSlugs {
		assert.True(t, canonical[a])
		assert.NotContains(t, a, "generic-",
			"generics must not require admin review")
	}
	// 5 of 6 regulated profiles need admin review (SOC2 self-attests).
	assert.Equal(t, 5, len(SeedAdminReviewAuditExportFormatTemplateSlugs))
}

func TestCoreAuditExportFormatTemplate_RenderFilenameSubstitutesTokens(t *testing.T) {
	tmpl := CoreAuditExportFormatTemplate{
		FilenamePattern: "agenthub-{tenant}-{kit}-{period}.zip",
	}
	got := tmpl.RenderFilename(map[string]string{
		"tenant": "acme",
		"kit":    "soc2",
		"period": "2026-Q2",
	})
	assert.Equal(t, "agenthub-acme-soc2-2026-Q2.zip", got)
}

func TestCoreAuditExportFormatTemplate_RenderFilenameLeavesUnknownTokensIntact(t *testing.T) {
	tmpl := CoreAuditExportFormatTemplate{
		FilenamePattern: "agenthub-{tenant}-{kit}.zip",
	}
	got := tmpl.RenderFilename(map[string]string{"tenant": "acme"})
	// {kit} not provided — stays literal so caller can detect.
	assert.Contains(t, got, "{kit}")
	assert.Contains(t, got, "acme")
}
