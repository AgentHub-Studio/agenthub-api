package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePermissionAuditRetentionDefaultTemplate is a platform-managed
// blueprint paired with PERM-010 PermissionAuditRetentionPolicy +
// PermissionAuditRedactionPolicy. Each template captures TTLs per
// decision tier (in days) AND redaction flags for compliance export.
type CorePermissionAuditRetentionDefaultTemplate struct {
	ID                          uuid.UUID
	Slug                        string
	Name                        string
	Description                 string
	CompliancePosture           string
	TargetUseCase               string
	AllowTTLDays                int
	DenyTTLDays                 int
	ConfirmApprovedTTLDays      int
	ConfirmDeniedTTLDays        int
	ConfirmEscalatedTTLDays     int
	RedactInputSnippet          bool
	RedactMatchedRule           bool
	RedactRunID                 bool
	RecommendedForTenantKind    string
	RequiresAdminReview         bool
	IsRecommended               bool
	IsActive                    bool
	SortOrder                   int
}

// CorePermissionAuditRetentionDefaultTemplateLoader loads templates.
type CorePermissionAuditRetentionDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePermissionAuditRetentionDefaultTemplateLoader creates the loader.
func NewCorePermissionAuditRetentionDefaultTemplateLoader(pool *pgxpool.Pool) *CorePermissionAuditRetentionDefaultTemplateLoader {
	return &CorePermissionAuditRetentionDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePermissionAuditRetentionDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePermissionAuditRetentionDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, compliance_posture, target_use_case,
		       allow_ttl_days, deny_ttl_days, confirm_approved_ttl_days,
		       confirm_denied_ttl_days, confirm_escalated_ttl_days,
		       redact_input_snippet, redact_matched_rule, redact_run_id,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.permission_audit_retention_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.permission_audit_retention_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query permission_audit_retention_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePermissionAuditRetentionDefaultTemplate
	for rows.Next() {
		var t CorePermissionAuditRetentionDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.CompliancePosture, &t.TargetUseCase,
			&t.AllowTTLDays, &t.DenyTTLDays, &t.ConfirmApprovedTTLDays,
			&t.ConfirmDeniedTTLDays, &t.ConfirmEscalatedTTLDays,
			&t.RedactInputSnippet, &t.RedactMatchedRule, &t.RedactRunID,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan permission_audit_retention_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate permission_audit_retention_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePermissionAuditRetentionDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePermissionAuditRetentionDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePermissionAuditRetentionDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePermissionAuditRetentionDefaultTemplate{}, false, nil
}

// LoadByPosture filters by compliance_posture.
func (l *CorePermissionAuditRetentionDefaultTemplateLoader) LoadByPosture(ctx context.Context, posture string) ([]CorePermissionAuditRetentionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionAuditRetentionDefaultTemplate
	for _, t := range all {
		if t.CompliancePosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case.
func (l *CorePermissionAuditRetentionDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CorePermissionAuditRetentionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionAuditRetentionDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePermissionAuditRetentionDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePermissionAuditRetentionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionAuditRetentionDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPARDTemplateSlugs is the closed canonical set.
var SeedExpectedPARDTemplateSlugs = []string{
	"minimal-30day",
	"balanced-90d-allow-365d-deny",
	"regulated-7y-deny-2y-confirm",
	"strict-pii-redacted-export",
	"forensic-hold-never-expires",
}

// SeedExpectedPARDTemplatePostures is the closed compliance posture set.
var SeedExpectedPARDTemplatePostures = []string{
	"minimal", "balanced", "regulated", "pii_strict", "forensic_hold",
}

// SeedExpectedPARDTemplateUseCases is the closed use-case set.
var SeedExpectedPARDTemplateUseCases = []string{
	"debug_only", "standard_audit",
	"compliance_audit", "gdpr_lgpd", "legal_hold",
}

// SeedExpectedPARDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedPARDTemplateTenantKinds = []string{"general"}

// SeedRecommendedPARDTemplateSlugs — all 5 are recommended.
var SeedRecommendedPARDTemplateSlugs = []string{
	"minimal-30day",
	"balanced-90d-allow-365d-deny",
	"regulated-7y-deny-2y-confirm",
	"strict-pii-redacted-export",
	"forensic-hold-never-expires",
}

// SeedAdminReviewPARDTemplateSlugs — regulated/pii/forensic require
// admin review (compliance impact).
var SeedAdminReviewPARDTemplateSlugs = []string{
	"regulated-7y-deny-2y-confirm",
	"strict-pii-redacted-export",
	"forensic-hold-never-expires",
}

// SeedExpectedPARDTemplateRowCount = 5.
const SeedExpectedPARDTemplateRowCount = 5
