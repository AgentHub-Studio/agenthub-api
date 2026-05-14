package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreExtensionMechanismProfileTemplate is a platform-managed preset for one of the
// four Table 2 extension mechanisms.
type CoreExtensionMechanismProfileTemplate struct {
	ID                    uuid.UUID
	Slug                  string
	Label                 string
	Description           string
	UniqueCapability      string
	ContextCostCategory   string // micro|small|medium|large|heavy
	InsertionPoint        string // assemble|model|execute|all
	IsZeroCostByDefault   bool   // hooks only
	CoversAllInsertPoints bool   // plugins only
	SortOrder             int
}

// SeedExpectedExtensionMechanismSlugs is the canonical closed set from Table 2.
var SeedExpectedExtensionMechanismSlugs = []string{
	"hooks",
	"skills",
	"plugins",
	"mcp_servers",
}

// SeedExpectedExtensionMechanismRowCount matches the migration INSERT count.
const SeedExpectedExtensionMechanismRowCount = 4

// SeedExtensionMechanismZeroCostSlugs are mechanisms with zero context cost by default.
var SeedExtensionMechanismZeroCostSlugs = []string{"hooks"}

// SeedExtensionMechanismAllInsertPointSlugs are mechanisms covering all loop insertion points.
var SeedExtensionMechanismAllInsertPointSlugs = []string{"plugins"}

// SeedExtensionMechanismInsertionPoints is the closed set of valid insertion_point values.
var SeedExtensionMechanismInsertionPoints = []string{"assemble", "model", "execute", "all"}

// CoreExtensionMechanismProfileDefaultTemplateLoader loads mechanism presets from ah_core.
type CoreExtensionMechanismProfileDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreExtensionMechanismProfileDefaultTemplateLoader creates a loader.
func NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool *pgxpool.Pool) *CoreExtensionMechanismProfileDefaultTemplateLoader {
	return &CoreExtensionMechanismProfileDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all mechanisms ordered by sort_order (context cost ascending).
func (l *CoreExtensionMechanismProfileDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreExtensionMechanismProfileTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description, unique_capability,
		       context_cost_category, insertion_point,
		       is_zero_cost_by_default, covers_all_insert_points, sort_order
		  FROM ah_core.extension_mechanism_profile_template
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.extension_mechanism_profile_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query extension_mechanism_profile_template: %w", err)
	}
	defer rows.Close()

	var mechanisms []CoreExtensionMechanismProfileTemplate
	for rows.Next() {
		var t CoreExtensionMechanismProfileTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description, &t.UniqueCapability,
			&t.ContextCostCategory, &t.InsertionPoint,
			&t.IsZeroCostByDefault, &t.CoversAllInsertPoints, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan extension_mechanism_profile_template: %w", err)
		}
		mechanisms = append(mechanisms, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate extension_mechanism_profile_template: %w", err)
	}
	return mechanisms, nil
}

// FindBySlug returns one mechanism template by slug.
func (l *CoreExtensionMechanismProfileDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreExtensionMechanismProfileTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreExtensionMechanismProfileTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreExtensionMechanismProfileTemplate{}, false, nil
}

// LoadZeroCostByDefault returns mechanisms with zero context cost by default.
func (l *CoreExtensionMechanismProfileDefaultTemplateLoader) LoadZeroCostByDefault(ctx context.Context) ([]CoreExtensionMechanismProfileTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionMechanismProfileTemplate
	for _, t := range all {
		if t.IsZeroCostByDefault {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadAtInsertionPoint returns mechanisms that operate at the given insertion point.
// Mechanisms with covers_all_insert_points=true are included for any point.
func (l *CoreExtensionMechanismProfileDefaultTemplateLoader) LoadAtInsertionPoint(ctx context.Context, point string) ([]CoreExtensionMechanismProfileTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionMechanismProfileTemplate
	for _, t := range all {
		if t.InsertionPoint == point || t.CoversAllInsertPoints {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
