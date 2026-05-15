package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreComplexityDriftThresholdDefaultTemplate is a starter warn/critical
// band paired with HUMAN-006 ComplexityDriftSignal. Fresh tenants
// inherit sensible defaults instead of guessing what scope_creep=??
// should be.
type CoreComplexityDriftThresholdDefaultTemplate struct {
	ID                        uuid.UUID
	Slug                      string
	Signal                    string // matches HUMAN-006 enum byte-for-byte
	WarnAt                    float64
	CriticalAt                float64
	Unit                      string
	Description               string
	AppliesToSubjectKind      string
	TypicalDedupWindowSeconds int
	IsRecommended             bool
	IsActive                  bool
	SortOrder                 int
}

// CoreComplexityDriftThresholdDefaultTemplateLoader loads templates.
type CoreComplexityDriftThresholdDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreComplexityDriftThresholdDefaultTemplateLoader creates the loader.
func NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool *pgxpool.Pool) *CoreComplexityDriftThresholdDefaultTemplateLoader {
	return &CoreComplexityDriftThresholdDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreComplexityDriftThresholdDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreComplexityDriftThresholdDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, signal, warn_at, critical_at, unit,
		       description, applies_to_subject_kind,
		       typical_dedup_window_seconds, is_recommended,
		       is_active, sort_order
		  FROM ah_core.complexity_drift_threshold_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.complexity_drift_threshold_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query complexity_drift_threshold_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreComplexityDriftThresholdDefaultTemplate
	for rows.Next() {
		var t CoreComplexityDriftThresholdDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Signal, &t.WarnAt, &t.CriticalAt, &t.Unit,
			&t.Description, &t.AppliesToSubjectKind,
			&t.TypicalDedupWindowSeconds, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan complexity_drift_threshold_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate complexity_drift_threshold_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreComplexityDriftThresholdDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreComplexityDriftThresholdDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreComplexityDriftThresholdDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreComplexityDriftThresholdDefaultTemplate{}, false, nil
}

// FindBySignal returns the template for a given HUMAN-006 signal.
func (l *CoreComplexityDriftThresholdDefaultTemplateLoader) FindBySignal(ctx context.Context, signal string) (CoreComplexityDriftThresholdDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreComplexityDriftThresholdDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Signal == signal {
			return t, true, nil
		}
	}
	return CoreComplexityDriftThresholdDefaultTemplate{}, false, nil
}

// SeedExpectedCDTTemplateSlugs is the closed canonical set.
var SeedExpectedCDTTemplateSlugs = []string{
	"scope-creep-default",
	"dependency-explosion-default",
	"test-decay-default",
	"churn-spike-default",
	"goal-drift-default",
	"cognitive-load-default",
}

// SeedExpectedCDTTemplateSignals — matches HUMAN-006 ComplexityDriftSignal
// enum byte-for-byte (6 signals).
var SeedExpectedCDTTemplateSignals = []string{
	"scope_creep", "dependency_explosion", "test_decay",
	"churn_spike", "goal_drift", "cognitive_load",
}

// SeedExpectedCDTTemplateUnits — closed unit set.
var SeedExpectedCDTTemplateUnits = []string{
	"sub_tasks", "modules_touched", "coverage_drop_ratio",
	"reedits_per_file", "cosine_distance", "context_fill_ratio",
}

// SeedExpectedCDTTemplateSubjectKinds — closed subject-kind set.
var SeedExpectedCDTTemplateSubjectKinds = []string{"run"}

// SeedRecommendedCDTTemplateSlugs — all 6 thresholds recommended.
var SeedRecommendedCDTTemplateSlugs = SeedExpectedCDTTemplateSlugs

// SeedExpectedCDTTemplateRowCount = 6.
const SeedExpectedCDTTemplateRowCount = 6
