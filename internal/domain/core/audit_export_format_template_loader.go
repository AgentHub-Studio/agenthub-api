package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreAuditExportFormatTemplate is a platform-managed blueprint paired
// with FUTURE-005 AuditExporter. Each template ties an export format +
// signature algorithm + compliance profile + filename pattern into one
// ready-to-instantiate configuration tenants can opt into.
type CoreAuditExportFormatTemplate struct {
	ID                    uuid.UUID
	Slug                  string
	Name                  string
	Description           string
	ExportFormat          string
	SignatureAlgorithm    string
	ComplianceProfile     string
	FilenamePattern       string
	RetentionDays         int
	IncludesRawEvidence   bool
	RequiresAdminReview   bool
	IsRecommended         bool
	IsActive              bool
	SortOrder             int
}

// RenderFilename substitutes simple {tenant}/{kit}/{period} tokens.
// Missing tokens leave the literal placeholder so callers can detect
// incomplete substitution.
func (t CoreAuditExportFormatTemplate) RenderFilename(values map[string]string) string {
	out := t.FilenamePattern
	for k, v := range values {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// CoreAuditExportFormatTemplateLoader loads audit export format templates.
type CoreAuditExportFormatTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreAuditExportFormatTemplateLoader creates the loader.
func NewCoreAuditExportFormatTemplateLoader(pool *pgxpool.Pool) *CoreAuditExportFormatTemplateLoader {
	return &CoreAuditExportFormatTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreAuditExportFormatTemplateLoader) LoadAll(ctx context.Context) ([]CoreAuditExportFormatTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, export_format, signature_algorithm,
		       compliance_profile, filename_pattern, retention_days,
		       includes_raw_evidence, requires_admin_review, is_recommended,
		       is_active, sort_order
		  FROM ah_core.audit_export_format_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.audit_export_format_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query audit_export_format_template: %w", err)
	}
	defer rows.Close()

	var out []CoreAuditExportFormatTemplate
	for rows.Next() {
		var t CoreAuditExportFormatTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.ExportFormat, &t.SignatureAlgorithm,
			&t.ComplianceProfile, &t.FilenamePattern, &t.RetentionDays,
			&t.IncludesRawEvidence, &t.RequiresAdminReview, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan audit_export_format_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate audit_export_format_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreAuditExportFormatTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreAuditExportFormatTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreAuditExportFormatTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreAuditExportFormatTemplate{}, false, nil
}

// LoadByComplianceProfile filters by compliance frame.
func (l *CoreAuditExportFormatTemplateLoader) LoadByComplianceProfile(ctx context.Context, profile string) ([]CoreAuditExportFormatTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreAuditExportFormatTemplate
	for _, t := range all {
		if t.ComplianceProfile == profile {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByExportFormat filters by export format.
func (l *CoreAuditExportFormatTemplateLoader) LoadByExportFormat(ctx context.Context, format string) ([]CoreAuditExportFormatTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreAuditExportFormatTemplate
	for _, t := range all {
		if t.ExportFormat == format {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreAuditExportFormatTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreAuditExportFormatTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreAuditExportFormatTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedAuditExportFormatTemplateSlugs is the closed canonical set.
var SeedExpectedAuditExportFormatTemplateSlugs = []string{
	"generic-monthly-summary",
	"generic-quarterly-bundle",
	"gdpr-data-subject-export",
	"hipaa-phi-access-bundle",
	"sox-financial-controls-bundle",
	"pci-cardholder-data-export",
	"soc2-trust-service-criteria",
	"iso27001-isms-controls-csv",
}

// SeedExpectedAuditExportFormats matches FUTURE-005 AuditExportFormat
// enum byte-for-byte (5 values).
var SeedExpectedAuditExportFormats = []string{
	"csv", "json", "pdf", "xlsx", "zip_bundle",
}

// SeedExpectedAuditSignatureAlgorithms matches FUTURE-005
// SignatureAlgorithm enum byte-for-byte (4 values).
var SeedExpectedAuditSignatureAlgorithms = []string{
	"sha256", "sha256-rsa", "sha256-ecdsa", "ed25519",
}

// SeedExpectedAuditExportComplianceProfiles is the closed set of
// regulatory profiles referenced (mirrors policy_engine_templates).
var SeedExpectedAuditExportComplianceProfiles = []string{
	"generic_audit", "gdpr", "hipaa", "sox", "pci_dss", "soc2", "iso27001",
}

// SeedRecommendedAuditExportFormatTemplateSlugs lists safe one-click
// defaults — all 8 because each implements a documented audit pattern.
var SeedRecommendedAuditExportFormatTemplateSlugs = []string{
	"generic-monthly-summary",
	"generic-quarterly-bundle",
	"gdpr-data-subject-export",
	"hipaa-phi-access-bundle",
	"sox-financial-controls-bundle",
	"pci-cardholder-data-export",
	"soc2-trust-service-criteria",
	"iso27001-isms-controls-csv",
}

// SeedAdminReviewAuditExportFormatTemplateSlugs is the regulated subset
// (every regulated profile requires admin sign-off; only generics don't).
var SeedAdminReviewAuditExportFormatTemplateSlugs = []string{
	"gdpr-data-subject-export",
	"hipaa-phi-access-bundle",
	"sox-financial-controls-bundle",
	"pci-cardholder-data-export",
	"iso27001-isms-controls-csv",
}

// SeedExpectedAuditExportFormatTemplateRowCount = 8.
const SeedExpectedAuditExportFormatTemplateRowCount = 8
