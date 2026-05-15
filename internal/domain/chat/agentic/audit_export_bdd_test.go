package agentic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FUTURE-005 — Regulator-facing audit export BDD.
//
// PDF arXiv:2604.14228v1 §11 (regulator-facing export — auditors need
// machine-parseable + signed bundles).

func TestBDD_AuditExport(t *testing.T) {

	t.Run("Scenario_ComplianceOfficerExportsQuarterlyGDPRBundle", func(t *testing.T) {
		// Given a compliance officer triggers GDPR quarterly export,
		exporter := NewStubAuditExporter("agenthub-platform-signer")
		now := time.Now().UTC()
		bundle, err := exporter.Build(context.Background(), AuditExportRequest{
			TenantID: "acme-eu", KitSlug: "gdpr-quarterly-audit",
			Period: ReportPeriod{
				Start: now.Add(-90 * 24 * time.Hour), End: now,
			},
			Formats: []AuditExportFormat{
				AuditExportFormatCSV, AuditExportFormatJSON, AuditExportFormatPDF,
			},
			RequestedBy: "compliance@acme-eu.com",
		})
		require.NoError(t, err)
		// 3 formats → 3 files in bundle.
		assert.Len(t, bundle.Files, 3)
		assert.Equal(t, "compliance@acme-eu.com", bundle.GeneratedBy,
			"audit chain preserved (who requested)")
		assert.NotEmpty(t, bundle.Signature.Signature)
	})

	t.Run("Scenario_TamperedBundleFailsVerification", func(t *testing.T) {
		// Given a malicious actor tampers with a file's checksum,
		exporter := NewStubAuditExporter("test-signer")
		bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
		bundle.Files[0].Checksum = "0000tampered0000"

		ok, err := exporter.VerifySignature(context.Background(), bundle)
		require.NoError(t, err)
		assert.False(t, ok, "tampered checksum must fail verification")
	})

	t.Run("Scenario_TamperedSignatureFailsVerification", func(t *testing.T) {
		// Given a malicious actor tampers with the signature directly,
		exporter := NewStubAuditExporter("test-signer")
		bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
		bundle.Signature.Signature = "deadbeefdeadbeef"

		ok, err := exporter.VerifySignature(context.Background(), bundle)
		require.NoError(t, err)
		assert.False(t, ok, "tampered signature must fail verification")
	})

	t.Run("Scenario_FreshBundleVerifiesClean", func(t *testing.T) {
		// Given a freshly-built bundle (no tampering),
		exporter := NewStubAuditExporter("test-signer")
		bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
		ok, err := exporter.VerifySignature(context.Background(), bundle)
		require.NoError(t, err)
		assert.True(t, ok, "fresh bundle verifies clean — round-trip integrity")
	})

	t.Run("Scenario_RequiredFieldsEnforceAuditCompleteness", func(t *testing.T) {
		// Given the audit chain requires WHO + WHAT + WHEN + WHERE,
		// When required fields are missing, build rejects.
		exporter := NewStubAuditExporter("test-signer")
		for _, missing := range []string{"tenant", "kit", "period", "requestedBy", "formats"} {
			req := validAuditExportRequest()
			switch missing {
			case "tenant":
				req.TenantID = ""
			case "kit":
				req.KitSlug = ""
			case "period":
				req.Period = ReportPeriod{}
			case "requestedBy":
				req.RequestedBy = ""
			case "formats":
				req.Formats = nil
			}
			_, err := exporter.Build(context.Background(), req)
			assert.Error(t, err, "missing %s must error", missing)
		}
	})

	t.Run("Scenario_FiveFormatsCoverRegulatorPreferences", func(t *testing.T) {
		// Given regulators prefer different formats,
		expected := map[string]bool{
			"csv": true, "json": true, "pdf": true,
			"xlsx": true, "zip_bundle": true,
		}
		for _, f := range AllAuditExportFormats() {
			assert.True(t, expected[string(f)])
		}
		assert.Equal(t, 5, len(AllAuditExportFormats()))
	})

	t.Run("Scenario_FourSignatureAlgorithmsCoverCommonCryptoChoices", func(t *testing.T) {
		expected := map[string]bool{
			"sha256": true, "sha256-rsa": true,
			"sha256-ecdsa": true, "ed25519": true,
		}
		for _, a := range AllSignatureAlgorithms() {
			assert.True(t, expected[string(a)])
		}
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantList", func(t *testing.T) {
		exporter := NewStubAuditExporter("test-signer")
		_, _ = exporter.Build(context.Background(), validAuditExportRequest())
		got, _ := exporter.ListByTenant(context.Background(), "different", time.Time{}, 0)
		assert.Empty(t, got, "cross-tenant leak forbidden")
	})

	t.Run("Scenario_FilePathsEncodeKitSlugAndDateForArchival", func(t *testing.T) {
		// Given regulators archive bundles by kit + date,
		exporter := NewStubAuditExporter("test-signer")
		req := validAuditExportRequest()
		bundle, _ := exporter.Build(context.Background(), req)
		for _, f := range bundle.Files {
			assert.Contains(t, f.Path, "gdpr-quarterly-audit",
				"kit slug in path for archival grouping")
		}
	})

	t.Run("Scenario_DeterministicChecksumsEnableIdempotentRebuild", func(t *testing.T) {
		// Given the same period + kit + formats produce same checksums,
		exporter := NewStubAuditExporter("test-signer")
		req := validAuditExportRequest()
		b1, _ := exporter.Build(context.Background(), req)
		b2, _ := exporter.Build(context.Background(), req)
		for i := range b1.Files {
			assert.Equal(t, b1.Files[i].Checksum, b2.Files[i].Checksum,
				"deterministic — rebuilding doesn't change content checksums")
		}
	})

	t.Run("Scenario_SignerIdentityIsRecordedForAuditChain", func(t *testing.T) {
		// Given regulators verify signer identity,
		exporter := NewStubAuditExporter("agenthub-prod-signer-v1")
		bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
		assert.Equal(t, "agenthub-prod-signer-v1", bundle.Signature.Signer,
			"signer identity preserved in bundle")
	})

	t.Run("Scenario_ListByTenantNewestFirstForRecencyDashboard", func(t *testing.T) {
		exporter := NewStubAuditExporter("test-signer")
		for i := 0; i < 3; i++ {
			_, _ = exporter.Build(context.Background(), validAuditExportRequest())
			time.Sleep(2 * time.Millisecond)
		}
		got, _ := exporter.ListByTenant(context.Background(), "t", time.Time{}, 0)
		require.Len(t, got, 3)
		for i := 1; i < len(got); i++ {
			assert.True(t,
				got[i-1].GeneratedAt.After(got[i].GeneratedAt) ||
					got[i-1].GeneratedAt.Equal(got[i].GeneratedAt))
		}
	})

	t.Run("Scenario_VerifyRejectsUnsupportedAlgorithmExplicitly", func(t *testing.T) {
		// Given the stub only supports sha256 (production would dispatch),
		exporter := NewStubAuditExporter("test-signer")
		bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
		bundle.Signature.Algorithm = SignatureAlgorithmEd25519
		_, err := exporter.VerifySignature(context.Background(), bundle)
		assert.Error(t, err,
			"unsupported algorithm errors explicitly — never silently passes")
	})

	t.Run("Scenario_BoundedFormatEnumCatchesTyposAtBuildTime", func(t *testing.T) {
		// Given a typo in format string (e.g. "html" instead of "json"),
		exporter := NewStubAuditExporter("test-signer")
		req := validAuditExportRequest()
		req.Formats = []AuditExportFormat{AuditExportFormat("html")}
		_, err := exporter.Build(context.Background(), req)
		assert.Error(t, err, "typo'd format caught at Build, not at delivery")
	})
}
