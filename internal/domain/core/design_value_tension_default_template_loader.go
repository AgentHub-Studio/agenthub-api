package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreDesignValueTensionTemplate is a platform-managed preset for one of the
// five Table 4 design value tensions (arXiv:2604.14228v1, §11.2).
type CoreDesignValueTensionTemplate struct {
	ID              uuid.UUID
	Slug            string
	Label           string
	Description     string
	Value1          string // human_authority|safety|reliability|capability|adaptability
	Value2          string
	TensionLabel    string
	EvidenceSummary string
	SortOrder       int
}

// SeedExpectedDesignValueTensionSlugs is the canonical closed set from Table 4.
var SeedExpectedDesignValueTensionSlugs = []string{
	"authority_safety",
	"safety_capability",
	"adaptability_safety",
	"capability_adaptability",
	"capability_reliability",
}

// SeedExpectedDesignValueTensionRowCount matches the migration INSERT count.
const SeedExpectedDesignValueTensionRowCount = 5

// SeedDesignValueTensionCapabilitySlugs are tensions where capability is one of the values.
var SeedDesignValueTensionCapabilitySlugs = []string{
	"safety_capability",
	"capability_adaptability",
	"capability_reliability",
}

// SeedDesignValueTensionSafetySlugs are tensions where safety is one of the values.
var SeedDesignValueTensionSafetySlugs = []string{
	"authority_safety",
	"safety_capability",
	"adaptability_safety",
}

// CoreDesignValueTensionDefaultTemplateLoader loads Table 4 tension presets from ah_core.
type CoreDesignValueTensionDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreDesignValueTensionDefaultTemplateLoader creates a loader.
func NewCoreDesignValueTensionDefaultTemplateLoader(pool *pgxpool.Pool) *CoreDesignValueTensionDefaultTemplateLoader {
	return &CoreDesignValueTensionDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all five tensions ordered by sort_order (Table 4 row order).
func (l *CoreDesignValueTensionDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreDesignValueTensionTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       value1, value2, tension_label, evidence_summary, sort_order
		  FROM ah_core.design_value_tension_template
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.design_value_tension_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query design_value_tension_template: %w", err)
	}
	defer rows.Close()

	var tensions []CoreDesignValueTensionTemplate
	for rows.Next() {
		var t CoreDesignValueTensionTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.Value1, &t.Value2, &t.TensionLabel, &t.EvidenceSummary, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan design_value_tension_template: %w", err)
		}
		tensions = append(tensions, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate design_value_tension_template: %w", err)
	}
	return tensions, nil
}

// FindBySlug returns one tension template by slug.
func (l *CoreDesignValueTensionDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreDesignValueTensionTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreDesignValueTensionTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreDesignValueTensionTemplate{}, false, nil
}

// LoadInvolvingValue returns tensions where the given design value slug appears as value1 or value2.
func (l *CoreDesignValueTensionDefaultTemplateLoader) LoadInvolvingValue(ctx context.Context, value string) ([]CoreDesignValueTensionTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreDesignValueTensionTemplate
	for _, t := range all {
		if t.Value1 == value || t.Value2 == value {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
