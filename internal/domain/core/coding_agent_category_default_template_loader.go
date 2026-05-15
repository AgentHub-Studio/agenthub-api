package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCodingAgentCategoryTemplate is a platform-managed coding agent taxonomy preset.
// §13.1 Table 5: four categories spanning passive (inline completion) to autonomous (fully autonomous).
type CoreCodingAgentCategoryTemplate struct {
	ID               uuid.UUID
	Slug             string
	Label            string
	Description      string
	GradientIndex    int      // 0=inline_completion → 3=fully_autonomous
	ExecutionPattern string
	IsolationModel   string
	ExampleSystems   []string
	IsAgentHubTarget bool
	RecommendedFor   []string
	SortOrder        int
}

// SeedExpectedCodingAgentCategorySlugs is the canonical closed set from Table 5.
var SeedExpectedCodingAgentCategorySlugs = []string{
	"inline_completion", "chat_integrated", "agentic_cli", "fully_autonomous",
}

// SeedExpectedCodingAgentCategoryRowCount matches the migration INSERT count.
const SeedExpectedCodingAgentCategoryRowCount = 4

// SeedCodingAgentCategoryAgentHubSlugs are the categories AgentHub web platform covers.
var SeedCodingAgentCategoryAgentHubSlugs = []string{"chat_integrated", "agentic_cli"}

// SeedCodingAgentCategoryDefaultSlug is AgentHub's primary deployment category.
const SeedCodingAgentCategoryDefaultSlug = "chat_integrated"

// SeedCodingAgentCategorySlugRE validates slug format (snake_case).
var SeedCodingAgentCategorySlugRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// SeedCodingAgentCategoryExecutionPatterns is the closed set of execution_pattern values.
var SeedCodingAgentCategoryExecutionPatterns = []string{
	"editor_plugin", "ide_coupled_product", "tool_use_loop", "sandbox_planning",
}

// CoreCodingAgentCategoryDefaultTemplateLoader loads category presets from ah_core.
type CoreCodingAgentCategoryDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCodingAgentCategoryDefaultTemplateLoader creates a loader.
func NewCoreCodingAgentCategoryDefaultTemplateLoader(pool *pgxpool.Pool) *CoreCodingAgentCategoryDefaultTemplateLoader {
	return &CoreCodingAgentCategoryDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all categories ordered by gradient_index, slug.
func (l *CoreCodingAgentCategoryDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreCodingAgentCategoryTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       gradient_index, execution_pattern, isolation_model,
		       example_systems, is_agenthub_target, recommended_for, sort_order
		  FROM ah_core.coding_agent_category_template
		 ORDER BY gradient_index, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.coding_agent_category_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query coding_agent_category_template: %w", err)
	}
	defer rows.Close()

	var categories []CoreCodingAgentCategoryTemplate
	for rows.Next() {
		var t CoreCodingAgentCategoryTemplate
		var examplesRaw, recommendedRaw []byte
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.GradientIndex, &t.ExecutionPattern, &t.IsolationModel,
			&examplesRaw, &t.IsAgentHubTarget, &recommendedRaw, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan coding_agent_category_template: %w", err)
		}
		if len(examplesRaw) > 0 {
			if err := json.Unmarshal(examplesRaw, &t.ExampleSystems); err != nil {
				return nil, fmt.Errorf("core: unmarshal example_systems: %w", err)
			}
		}
		if len(recommendedRaw) > 0 {
			if err := json.Unmarshal(recommendedRaw, &t.RecommendedFor); err != nil {
				return nil, fmt.Errorf("core: unmarshal recommended_for: %w", err)
			}
		}
		categories = append(categories, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate coding_agent_category_template: %w", err)
	}
	return categories, nil
}

// FindBySlug returns one category by slug.
func (l *CoreCodingAgentCategoryDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreCodingAgentCategoryTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreCodingAgentCategoryTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreCodingAgentCategoryTemplate{}, false, nil
}

// LoadAgentHubTargets returns only the categories flagged as AgentHub targets.
func (l *CoreCodingAgentCategoryDefaultTemplateLoader) LoadAgentHubTargets(ctx context.Context) ([]CoreCodingAgentCategoryTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCodingAgentCategoryTemplate
	for _, t := range all {
		if t.IsAgentHubTarget {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
