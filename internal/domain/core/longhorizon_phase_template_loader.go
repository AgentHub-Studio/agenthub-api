package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreLongHorizonPhaseTemplate is a platform-managed blueprint paired
// with FUTURE-004 LongHorizonTask. Each row is one phase of a multi-
// week task template; multiple rows compose one task template via
// shared task_template_slug.
type CoreLongHorizonPhaseTemplate struct {
	ID                    uuid.UUID
	TaskTemplateSlug      string
	PhaseSlug             string
	Name                  string
	Description           string
	DependsOn             string
	EstimatedDays         int
	ExpectedDeliverables  string
	RequiresAdminReview   bool
	SortOrder             int
	IsActive              bool
}

// DependsOnList parses comma-separated dependency phase slugs.
func (t CoreLongHorizonPhaseTemplate) DependsOnList() []string {
	if strings.TrimSpace(t.DependsOn) == "" {
		return nil
	}
	parts := strings.Split(t.DependsOn, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CoreLongHorizonPhaseTemplateLoader loads phase templates.
type CoreLongHorizonPhaseTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreLongHorizonPhaseTemplateLoader creates the loader.
func NewCoreLongHorizonPhaseTemplateLoader(pool *pgxpool.Pool) *CoreLongHorizonPhaseTemplateLoader {
	return &CoreLongHorizonPhaseTemplateLoader{pool: pool}
}

// LoadAll returns active phases ordered by sort_order then phase_slug.
func (l *CoreLongHorizonPhaseTemplateLoader) LoadAll(ctx context.Context) ([]CoreLongHorizonPhaseTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, task_template_slug, phase_slug, name, description,
		       depends_on, estimated_days, expected_deliverables,
		       requires_admin_review, sort_order, is_active
		  FROM ah_core.longhorizon_phase_template
		 WHERE is_active = true
		 ORDER BY sort_order, phase_slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.longhorizon_phase_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query longhorizon_phase_template: %w", err)
	}
	defer rows.Close()

	var out []CoreLongHorizonPhaseTemplate
	for rows.Next() {
		var t CoreLongHorizonPhaseTemplate
		if err := rows.Scan(
			&t.ID, &t.TaskTemplateSlug, &t.PhaseSlug, &t.Name, &t.Description,
			&t.DependsOn, &t.EstimatedDays, &t.ExpectedDeliverables,
			&t.RequiresAdminReview, &t.SortOrder, &t.IsActive,
		); err != nil {
			return nil, fmt.Errorf("core: scan longhorizon_phase_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate longhorizon_phase_template: %w", err)
	}
	return out, nil
}

// LoadByTaskTemplate returns all phases for one task template,
// ordered by sort_order.
func (l *CoreLongHorizonPhaseTemplateLoader) LoadByTaskTemplate(ctx context.Context, taskSlug string) ([]CoreLongHorizonPhaseTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreLongHorizonPhaseTemplate
	for _, t := range all {
		if t.TaskTemplateSlug == taskSlug {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// FindByTaskAndPhase returns one specific phase.
func (l *CoreLongHorizonPhaseTemplateLoader) FindByTaskAndPhase(ctx context.Context, taskSlug, phaseSlug string) (CoreLongHorizonPhaseTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreLongHorizonPhaseTemplate{}, false, err
	}
	for _, t := range all {
		if t.TaskTemplateSlug == taskSlug && t.PhaseSlug == phaseSlug {
			return t, true, nil
		}
	}
	return CoreLongHorizonPhaseTemplate{}, false, nil
}

// ListTaskTemplateSlugs returns the unique set of task_template_slug values.
func (l *CoreLongHorizonPhaseTemplateLoader) ListTaskTemplateSlugs(ctx context.Context) ([]string, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, t := range all {
		if !seen[t.TaskTemplateSlug] {
			out = append(out, t.TaskTemplateSlug)
			seen[t.TaskTemplateSlug] = true
		}
	}
	return out, nil
}

// SeedExpectedLongHorizonTaskTemplateSlugs is the closed canonical set
// of task templates seeded.
var SeedExpectedLongHorizonTaskTemplateSlugs = []string{
	"customer-30day-monitoring",
	"quarterly-product-research",
	"compliance-annual-recert",
}

// SeedExpectedLongHorizonPhaseSlugs maps task_template_slug → expected
// phase_slugs. Used by integration tests to assert phase composition.
var SeedExpectedLongHorizonPhaseSlugs = map[string][]string{
	"customer-30day-monitoring": {
		"baseline-snapshot", "daily-checkin-loop", "final-report",
	},
	"quarterly-product-research": {
		"scoping-and-questions", "drafting-and-evidence", "final-review-and-publish",
	},
	"compliance-annual-recert": {
		"evidence-collection", "admin-attestation",
	},
}

// SeedAdminReviewLongHorizonPhases is the closed admin-review set.
// Format: "task/phase".
var SeedAdminReviewLongHorizonPhases = []string{
	"customer-30day-monitoring/final-report",
	"quarterly-product-research/final-review-and-publish",
	"compliance-annual-recert/admin-attestation",
}

// SeedExpectedLongHorizonPhaseRowCount = 8 (3 + 3 + 2).
const SeedExpectedLongHorizonPhaseRowCount = 8
