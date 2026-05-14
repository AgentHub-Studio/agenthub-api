package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePermissionPrefilterStanceDefaultTemplate is a platform-managed
// blueprint paired with PERM-004 pre-filter. Each template describes
// a stance + baseline deny/confirm patterns tenants can adopt.
type CorePermissionPrefilterStanceDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetStance             string
	TargetUseCase            string
	BaselineDenyPatternsJSON string
	BaselineConfirmPatternsJSON string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// BaselineDenyPatterns parses the JSONB array.
func (t CorePermissionPrefilterStanceDefaultTemplate) BaselineDenyPatterns() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.BaselineDenyPatternsJSON), &out); err != nil {
		return nil, fmt.Errorf("permission_prefilter_stance_template %s: invalid baseline_deny_patterns: %w", t.Slug, err)
	}
	return out, nil
}

// BaselineConfirmPatterns parses the JSONB array.
func (t CorePermissionPrefilterStanceDefaultTemplate) BaselineConfirmPatterns() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.BaselineConfirmPatternsJSON), &out); err != nil {
		return nil, fmt.Errorf("permission_prefilter_stance_template %s: invalid baseline_confirm_patterns: %w", t.Slug, err)
	}
	return out, nil
}

// CorePermissionPrefilterStanceDefaultTemplateLoader loads stance
// templates.
type CorePermissionPrefilterStanceDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePermissionPrefilterStanceDefaultTemplateLoader creates the
// loader.
func NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool *pgxpool.Pool) *CorePermissionPrefilterStanceDefaultTemplateLoader {
	return &CorePermissionPrefilterStanceDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePermissionPrefilterStanceDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePermissionPrefilterStanceDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_stance, target_use_case,
		       baseline_deny_patterns::text, baseline_confirm_patterns::text,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.permission_prefilter_stance_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.permission_prefilter_stance_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query permission_prefilter_stance_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePermissionPrefilterStanceDefaultTemplate
	for rows.Next() {
		var t CorePermissionPrefilterStanceDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetStance, &t.TargetUseCase,
			&t.BaselineDenyPatternsJSON, &t.BaselineConfirmPatternsJSON,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan permission_prefilter_stance_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate permission_prefilter_stance_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePermissionPrefilterStanceDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePermissionPrefilterStanceDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePermissionPrefilterStanceDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePermissionPrefilterStanceDefaultTemplate{}, false, nil
}

// LoadByStance filters by PERM-004 PrefilterStance label.
func (l *CorePermissionPrefilterStanceDefaultTemplateLoader) LoadByStance(ctx context.Context, stance string) ([]CorePermissionPrefilterStanceDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionPrefilterStanceDefaultTemplate
	for _, t := range all {
		if t.TargetStance == stance {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case label.
func (l *CorePermissionPrefilterStanceDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CorePermissionPrefilterStanceDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionPrefilterStanceDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePermissionPrefilterStanceDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePermissionPrefilterStanceDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionPrefilterStanceDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPPSDTemplateSlugs is the closed canonical set.
var SeedExpectedPPSDTemplateSlugs = []string{
	"interactive-default",
	"unattended-batch",
	"lockdown-readonly",
	"audit-strict-trace",
}

// SeedExpectedPPSDTemplateStances matches PERM-004 PrefilterStance
// enum byte-for-byte (2 stances).
var SeedExpectedPPSDTemplateStances = []string{
	"show_confirm", "hide_confirm",
}

// SeedExpectedPPSDTemplateUseCases is the closed use-case set.
var SeedExpectedPPSDTemplateUseCases = []string{
	"interactive_chat", "background_job", "incident_response", "compliance_audit",
}

// SeedExpectedPPSDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedPPSDTemplateTenantKinds = []string{"general"}

// SeedRecommendedPPSDTemplateSlugs — all 4 are recommended.
var SeedRecommendedPPSDTemplateSlugs = []string{
	"interactive-default",
	"unattended-batch",
	"lockdown-readonly",
	"audit-strict-trace",
}

// SeedAdminReviewPPSDTemplateSlugs gates 3 of 4 templates (interactive
// is routine; others change security posture materially).
var SeedAdminReviewPPSDTemplateSlugs = []string{
	"unattended-batch",
	"lockdown-readonly",
	"audit-strict-trace",
}

// SeedExpectedPPSDTemplateRowCount = 4.
const SeedExpectedPPSDTemplateRowCount = 4
