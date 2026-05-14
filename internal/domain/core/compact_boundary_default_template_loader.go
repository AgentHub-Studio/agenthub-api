package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCompactBoundaryDefaultTemplate is a platform-managed blueprint
// paired with CTX-011 CompactBoundaryRegistry. Each row encodes a
// trigger profile (when compaction fires) tenants pick instead of
// inventing thresholds.
type CoreCompactBoundaryDefaultTemplate struct {
	ID                        uuid.UUID
	Slug                      string
	Name                      string
	Description               string
	TriggerKind               string
	MaxTurnsBeforeCompact     int
	TokenPressurePct          int
	CostThresholdUSD          float64
	IdleSeconds               int
	PreserveTailMessages      int
	TargetUseCase             string
	RecommendedForModelFamily string
	RequiresAdminReview       bool
	IsRecommended             bool
	IsActive                  bool
	SortOrder                 int
}

// IdleDuration returns IdleSeconds as a Duration.
func (t CoreCompactBoundaryDefaultTemplate) IdleDuration() time.Duration {
	return time.Duration(t.IdleSeconds) * time.Second
}

// CoreCompactBoundaryDefaultTemplateLoader loads compact boundary trigger templates.
type CoreCompactBoundaryDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCompactBoundaryDefaultTemplateLoader creates the loader.
func NewCoreCompactBoundaryDefaultTemplateLoader(pool *pgxpool.Pool) *CoreCompactBoundaryDefaultTemplateLoader {
	return &CoreCompactBoundaryDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreCompactBoundaryDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreCompactBoundaryDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, trigger_kind,
		       max_turns_before_compact, token_pressure_pct,
		       cost_threshold_usd, idle_seconds, preserve_tail_messages,
		       target_use_case, recommended_for_model_family,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.compact_boundary_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.compact_boundary_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query compact_boundary_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreCompactBoundaryDefaultTemplate
	for rows.Next() {
		var t CoreCompactBoundaryDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TriggerKind,
			&t.MaxTurnsBeforeCompact, &t.TokenPressurePct,
			&t.CostThresholdUSD, &t.IdleSeconds, &t.PreserveTailMessages,
			&t.TargetUseCase, &t.RecommendedForModelFamily,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan compact_boundary_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate compact_boundary_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreCompactBoundaryDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreCompactBoundaryDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreCompactBoundaryDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreCompactBoundaryDefaultTemplate{}, false, nil
}

// LoadByTriggerKind filters by trigger kind.
func (l *CoreCompactBoundaryDefaultTemplateLoader) LoadByTriggerKind(ctx context.Context, kind string) ([]CoreCompactBoundaryDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCompactBoundaryDefaultTemplate
	for _, t := range all {
		if t.TriggerKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreCompactBoundaryDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreCompactBoundaryDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCompactBoundaryDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedCBDTemplateSlugs is the closed canonical set.
var SeedExpectedCBDTemplateSlugs = []string{
	"hybrid-balanced",
	"turn-count-strict",
	"token-pressure-driven",
	"cost-strict",
	"idle-driven",
	"dev-debug-no-compact",
}

// SeedExpectedCBDTemplateTriggerKinds is the closed trigger-kind set.
var SeedExpectedCBDTemplateTriggerKinds = []string{
	"hybrid_balanced", "turn_count", "token_pressure",
	"cost", "idle", "dev_debug",
}

// SeedExpectedCBDTemplateUseCases is the closed use-case set.
var SeedExpectedCBDTemplateUseCases = []string{
	"general", "research",
}

// SeedExpectedCBDTemplateModelFamilies is the closed model-family set.
var SeedExpectedCBDTemplateModelFamilies = []string{
	"mid_tier", "large_context", "dev_local",
}

// SeedRecommendedCBDTemplateSlugs is the safe one-click subset.
var SeedRecommendedCBDTemplateSlugs = []string{
	"hybrid-balanced",
	"turn-count-strict",
	"token-pressure-driven",
	"cost-strict",
	"idle-driven",
}

// SeedAdminReviewCBDTemplateSlugs lists templates requiring admin review.
var SeedAdminReviewCBDTemplateSlugs = []string{
	"cost-strict",
}

// SeedExpectedCBDTemplateRowCount = 6.
const SeedExpectedCBDTemplateRowCount = 6
