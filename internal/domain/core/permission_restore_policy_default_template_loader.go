package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePermissionRestorePolicyDefaultTemplate is a platform-managed
// blueprint paired with PERM-009 FilterRestorableGrants. Each template
// represents one PermissionRestorePolicy variant the tenant can adopt
// for resume/fork operations.
type CorePermissionRestorePolicyDefaultTemplate struct {
	ID                          uuid.UUID
	Slug                        string
	Name                        string
	Description                 string
	TargetRestorePolicy         string
	TargetUseCase               string
	SafetyPosture               string
	SurvivingDurabilitiesJSON   string
	FlagsFirstUseConfirmation   bool
	RecommendedForTenantKind    string
	RequiresAdminReview         bool
	IsRecommended               bool
	IsActive                    bool
	SortOrder                   int
}

// SurvivingDurabilities parses the JSONB array of GrantDurability
// labels that survive this policy.
func (t CorePermissionRestorePolicyDefaultTemplate) SurvivingDurabilities() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SurvivingDurabilitiesJSON), &out); err != nil {
		return nil, fmt.Errorf("permission_restore_policy_template %s: invalid surviving_durabilities: %w", t.Slug, err)
	}
	return out, nil
}

// CorePermissionRestorePolicyDefaultTemplateLoader loads templates.
type CorePermissionRestorePolicyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePermissionRestorePolicyDefaultTemplateLoader creates the loader.
func NewCorePermissionRestorePolicyDefaultTemplateLoader(pool *pgxpool.Pool) *CorePermissionRestorePolicyDefaultTemplateLoader {
	return &CorePermissionRestorePolicyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePermissionRestorePolicyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePermissionRestorePolicyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_restore_policy, target_use_case,
		       safety_posture, surviving_durabilities::text, flags_first_use_confirmation,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.permission_restore_policy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.permission_restore_policy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query permission_restore_policy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePermissionRestorePolicyDefaultTemplate
	for rows.Next() {
		var t CorePermissionRestorePolicyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetRestorePolicy, &t.TargetUseCase,
			&t.SafetyPosture, &t.SurvivingDurabilitiesJSON, &t.FlagsFirstUseConfirmation,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan permission_restore_policy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate permission_restore_policy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePermissionRestorePolicyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePermissionRestorePolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePermissionRestorePolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePermissionRestorePolicyDefaultTemplate{}, false, nil
}

// LoadByPolicy filters by PERM-009 PermissionRestorePolicy.
func (l *CorePermissionRestorePolicyDefaultTemplateLoader) LoadByPolicy(ctx context.Context, policy string) ([]CorePermissionRestorePolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionRestorePolicyDefaultTemplate
	for _, t := range all {
		if t.TargetRestorePolicy == policy {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadBySafetyPosture filters by posture.
func (l *CorePermissionRestorePolicyDefaultTemplateLoader) LoadBySafetyPosture(ctx context.Context, posture string) ([]CorePermissionRestorePolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionRestorePolicyDefaultTemplate
	for _, t := range all {
		if t.SafetyPosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePermissionRestorePolicyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePermissionRestorePolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionRestorePolicyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPRPDTemplateSlugs is the closed canonical set.
var SeedExpectedPRPDTemplateSlugs = []string{
	"discard-all-fresh-context",
	"preserve-durable-routine",
	"preserve-explicit-compliance",
	"strict-re-request-audit",
}

// SeedExpectedPRPDTemplatePolicies matches PERM-009 PermissionRestorePolicy
// enum byte-for-byte (4 policies).
var SeedExpectedPRPDTemplatePolicies = []string{
	"discard_all", "preserve_durable_only",
	"preserve_explicit_grants", "strict_re_request",
}

// SeedExpectedPRPDTemplateUseCases is the closed use-case set.
var SeedExpectedPRPDTemplateUseCases = []string{
	"fresh_resume", "routine_resume",
	"compliance_audit", "audit_strict_resume",
}

// SeedExpectedPRPDTemplateSafetyPostures is the closed posture set.
var SeedExpectedPRPDTemplateSafetyPostures = []string{
	"strict", "balanced", "conservative",
}

// SeedExpectedPRPDTemplateDurabilities matches PERM-009 GrantDurability
// enum byte-for-byte (4 tiers).
var SeedExpectedPRPDTemplateDurabilities = []string{
	"one_shot", "session_scoped", "persisted", "explicit_admin",
}

// SeedExpectedPRPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedPRPDTemplateTenantKinds = []string{"general"}

// SeedRecommendedPRPDTemplateSlugs — all 4 are recommended.
var SeedRecommendedPRPDTemplateSlugs = []string{
	"discard-all-fresh-context",
	"preserve-durable-routine",
	"preserve-explicit-compliance",
	"strict-re-request-audit",
}

// SeedAdminReviewPRPDTemplateSlugs — every preset except routine.
var SeedAdminReviewPRPDTemplateSlugs = []string{
	"discard-all-fresh-context",
	"preserve-explicit-compliance",
	"strict-re-request-audit",
}

// SeedExpectedPRPDTemplateRowCount = 4.
const SeedExpectedPRPDTemplateRowCount = 4
