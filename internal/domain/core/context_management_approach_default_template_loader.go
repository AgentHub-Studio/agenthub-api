package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreContextManagementApproachTemplate is a platform-managed preset for one of the
// five context-management strategies from Table 6 (arXiv:2604.14228v1, §13.2).
type CoreContextManagementApproachTemplate struct {
	ID                 uuid.UUID
	Slug               string
	Label              string
	Description        string
	Mechanism          string // human-readable mechanism name from Table 6
	Granularity        string // coarse|medium|fine|very_fine
	IsAgentHubApproach bool   // true = graduated_compaction only
	SortOrder          int
}

// SeedExpectedContextManagementApproachSlugs is the canonical closed set from Table 6.
var SeedExpectedContextManagementApproachSlugs = []string{
	"simple_truncation",
	"sliding_window",
	"rag",
	"single_summarization",
	"graduated_compaction",
}

// SeedExpectedContextManagementApproachRowCount matches the migration INSERT count.
const SeedExpectedContextManagementApproachRowCount = 5

// SeedContextManagementAgentHubApproachSlug is the approach AgentHub uses.
const SeedContextManagementAgentHubApproachSlug = "graduated_compaction"

// SeedContextManagementCoarseApproachSlugs are the two coarse-granularity entries.
var SeedContextManagementCoarseApproachSlugs = []string{
	"simple_truncation",
	"single_summarization",
}

// SeedContextManagementGranularityValues is the closed set of valid granularity values.
var SeedContextManagementGranularityValues = []string{"coarse", "medium", "fine", "very_fine"}

// CoreContextManagementApproachDefaultTemplateLoader loads Table 6 presets from ah_core.
type CoreContextManagementApproachDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreContextManagementApproachDefaultTemplateLoader creates a loader.
func NewCoreContextManagementApproachDefaultTemplateLoader(pool *pgxpool.Pool) *CoreContextManagementApproachDefaultTemplateLoader {
	return &CoreContextManagementApproachDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all five approaches ordered by sort_order (Table 6 row order).
func (l *CoreContextManagementApproachDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreContextManagementApproachTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       mechanism, granularity, is_agenthub_approach, sort_order
		  FROM ah_core.context_management_approach_template
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.context_management_approach_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query context_management_approach_template: %w", err)
	}
	defer rows.Close()

	var approaches []CoreContextManagementApproachTemplate
	for rows.Next() {
		var t CoreContextManagementApproachTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.Mechanism, &t.Granularity, &t.IsAgentHubApproach, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan context_management_approach_template: %w", err)
		}
		approaches = append(approaches, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate context_management_approach_template: %w", err)
	}
	return approaches, nil
}

// FindBySlug returns one approach template by slug.
func (l *CoreContextManagementApproachDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreContextManagementApproachTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreContextManagementApproachTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreContextManagementApproachTemplate{}, false, nil
}

// LoadAgentHubApproach returns the approach used by the AgentHub platform.
func (l *CoreContextManagementApproachDefaultTemplateLoader) LoadAgentHubApproach(ctx context.Context) (CoreContextManagementApproachTemplate, bool, error) {
	return l.FindBySlug(ctx, SeedContextManagementAgentHubApproachSlug)
}

// LoadByGranularity returns approaches with the given granularity level.
func (l *CoreContextManagementApproachDefaultTemplateLoader) LoadByGranularity(ctx context.Context, granularity string) ([]CoreContextManagementApproachTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextManagementApproachTemplate
	for _, t := range all {
		if t.Granularity == granularity {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
