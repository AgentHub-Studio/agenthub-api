package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreBubbleModePolicyDefaultTemplate is a platform-managed blueprint
// paired with PERM-003b BubbleModeEvaluator. Each template captures a
// proven (max_depth + requires_parent + history_capture) combination.
type CoreBubbleModePolicyDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetUseCase            string
	SafetyPosture            string
	MaxBubbleDepth           int
	RequiresParentResolver   bool
	RecordsChainHistory      bool
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreBubbleModePolicyDefaultTemplateLoader loads templates.
type CoreBubbleModePolicyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreBubbleModePolicyDefaultTemplateLoader creates the loader.
func NewCoreBubbleModePolicyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreBubbleModePolicyDefaultTemplateLoader {
	return &CoreBubbleModePolicyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreBubbleModePolicyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreBubbleModePolicyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_use_case, safety_posture,
		       max_bubble_depth, requires_parent_resolver, records_chain_history,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.bubble_mode_policy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.bubble_mode_policy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query bubble_mode_policy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreBubbleModePolicyDefaultTemplate
	for rows.Next() {
		var t CoreBubbleModePolicyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetUseCase, &t.SafetyPosture,
			&t.MaxBubbleDepth, &t.RequiresParentResolver, &t.RecordsChainHistory,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan bubble_mode_policy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate bubble_mode_policy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreBubbleModePolicyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreBubbleModePolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreBubbleModePolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreBubbleModePolicyDefaultTemplate{}, false, nil
}

// LoadByUseCase filters by use_case.
func (l *CoreBubbleModePolicyDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreBubbleModePolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBubbleModePolicyDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadBySafetyPosture filters by posture.
func (l *CoreBubbleModePolicyDefaultTemplateLoader) LoadBySafetyPosture(ctx context.Context, posture string) ([]CoreBubbleModePolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBubbleModePolicyDefaultTemplate
	for _, t := range all {
		if t.SafetyPosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreBubbleModePolicyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreBubbleModePolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBubbleModePolicyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedBMPDTemplateSlugs is the closed canonical set.
var SeedExpectedBMPDTemplateSlugs = []string{
	"leaf-auto-deny",
	"single-hop",
	"two-hop-routine",
	"three-hop-orchestration",
	"five-hop-research",
}

// SeedExpectedBMPDTemplateUseCases is the closed use-case set.
var SeedExpectedBMPDTemplateUseCases = []string{
	"leaf_subagent", "two_tier", "three_tier_orchestration",
	"multi_stage_pipeline", "research_curator_network",
}

// SeedExpectedBMPDTemplateSafetyPostures is the closed posture set.
var SeedExpectedBMPDTemplateSafetyPostures = []string{
	"strict", "balanced", "permissive",
}

// SeedExpectedBMPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedBMPDTemplateTenantKinds = []string{"general"}

// SeedRecommendedBMPDTemplateSlugs — all 5 are recommended.
var SeedRecommendedBMPDTemplateSlugs = []string{
	"leaf-auto-deny",
	"single-hop",
	"two-hop-routine",
	"three-hop-orchestration",
	"five-hop-research",
}

// SeedAdminReviewBMPDTemplateSlugs — every preset except routine
// single-hop requires admin review.
var SeedAdminReviewBMPDTemplateSlugs = []string{
	"leaf-auto-deny",
	"two-hop-routine",
	"three-hop-orchestration",
	"five-hop-research",
}

// SeedExpectedBMPDTemplateRowCount = 5.
const SeedExpectedBMPDTemplateRowCount = 5
