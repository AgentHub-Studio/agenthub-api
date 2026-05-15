package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validAuditExportRequest() AuditExportRequest {
	now := time.Now().UTC()
	return AuditExportRequest{
		TenantID:    "t",
		KitSlug:     "gdpr-quarterly-audit",
		Period:      ReportPeriod{Start: now.Add(-90 * 24 * time.Hour), End: now},
		Formats:     []AuditExportFormat{AuditExportFormatCSV, AuditExportFormatJSON},
		RequestedBy: "admin@tenant.com",
	}
}

func TestAuditExport_FormatEnumIsBounded(t *testing.T) {
	for _, f := range AllAuditExportFormats() {
		assert.True(t, IsValidAuditExportFormat(f))
	}
	assert.False(t, IsValidAuditExportFormat(AuditExportFormat("html")))
}

func TestAuditExport_AllFormatsCount(t *testing.T) {
	// 5 formats: csv/json/pdf/xlsx/zip_bundle.
	assert.Equal(t, 5, len(AllAuditExportFormats()))
}

func TestAuditExport_SignatureAlgorithmEnumIsBounded(t *testing.T) {
	for _, a := range AllSignatureAlgorithms() {
		assert.True(t, IsValidSignatureAlgorithm(a))
	}
	assert.False(t, IsValidSignatureAlgorithm(SignatureAlgorithm("md5")))
}

func TestAuditExport_AllSignatureAlgorithmsCount(t *testing.T) {
	assert.Equal(t, 4, len(AllSignatureAlgorithms()))
}

func TestAuditExport_Build_RejectsRequiredFields(t *testing.T) {
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
}

func TestAuditExport_Build_RejectsInvalidFormat(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	req := validAuditExportRequest()
	req.Formats = []AuditExportFormat{AuditExportFormat("html")}
	_, err := exporter.Build(context.Background(), req)
	assert.True(t, errors.Is(err, ErrInvalidAuditExportFormat))
}

func TestAuditExport_Build_AssignsIDAndProducesFilesPerFormat(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	req := validAuditExportRequest()
	bundle, err := exporter.Build(context.Background(), req)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, bundle.ID)
	assert.Len(t, bundle.Files, 2, "one file per requested format")
	for i, f := range bundle.Files {
		assert.Equal(t, req.Formats[i], f.Format)
		assert.NotEmpty(t, f.Path)
		assert.NotEmpty(t, f.Checksum)
	}
}

func TestAuditExport_Build_ChecksumsAreDeterministicBasedOnPath(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	req := validAuditExportRequest()
	b1, _ := exporter.Build(context.Background(), req)
	b2, _ := exporter.Build(context.Background(), req)
	for i := range b1.Files {
		assert.Equal(t, b1.Files[i].Checksum, b2.Files[i].Checksum,
			"same path → same checksum (deterministic)")
	}
}

func TestAuditExport_Build_SignatureUsesSHA256(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
	assert.Equal(t, SignatureAlgorithmSHA256, bundle.Signature.Algorithm)
	assert.NotEmpty(t, bundle.Signature.Signature)
	assert.Equal(t, "test-signer", bundle.Signature.Signer)
	assert.False(t, bundle.Signature.SignedAt.IsZero())
}

func TestAuditExport_FindByID_RoundTrips(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	b1, _ := exporter.Build(context.Background(), validAuditExportRequest())
	got, err := exporter.FindByID(context.Background(), b1.ID)
	require.NoError(t, err)
	assert.Equal(t, b1.ID, got.ID)
}

func TestAuditExport_FindByID_UnknownReturnsNotFound(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	_, err := exporter.FindByID(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrAuditExportBundleNotFound))
}

func TestAuditExport_ListByTenant_NewestFirst(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	for i := 0; i < 3; i++ {
		_, _ = exporter.Build(context.Background(), validAuditExportRequest())
		time.Sleep(2 * time.Millisecond)
	}
	got, err := exporter.ListByTenant(context.Background(), "t", time.Time{}, 0)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.True(t, got[i-1].GeneratedAt.After(got[i].GeneratedAt) ||
			got[i-1].GeneratedAt.Equal(got[i].GeneratedAt))
	}
}

func TestAuditExport_ListByTenant_TenantIsolation(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	_, _ = exporter.Build(context.Background(), validAuditExportRequest())
	got, _ := exporter.ListByTenant(context.Background(), "different-tenant", time.Time{}, 0)
	assert.Empty(t, got)
}

func TestAuditExport_ListByTenant_RespectsLimit(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	for i := 0; i < 5; i++ {
		_, _ = exporter.Build(context.Background(), validAuditExportRequest())
	}
	got, _ := exporter.ListByTenant(context.Background(), "t", time.Time{}, 2)
	assert.Len(t, got, 2)
}

func TestAuditExport_ListByTenant_FiltersBySinceCutoff(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	_, _ = exporter.Build(context.Background(), validAuditExportRequest())
	cutoff := time.Now()
	time.Sleep(10 * time.Millisecond)
	_, _ = exporter.Build(context.Background(), validAuditExportRequest())

	got, _ := exporter.ListByTenant(context.Background(), "t", cutoff, 0)
	assert.Len(t, got, 1, "only post-cutoff bundles returned")
}

func TestAuditExport_VerifySignature_ValidBundlePasses(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
	ok, err := exporter.VerifySignature(context.Background(), bundle)
	require.NoError(t, err)
	assert.True(t, ok, "freshly-built bundle must verify clean")
}

func TestAuditExport_VerifySignature_TamperedFileChecksumFails(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
	bundle.Files[0].Checksum = "tampered"
	ok, err := exporter.VerifySignature(context.Background(), bundle)
	require.NoError(t, err)
	assert.False(t, ok, "tampered checksum must fail verification")
}

func TestAuditExport_VerifySignature_TamperedSignatureFails(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
	bundle.Signature.Signature = "deadbeef"
	ok, err := exporter.VerifySignature(context.Background(), bundle)
	require.NoError(t, err)
	assert.False(t, ok, "tampered signature must fail verification")
}

func TestAuditExport_VerifySignature_RejectsUnsupportedAlgorithm(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
	bundle.Signature.Algorithm = SignatureAlgorithmEd25519 // valid enum, unsupported by stub
	_, err := exporter.VerifySignature(context.Background(), bundle)
	assert.Error(t, err, "stub only supports sha256")
}

func TestAuditExport_VerifySignature_RejectsInvalidAlgorithm(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())
	bundle.Signature.Algorithm = SignatureAlgorithm("md5")
	_, err := exporter.VerifySignature(context.Background(), bundle)
	assert.True(t, errors.Is(err, ErrInvalidSignatureAlgorithm))
}

func TestAuditExport_Build_FilePathsIncludeKitAndDate(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	req := validAuditExportRequest()
	bundle, _ := exporter.Build(context.Background(), req)
	for _, f := range bundle.Files {
		assert.Contains(t, f.Path, req.KitSlug, "path includes kit slug")
		assert.Contains(t, f.Path, req.Period.Start.UTC().Format("2006-01-02"),
			"path includes period start date")
	}
}

func TestAuditExport_ConcurrentBuildIsSafe(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := exporter.Build(context.Background(), validAuditExportRequest())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	got, _ := exporter.ListByTenant(context.Background(), "t", time.Time{}, 0)
	assert.Len(t, got, 50)
}

func TestAuditExport_ContextCancelledOperationsError(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	bundle, _ := exporter.Build(context.Background(), validAuditExportRequest())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := exporter.Build(ctx, validAuditExportRequest())
	assert.Error(t, err)

	_, err = exporter.FindByID(ctx, bundle.ID)
	assert.Error(t, err)

	_, err = exporter.ListByTenant(ctx, "t", time.Time{}, 0)
	assert.Error(t, err)

	_, err = exporter.VerifySignature(ctx, bundle)
	assert.Error(t, err)
}

func TestAuditExport_GeneratedByCarriesAuditIdentity(t *testing.T) {
	exporter := NewStubAuditExporter("test-signer")
	req := validAuditExportRequest()
	req.RequestedBy = "compliance-officer@tenant.com"
	bundle, _ := exporter.Build(context.Background(), req)
	assert.Equal(t, "compliance-officer@tenant.com", bundle.GeneratedBy,
		"audit chain: requester carried into bundle")
}
