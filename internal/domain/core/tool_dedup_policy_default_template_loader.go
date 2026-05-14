package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreToolDedupPolicyDefaultTemplate is a platform-managed blueprint
// paired with TOOL-004 tool dedup / precedence. Each template describes
// one ToolDedupPolicy variant tenants can adopt without inventing
// version-resolution semantics.
type CoreToolDedupPolicyDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetPolicy             string
	TargetUseCase            string
	SafetyPosture            string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreToolDedupPolicyDefaultTemplateLoader loads dedup policy templates.
type CoreToolDedupPolicyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreToolDedupPolicyDefaultTemplateLoader creates the loader.
func NewCoreToolDedupPolicyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreToolDedupPolicyDefaultTemplateLoader {
	return &CoreToolDedupPolicyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreToolDedupPolicyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreToolDedupPolicyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_policy, target_use_case,
		       safety_posture, recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.tool_dedup_policy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.tool_dedup_policy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query tool_dedup_policy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreToolDedupPolicyDefaultTemplate
	for rows.Next() {
		var t CoreToolDedupPolicyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetPolicy, &t.TargetUseCase,
			&t.SafetyPosture, &t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan tool_dedup_policy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate tool_dedup_policy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreToolDedupPolicyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreToolDedupPolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreToolDedupPolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreToolDedupPolicyDefaultTemplate{}, false, nil
}

// LoadByPolicy filters by TOOL-004 ToolDedupPolicy label.
func (l *CoreToolDedupPolicyDefaultTemplateLoader) LoadByPolicy(ctx context.Context, policy string) ([]CoreToolDedupPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolDedupPolicyDefaultTemplate
	for _, t := range all {
		if t.TargetPolicy == policy {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadBySafetyPosture filters by posture label.
func (l *CoreToolDedupPolicyDefaultTemplateLoader) LoadBySafetyPosture(ctx context.Context, posture string) ([]CoreToolDedupPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolDedupPolicyDefaultTemplate
	for _, t := range all {
		if t.SafetyPosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreToolDedupPolicyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreToolDedupPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolDedupPolicyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedTDPDTemplateSlugs is the closed canonical set.
var SeedExpectedTDPDTemplateSlugs = []string{
	"default-source-rank",
	"rolling-latest-version",
	"admin-pinned-conservative",
	"dual-version-migration",
	"fail-fast-no-drift",
}

// SeedExpectedTDPDTemplatePolicies matches TOOL-004 ToolDedupPolicy
// enum byte-for-byte (5 policies).
var SeedExpectedTDPDTemplatePolicies = []string{
	"prefer_source_rank",
	"prefer_latest_version",
	"prefer_pinned",
	"keep_all_versions",
	"deny_collision",
}

// SeedExpectedTDPDTemplateUseCases is the closed use-case set.
var SeedExpectedTDPDTemplateUseCases = []string{
	"general_default",
	"continuous_upgrade",
	"compliance_pin",
	"migration_window",
	"audit_strict",
}

// SeedExpectedTDPDTemplateSafetyPostures is the closed safety set.
var SeedExpectedTDPDTemplateSafetyPostures = []string{
	"balanced", "progressive", "conservative", "permissive", "strict",
}

// SeedExpectedTDPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedTDPDTemplateTenantKinds = []string{"general"}

// SeedRecommendedTDPDTemplateSlugs — all 5 are recommended (each
// implements a documented TOOL-004 policy variant).
var SeedRecommendedTDPDTemplateSlugs = []string{
	"default-source-rank",
	"rolling-latest-version",
	"admin-pinned-conservative",
	"dual-version-migration",
	"fail-fast-no-drift",
}

// SeedAdminReviewTDPDTemplateSlugs gates compliance / strict policies
// because they materially change which versions agents see.
var SeedAdminReviewTDPDTemplateSlugs = []string{
	"admin-pinned-conservative",
	"fail-fast-no-drift",
}

// SeedExpectedTDPDTemplateRowCount = 5.
const SeedExpectedTDPDTemplateRowCount = 5
