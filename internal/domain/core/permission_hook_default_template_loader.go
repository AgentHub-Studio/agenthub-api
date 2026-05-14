package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePermissionHookDefaultTemplate is a platform-managed blueprint
// paired with PERM-005 permission hooks. Each template describes a
// hook pattern (phase + outcome + use case) tenants can implement.
type CorePermissionHookDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetPhase              string
	TargetOutcome            string
	TargetUseCase            string
	DefaultPriority          int
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CorePermissionHookDefaultTemplateLoader loads hook templates.
type CorePermissionHookDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePermissionHookDefaultTemplateLoader creates the loader.
func NewCorePermissionHookDefaultTemplateLoader(pool *pgxpool.Pool) *CorePermissionHookDefaultTemplateLoader {
	return &CorePermissionHookDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePermissionHookDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePermissionHookDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_phase, target_outcome,
		       target_use_case, default_priority, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.permission_hook_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.permission_hook_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query permission_hook_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePermissionHookDefaultTemplate
	for rows.Next() {
		var t CorePermissionHookDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetPhase, &t.TargetOutcome,
			&t.TargetUseCase, &t.DefaultPriority, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan permission_hook_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate permission_hook_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePermissionHookDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePermissionHookDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePermissionHookDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePermissionHookDefaultTemplate{}, false, nil
}

// LoadByPhase filters by PERM-005 PermissionHookPhase label.
func (l *CorePermissionHookDefaultTemplateLoader) LoadByPhase(ctx context.Context, phase string) ([]CorePermissionHookDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionHookDefaultTemplate
	for _, t := range all {
		if t.TargetPhase == phase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByOutcome filters by PERM-005 PermissionHookOutcome label.
func (l *CorePermissionHookDefaultTemplateLoader) LoadByOutcome(ctx context.Context, outcome string) ([]CorePermissionHookDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionHookDefaultTemplate
	for _, t := range all {
		if t.TargetOutcome == outcome {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case label.
func (l *CorePermissionHookDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CorePermissionHookDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionHookDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePermissionHookDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePermissionHookDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionHookDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPHDTemplateSlugs is the closed canonical set.
var SeedExpectedPHDTemplateSlugs = []string{
	"oncall-bypass-allow",
	"business-hours-gate-deny",
	"pii-input-escalate-confirm",
	"audit-trace-observer-continue",
	"rate-limit-cooldown-deny",
	"sensitive-customer-deny",
}

// SeedExpectedPHDTemplatePhases matches PERM-005 PermissionHookPhase
// enum byte-for-byte (2 phases).
var SeedExpectedPHDTemplatePhases = []string{
	"before_evaluate", "after_evaluate",
}

// SeedExpectedPHDTemplateOutcomes matches PERM-005 PermissionHookOutcome
// enum byte-for-byte (4 outcomes).
var SeedExpectedPHDTemplateOutcomes = []string{
	"continue", "override_allow", "override_deny", "override_confirm",
}

// SeedExpectedPHDTemplateUseCases is the closed use-case set.
var SeedExpectedPHDTemplateUseCases = []string{
	"oncall_escalation", "time_window_gate", "data_sensitivity",
	"audit_observability", "rate_limit", "customer_protection",
}

// SeedExpectedPHDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedPHDTemplateTenantKinds = []string{"general"}

// SeedRecommendedPHDTemplateSlugs — all 6 are recommended.
var SeedRecommendedPHDTemplateSlugs = []string{
	"oncall-bypass-allow",
	"business-hours-gate-deny",
	"pii-input-escalate-confirm",
	"audit-trace-observer-continue",
	"rate-limit-cooldown-deny",
	"sensitive-customer-deny",
}

// SeedAdminReviewPHDTemplateSlugs gates 5 of 6 (everything except the
// pure observer audit-trace, which has no decision impact).
var SeedAdminReviewPHDTemplateSlugs = []string{
	"oncall-bypass-allow",
	"business-hours-gate-deny",
	"pii-input-escalate-confirm",
	"rate-limit-cooldown-deny",
	"sensitive-customer-deny",
}

// SeedExpectedPHDTemplateRowCount = 6.
const SeedExpectedPHDTemplateRowCount = 6
