package agentic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FUTURE-005 — Regulator-facing audit export.
//
// PDF arXiv:2604.14228v1 §11 (regulator-facing export — auditors need
// machine-parseable + signed bundles).
//
// Builds on:
//   - GOV-001 audit trail (source of evidence)
//   - GOV-005 GovernanceReport (aggregates governance signals)
//   - compliance_evidence_kits seed (kit configuration templates)
//
// AuditExporter takes a request (kit slug + period + tenant) and
// produces a signed export bundle: list of files, checksum per file,
// overall signature, signer identity, generated-at timestamp.
//
// Distinction from GOV-005:
//   - GOV-005 = aggregator that collapses raw signals into a report
//     STRUCTURE.
//   - FUTURE-005 = export ORCHESTRATOR that takes report + raw evidence
//     and packages them as multi-file BUNDLE with signature for delivery.

// AuditExportFormat bounded enum.
type AuditExportFormat string

const (
	AuditExportFormatCSV       AuditExportFormat = "csv"
	AuditExportFormatJSON      AuditExportFormat = "json"
	AuditExportFormatPDF       AuditExportFormat = "pdf"
	AuditExportFormatXLSX      AuditExportFormat = "xlsx"
	AuditExportFormatZipBundle AuditExportFormat = "zip_bundle"
)

var allAuditExportFormats = []AuditExportFormat{
	AuditExportFormatCSV, AuditExportFormatJSON,
	AuditExportFormatPDF, AuditExportFormatXLSX,
	AuditExportFormatZipBundle,
}

// IsValidAuditExportFormat returns true for the bounded set.
func IsValidAuditExportFormat(f AuditExportFormat) bool {
	for _, v := range allAuditExportFormats {
		if f == v {
			return true
		}
	}
	return false
}

// AllAuditExportFormats returns a copy of the bounded set.
func AllAuditExportFormats() []AuditExportFormat {
	out := make([]AuditExportFormat, len(allAuditExportFormats))
	copy(out, allAuditExportFormats)
	return out
}

// SignatureAlgorithm bounded enum.
type SignatureAlgorithm string

const (
	SignatureAlgorithmSHA256        SignatureAlgorithm = "sha256"
	SignatureAlgorithmSHA256RSA     SignatureAlgorithm = "sha256-rsa"
	SignatureAlgorithmSHA256ECDSA   SignatureAlgorithm = "sha256-ecdsa"
	SignatureAlgorithmEd25519       SignatureAlgorithm = "ed25519"
)

var allSignatureAlgorithms = []SignatureAlgorithm{
	SignatureAlgorithmSHA256, SignatureAlgorithmSHA256RSA,
	SignatureAlgorithmSHA256ECDSA, SignatureAlgorithmEd25519,
}

// IsValidSignatureAlgorithm returns true for the bounded set.
func IsValidSignatureAlgorithm(a SignatureAlgorithm) bool {
	for _, v := range allSignatureAlgorithms {
		if a == v {
			return true
		}
	}
	return false
}

// AllSignatureAlgorithms returns a copy.
func AllSignatureAlgorithms() []SignatureAlgorithm {
	out := make([]SignatureAlgorithm, len(allSignatureAlgorithms))
	copy(out, allSignatureAlgorithms)
	return out
}

// AuditExportRequest is the input the exporter consumes.
type AuditExportRequest struct {
	TenantID       string            `json:"tenantId"`
	KitSlug        string            `json:"kitSlug"` // FK to ah_core.compliance_evidence_kit.slug
	Period         ReportPeriod      `json:"period"`
	Formats        []AuditExportFormat `json:"formats"`
	RequestedBy    string            `json:"requestedBy"` // user/admin ID
	IncludeRawData bool              `json:"includeRawData"`
}

// AuditExportFile is one file in the bundle.
type AuditExportFile struct {
	Path     string            `json:"path"`     // relative path inside bundle
	Format   AuditExportFormat `json:"format"`
	Bytes    int64             `json:"bytes"`
	Checksum string            `json:"checksum"` // sha256 hex
}

// AuditExportSignature is the bundle's overall signature.
type AuditExportSignature struct {
	Algorithm   SignatureAlgorithm `json:"algorithm"`
	Signature   string             `json:"signature"`   // hex-encoded
	Signer      string             `json:"signer"`      // identity (e.g. tenant admin email)
	SignedAt    time.Time          `json:"signedAt"`
}

// AuditExportBundle is the result the exporter produces.
type AuditExportBundle struct {
	ID          uuid.UUID            `json:"id"`
	TenantID    string               `json:"tenantId"`
	KitSlug     string               `json:"kitSlug"`
	Period      ReportPeriod         `json:"period"`
	Files       []AuditExportFile    `json:"files"`
	Signature   AuditExportSignature `json:"signature"`
	GeneratedAt time.Time            `json:"generatedAt"`
	GeneratedBy string               `json:"generatedBy"`
}

// Sentinels.
var (
	ErrInvalidAuditExportRequest    = errors.New("audit export: invalid request")
	ErrInvalidAuditExportFormat     = errors.New("audit export: invalid format")
	ErrInvalidSignatureAlgorithm    = errors.New("audit export: invalid signature algorithm")
	ErrAuditExportBundleNotFound    = errors.New("audit export: bundle not found")
)

func validateAuditExportRequest(req AuditExportRequest) error {
	if req.TenantID == "" {
		return fmt.Errorf("%w: tenantId required", ErrInvalidAuditExportRequest)
	}
	if req.KitSlug == "" {
		return fmt.Errorf("%w: kitSlug required", ErrInvalidAuditExportRequest)
	}
	if !req.Period.IsValid() {
		return fmt.Errorf("%w: period invalid", ErrInvalidAuditExportRequest)
	}
	if req.RequestedBy == "" {
		return fmt.Errorf("%w: requestedBy required (audit chain)", ErrInvalidAuditExportRequest)
	}
	if len(req.Formats) == 0 {
		return fmt.Errorf("%w: at least one format required", ErrInvalidAuditExportRequest)
	}
	for _, f := range req.Formats {
		if !IsValidAuditExportFormat(f) {
			return fmt.Errorf("%w: %q", ErrInvalidAuditExportFormat, f)
		}
	}
	return nil
}

// AuditExporter produces signed bundles from evidence kit configurations.
type AuditExporter interface {
	Build(ctx context.Context, req AuditExportRequest) (AuditExportBundle, error)
	FindByID(ctx context.Context, id uuid.UUID) (AuditExportBundle, error)
	ListByTenant(ctx context.Context, tenantID string, since time.Time, limit int) ([]AuditExportBundle, error)
	VerifySignature(ctx context.Context, bundle AuditExportBundle) (bool, error)
}

// --- StubAuditExporter ---

// StubAuditExporter is a default in-memory implementation suitable for
// tests + dev. Production exporter would integrate with real cryptographic
// signing (HSM, KMS, etc.).
type StubAuditExporter struct {
	signerIdentity string
	mu             sync.Mutex
	bundles        map[uuid.UUID]AuditExportBundle
}

// NewStubAuditExporter creates a stub exporter. signerIdentity identifies
// the signing party (e.g. "agenthub-platform-signer").
func NewStubAuditExporter(signerIdentity string) *StubAuditExporter {
	return &StubAuditExporter{
		signerIdentity: signerIdentity,
		bundles:        map[uuid.UUID]AuditExportBundle{},
	}
}

// Build constructs a stub bundle with placeholder files for each
// requested format. Each file has computed sha256 checksum from path.
func (e *StubAuditExporter) Build(ctx context.Context, req AuditExportRequest) (AuditExportBundle, error) {
	if err := ctx.Err(); err != nil {
		return AuditExportBundle{}, err
	}
	if err := validateAuditExportRequest(req); err != nil {
		return AuditExportBundle{}, err
	}

	bundle := AuditExportBundle{
		ID:          uuid.New(),
		TenantID:    req.TenantID,
		KitSlug:     req.KitSlug,
		Period:      req.Period,
		GeneratedAt: time.Now(),
		GeneratedBy: req.RequestedBy,
	}

	// Generate one file per requested format.
	concatChecksum := sha256.New()
	for _, format := range req.Formats {
		path := fmt.Sprintf("%s/%s.%s",
			req.KitSlug,
			req.Period.Start.UTC().Format("2006-01-02"),
			format)
		// Stub bytes: deterministic from path.
		fileBytes := int64(len(path) * 100)
		hash := sha256.Sum256([]byte(path))
		checksum := hex.EncodeToString(hash[:])
		bundle.Files = append(bundle.Files, AuditExportFile{
			Path:     path,
			Format:   format,
			Bytes:    fileBytes,
			Checksum: checksum,
		})
		// Concatenate per-file checksums for bundle signature.
		concatChecksum.Write(hash[:])
	}

	// Bundle signature = sha256(concat per-file checksums).
	sigBytes := concatChecksum.Sum(nil)
	bundle.Signature = AuditExportSignature{
		Algorithm: SignatureAlgorithmSHA256,
		Signature: hex.EncodeToString(sigBytes),
		Signer:    e.signerIdentity,
		SignedAt:  time.Now(),
	}

	e.mu.Lock()
	e.bundles[bundle.ID] = bundle
	e.mu.Unlock()
	return bundle, nil
}

// FindByID returns a bundle by ID.
func (e *StubAuditExporter) FindByID(ctx context.Context, id uuid.UUID) (AuditExportBundle, error) {
	if err := ctx.Err(); err != nil {
		return AuditExportBundle{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	b, ok := e.bundles[id]
	if !ok {
		return AuditExportBundle{}, ErrAuditExportBundleNotFound
	}
	return b, nil
}

// ListByTenant returns bundles generated since cutoff, newest-first, capped.
func (e *StubAuditExporter) ListByTenant(ctx context.Context, tenantID string, since time.Time, limit int) ([]AuditExportBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	matched := []AuditExportBundle{}
	for _, b := range e.bundles {
		if b.TenantID != tenantID {
			continue
		}
		if !since.IsZero() && b.GeneratedAt.Before(since) {
			continue
		}
		matched = append(matched, b)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].GeneratedAt.After(matched[j].GeneratedAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// VerifySignature recomputes the bundle signature and compares.
// Returns false if signature was tampered with.
func (e *StubAuditExporter) VerifySignature(ctx context.Context, bundle AuditExportBundle) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !IsValidSignatureAlgorithm(bundle.Signature.Algorithm) {
		return false, fmt.Errorf("%w: %q", ErrInvalidSignatureAlgorithm, bundle.Signature.Algorithm)
	}
	if bundle.Signature.Algorithm != SignatureAlgorithmSHA256 {
		// Stub only verifies sha256; real impl would dispatch to crypto.
		return false, fmt.Errorf("audit export: stub verifier supports only sha256 (got %q)", bundle.Signature.Algorithm)
	}
	concatChecksum := sha256.New()
	for _, f := range bundle.Files {
		// Reconstruct per-file checksum from path (stub: derived from path).
		hash := sha256.Sum256([]byte(f.Path))
		expected := hex.EncodeToString(hash[:])
		if f.Checksum != expected {
			return false, nil
		}
		concatChecksum.Write(hash[:])
	}
	expectedSig := hex.EncodeToString(concatChecksum.Sum(nil))
	return bundle.Signature.Signature == expectedSig, nil
}
