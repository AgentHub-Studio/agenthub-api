package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreComplianceEvidenceKit represents a platform-managed evidence
// collection kit for a specific compliance regulation. Pairs with
// policy_engine_templates and GOV-001/GOV-005 audit infrastructure.
type CoreComplianceEvidenceKit struct {
	ID                              uuid.UUID
	Slug                            string
	DisplayName                     string
	Description                     string
	ComplianceProfile               string // gdpr/hipaa/sox/pci_dss/soc2/iso27001/generic_audit/generic_security
	CollectionPeriodDays            int
	IncludesAuditTrail              bool
	IncludesQualityReports          bool
	IncludesGovernanceDecisions     bool
	IncludesUserConsentLogs         bool
	IncludesDataAccessLogs          bool
	ExportFormats                   string // comma-separated
	RetentionDays                   int
	RequiresAdminSignoff            bool
	TargetWebhookTemplateSlug       string // empty = no auto-delivery
	IsRecommended                   bool
	IsActive                        bool
	SortOrder                       int
}

// ExportFormatsList returns parsed format names.
func (k CoreComplianceEvidenceKit) ExportFormatsList() []string {
	return parseCommaList(k.ExportFormats)
}

// CoreComplianceEvidenceKitLoader loads compliance evidence kits.
type CoreComplianceEvidenceKitLoader struct {
	pool *pgxpool.Pool
}

// NewCoreComplianceEvidenceKitLoader creates loader.
func NewCoreComplianceEvidenceKitLoader(pool *pgxpool.Pool) *CoreComplianceEvidenceKitLoader {
	return &CoreComplianceEvidenceKitLoader{pool: pool}
}

// LoadAll returns all active kits.
func (l *CoreComplianceEvidenceKitLoader) LoadAll(ctx context.Context) ([]CoreComplianceEvidenceKit, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description, compliance_profile,
		       collection_period_days,
		       includes_audit_trail, includes_quality_reports,
		       includes_governance_decisions, includes_user_consent_logs,
		       includes_data_access_logs,
		       export_formats, retention_days, requires_admin_signoff,
		       COALESCE(target_webhook_template_slug, '') AS target_webhook_template_slug,
		       is_recommended, is_active, sort_order
		  FROM ah_core.compliance_evidence_kit
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.compliance_evidence_kit not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query compliance_evidence_kit: %w", err)
	}
	defer rows.Close()

	var kits []CoreComplianceEvidenceKit
	for rows.Next() {
		var k CoreComplianceEvidenceKit
		if err := rows.Scan(
			&k.ID, &k.Slug, &k.DisplayName, &k.Description, &k.ComplianceProfile,
			&k.CollectionPeriodDays,
			&k.IncludesAuditTrail, &k.IncludesQualityReports,
			&k.IncludesGovernanceDecisions, &k.IncludesUserConsentLogs,
			&k.IncludesDataAccessLogs,
			&k.ExportFormats, &k.RetentionDays, &k.RequiresAdminSignoff,
			&k.TargetWebhookTemplateSlug,
			&k.IsRecommended, &k.IsActive, &k.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan compliance_evidence_kit: %w", err)
		}
		kits = append(kits, k)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate compliance_evidence_kit: %w", err)
	}
	return kits, nil
}

// FindBySlug returns one kit by slug.
func (l *CoreComplianceEvidenceKitLoader) FindBySlug(ctx context.Context, slug string) (CoreComplianceEvidenceKit, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreComplianceEvidenceKit{}, false, err
	}
	for _, k := range all {
		if k.Slug == slug {
			return k, true, nil
		}
	}
	return CoreComplianceEvidenceKit{}, false, nil
}

// LoadByComplianceProfile returns kits for a profile.
func (l *CoreComplianceEvidenceKitLoader) LoadByComplianceProfile(ctx context.Context, profile string) ([]CoreComplianceEvidenceKit, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreComplianceEvidenceKit
	for _, k := range all {
		if k.ComplianceProfile == profile {
			matched = append(matched, k)
		}
	}
	return matched, nil
}

// LoadRecommended returns recommended kits.
func (l *CoreComplianceEvidenceKitLoader) LoadRecommended(ctx context.Context) ([]CoreComplianceEvidenceKit, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreComplianceEvidenceKit
	for _, k := range all {
		if k.IsRecommended {
			matched = append(matched, k)
		}
	}
	return matched, nil
}

// SeedExpectedComplianceEvidenceKitSlugs is the canonical list.
var SeedExpectedComplianceEvidenceKitSlugs = []string{
	"gdpr-quarterly-audit",
	"hipaa-monthly-phi-access",
	"sox-quarterly-financial-controls",
	"pci-dss-quarterly-cardholder",
	"soc2-quarterly-trust-criteria",
	"iso27001-annual-isms",
	"generic-monthly-audit",
	"generic-weekly-security",
}

// SeedExpectedComplianceEvidenceKitProfiles is the closed set.
var SeedExpectedComplianceEvidenceKitProfiles = []string{
	"gdpr", "hipaa", "sox", "pci_dss", "soc2", "iso27001",
	"generic_audit", "generic_security",
}

// SeedExpectedComplianceEvidenceKitFormats is the closed set of export formats.
var SeedExpectedComplianceEvidenceKitFormats = []string{
	"csv", "json", "pdf", "xlsx",
}

// SeedRecommendedComplianceEvidenceKitSlugs is the curated subset
// (only safe one-click kits — regulated kits require admin signoff).
var SeedRecommendedComplianceEvidenceKitSlugs = []string{
	"soc2-quarterly-trust-criteria",
	"generic-monthly-audit",
	"generic-weekly-security",
}

// SeedAdminSignoffComplianceEvidenceKitSlugs lists kits requiring
// admin signoff (regulated profiles).
var SeedAdminSignoffComplianceEvidenceKitSlugs = []string{
	"gdpr-quarterly-audit",
	"hipaa-monthly-phi-access",
	"sox-quarterly-financial-controls",
	"pci-dss-quarterly-cardholder",
	"iso27001-annual-isms",
}
