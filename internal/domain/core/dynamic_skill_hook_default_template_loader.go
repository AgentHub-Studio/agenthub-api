package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreDynamicSkillHookDefaultTemplate is a platform-managed blueprint
// paired with EXT-007 DynamicSkillHookRegistry. Each row provides a
// hook pattern fresh extension authors copy when registering hooks for
// their skills.
type CoreDynamicSkillHookDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetPhase              string
	HandlerPattern           string
	DefaultPriority          int
	TargetUseCase            string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreDynamicSkillHookDefaultTemplateLoader loads dynamic skill hook templates.
type CoreDynamicSkillHookDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreDynamicSkillHookDefaultTemplateLoader creates the loader.
func NewCoreDynamicSkillHookDefaultTemplateLoader(pool *pgxpool.Pool) *CoreDynamicSkillHookDefaultTemplateLoader {
	return &CoreDynamicSkillHookDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreDynamicSkillHookDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreDynamicSkillHookDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_phase, handler_pattern,
		       default_priority, target_use_case, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.dynamic_skill_hook_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.dynamic_skill_hook_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query dynamic_skill_hook_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreDynamicSkillHookDefaultTemplate
	for rows.Next() {
		var t CoreDynamicSkillHookDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetPhase, &t.HandlerPattern,
			&t.DefaultPriority, &t.TargetUseCase, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan dynamic_skill_hook_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate dynamic_skill_hook_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreDynamicSkillHookDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreDynamicSkillHookDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreDynamicSkillHookDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreDynamicSkillHookDefaultTemplate{}, false, nil
}

// LoadByPhase filters by EXT-007 phase label.
func (l *CoreDynamicSkillHookDefaultTemplateLoader) LoadByPhase(ctx context.Context, phase string) ([]CoreDynamicSkillHookDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreDynamicSkillHookDefaultTemplate
	for _, t := range all {
		if t.TargetPhase == phase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreDynamicSkillHookDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreDynamicSkillHookDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreDynamicSkillHookDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedDSHDTemplateSlugs is the closed canonical set.
var SeedExpectedDSHDTemplateSlugs = []string{
	"validate-input",
	"redact-pii",
	"audit-tool-call",
	"cost-track",
	"sanitize-output",
	"error-recovery",
}

// SeedExpectedDSHDTemplatePhases matches EXT-007 DynamicSkillHookPhase enum.
var SeedExpectedDSHDTemplatePhases = []string{
	"before_invocation", "before_tool_call", "after_tool_call",
	"after_invocation", "on_error",
}

// SeedExpectedDSHDTemplateUseCases is the closed use-case set.
var SeedExpectedDSHDTemplateUseCases = []string{
	"input_validation", "privacy_compliance", "audit_trail",
	"cost_analytics", "output_sanitization", "error_recovery",
}

// SeedExpectedDSHDTemplateTenantKinds is the closed audience set.
var SeedExpectedDSHDTemplateTenantKinds = []string{
	"general", "regulated",
}

// SeedRecommendedDSHDTemplateSlugs lists the safe one-click subset.
// All 6 are recommended (proven patterns from PDF §6.4).
var SeedRecommendedDSHDTemplateSlugs = []string{
	"validate-input",
	"redact-pii",
	"audit-tool-call",
	"cost-track",
	"sanitize-output",
	"error-recovery",
}

// SeedAdminReviewDSHDTemplateSlugs is the admin-review set
// (privacy posture has org-wide implications).
var SeedAdminReviewDSHDTemplateSlugs = []string{
	"redact-pii",
}

// SeedExpectedDSHDTemplateRowCount = 6.
const SeedExpectedDSHDTemplateRowCount = 6
