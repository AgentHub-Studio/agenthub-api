package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCustomAgentDefinitionDefaultTemplate is a starter shape paired
// with SUB-003 CustomAgentDefinition. Each template suggests a
// proven recipe (bare/derived/researcher_derived/dual_loop) so tenants
// can clone-and-modify instead of designing from scratch.
type CoreCustomAgentDefinitionDefaultTemplate struct {
	ID                         uuid.UUID
	Slug                       string
	ShapeKind                  string
	Name                       string
	Description                string
	SystemPromptTemplate       string
	ToolsetPolicySlug          string
	InheritanceModeSlug        string
	SummaryShapeSlug           string
	DerivedFromBuiltinSlug     string // "" means greenfield
	SuggestedVisibility        string
	RecommendedStartingVersion string
	IsRecommended              bool
	IsActive                   bool
	SortOrder                  int
}

// CoreCustomAgentDefinitionDefaultTemplateLoader loads templates.
type CoreCustomAgentDefinitionDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCustomAgentDefinitionDefaultTemplateLoader creates the loader.
func NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool *pgxpool.Pool) *CoreCustomAgentDefinitionDefaultTemplateLoader {
	return &CoreCustomAgentDefinitionDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreCustomAgentDefinitionDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreCustomAgentDefinitionDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, shape_kind, name, description, system_prompt_template,
		       toolset_policy_slug, inheritance_mode_slug, summary_shape_slug,
		       derived_from_builtin_slug, suggested_visibility,
		       recommended_starting_version, is_recommended, is_active, sort_order
		  FROM ah_core.custom_agent_definition_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.custom_agent_definition_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query custom_agent_definition_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreCustomAgentDefinitionDefaultTemplate
	for rows.Next() {
		var t CoreCustomAgentDefinitionDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.ShapeKind, &t.Name, &t.Description,
			&t.SystemPromptTemplate,
			&t.ToolsetPolicySlug, &t.InheritanceModeSlug, &t.SummaryShapeSlug,
			&t.DerivedFromBuiltinSlug, &t.SuggestedVisibility,
			&t.RecommendedStartingVersion, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan custom_agent_definition_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate custom_agent_definition_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreCustomAgentDefinitionDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreCustomAgentDefinitionDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreCustomAgentDefinitionDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreCustomAgentDefinitionDefaultTemplate{}, false, nil
}

// LoadByShapeKind filters by shape_kind.
func (l *CoreCustomAgentDefinitionDefaultTemplateLoader) LoadByShapeKind(ctx context.Context, kind string) ([]CoreCustomAgentDefinitionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCustomAgentDefinitionDefaultTemplate
	for _, t := range all {
		if t.ShapeKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadDerivedFromBuiltin filters by SUB-002 lineage slug.
func (l *CoreCustomAgentDefinitionDefaultTemplateLoader) LoadDerivedFromBuiltin(ctx context.Context, builtinSlug string) ([]CoreCustomAgentDefinitionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCustomAgentDefinitionDefaultTemplate
	for _, t := range all {
		if t.DerivedFromBuiltinSlug == builtinSlug {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadGreenfield returns templates with no SUB-002 lineage.
func (l *CoreCustomAgentDefinitionDefaultTemplateLoader) LoadGreenfield(ctx context.Context) ([]CoreCustomAgentDefinitionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCustomAgentDefinitionDefaultTemplate
	for _, t := range all {
		if t.DerivedFromBuiltinSlug == "" {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedCADTemplateSlugs is the closed canonical set.
var SeedExpectedCADTemplateSlugs = []string{
	"bare-greenfield",
	"researcher-derived",
	"coder-derived",
	"dual-loop-planner",
}

// SeedExpectedCADTemplateShapeKinds is the closed shape-kind set.
var SeedExpectedCADTemplateShapeKinds = []string{
	"bare", "researcher_derived", "derived", "dual_loop",
}

// SeedExpectedCADTemplateToolsetSlugs — SUB-005 refs used by these starters.
var SeedExpectedCADTemplateToolsetSlugs = []string{
	"documentation-readonly-allowlist",
	"code-write-scoped",
}

// SeedExpectedCADTemplateInheritanceSlugs — SUB-006 refs.
var SeedExpectedCADTemplateInheritanceSlugs = []string{
	"restrict-to-readonly",
	"extend-parent-rights",
}

// SeedExpectedCADTemplateSummarySlugs — SUB-010 refs.
var SeedExpectedCADTemplateSummarySlugs = []string{
	"structured-findings",
	"success-with-artifacts",
	"plan-only",
}

// SeedExpectedCADTemplateBuiltinLineageSlugs — SUB-002 refs used (3
// templates have lineage; bare-greenfield has none).
var SeedExpectedCADTemplateBuiltinLineageSlugs = []string{
	"researcher-baseline",
	"coder-baseline",
	"planner-baseline",
}

// SeedExpectedCADTemplateVisibilities — closed visibility set.
var SeedExpectedCADTemplateVisibilities = []string{"private"}

// SeedRecommendedCADTemplateSlugs — all 4 starters are recommended.
var SeedRecommendedCADTemplateSlugs = []string{
	"bare-greenfield", "researcher-derived", "coder-derived", "dual-loop-planner",
}

// SeedGreenfieldCADTemplateSlugs — only the bare shape has no SUB-002
// lineage.
var SeedGreenfieldCADTemplateSlugs = []string{"bare-greenfield"}

// SeedExpectedCADTemplateRowCount = 4.
const SeedExpectedCADTemplateRowCount = 4
