package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreContextSectionBudgetTemplate is a platform-managed blueprint
// paired with CTX-001 ContextAssembler. Each template ties a total
// token budget + per-section caps into a ready-to-use profile.
type CoreContextSectionBudgetTemplate struct {
	ID                         uuid.UUID
	Slug                       string
	Name                       string
	Description                string
	TotalBudgetTokens          int
	CapSystem                  int
	CapMemory                  int
	CapRules                   int
	CapSkillCatalog            int
	CapToolCatalog             int
	CapKBSummary               int
	CapRecentMessages          int
	CapAuxPrompt               int
	CapScratchpad              int
	TargetUseCase              string
	RecommendedForModelFamily  string
	IsRecommended              bool
	IsActive                   bool
	SortOrder                  int
}

// PerSectionCaps returns a map of section-kind → cap, matching the
// ContextSectionKind labels used in CTX-001.
func (t CoreContextSectionBudgetTemplate) PerSectionCaps() map[string]int {
	return map[string]int{
		"system":          t.CapSystem,
		"memory":          t.CapMemory,
		"rules":           t.CapRules,
		"skill_catalog":   t.CapSkillCatalog,
		"tool_catalog":    t.CapToolCatalog,
		"kb_summary":      t.CapKBSummary,
		"recent_messages": t.CapRecentMessages,
		"aux_prompt":      t.CapAuxPrompt,
		"scratchpad":      t.CapScratchpad,
	}
}

// CoreContextSectionBudgetTemplateLoader loads context budget templates.
type CoreContextSectionBudgetTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreContextSectionBudgetTemplateLoader creates the loader.
func NewCoreContextSectionBudgetTemplateLoader(pool *pgxpool.Pool) *CoreContextSectionBudgetTemplateLoader {
	return &CoreContextSectionBudgetTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreContextSectionBudgetTemplateLoader) LoadAll(ctx context.Context) ([]CoreContextSectionBudgetTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, total_budget_tokens,
		       cap_system, cap_memory, cap_rules, cap_skill_catalog,
		       cap_tool_catalog, cap_kb_summary, cap_recent_messages,
		       cap_aux_prompt, cap_scratchpad,
		       target_use_case, recommended_for_model_family,
		       is_recommended, is_active, sort_order
		  FROM ah_core.context_section_budget_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.context_section_budget_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query context_section_budget_template: %w", err)
	}
	defer rows.Close()

	var out []CoreContextSectionBudgetTemplate
	for rows.Next() {
		var t CoreContextSectionBudgetTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TotalBudgetTokens,
			&t.CapSystem, &t.CapMemory, &t.CapRules, &t.CapSkillCatalog,
			&t.CapToolCatalog, &t.CapKBSummary, &t.CapRecentMessages,
			&t.CapAuxPrompt, &t.CapScratchpad,
			&t.TargetUseCase, &t.RecommendedForModelFamily,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan context_section_budget_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate context_section_budget_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreContextSectionBudgetTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreContextSectionBudgetTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreContextSectionBudgetTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreContextSectionBudgetTemplate{}, false, nil
}

// LoadByUseCase filters by target_use_case.
func (l *CoreContextSectionBudgetTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreContextSectionBudgetTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextSectionBudgetTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreContextSectionBudgetTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreContextSectionBudgetTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextSectionBudgetTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedContextBudgetTemplateSlugs is the closed canonical set.
var SeedExpectedContextBudgetTemplateSlugs = []string{
	"balanced-default",
	"small-context-tight",
	"research-heavy-128k",
	"conversation-heavy-64k",
	"code-heavy-64k",
	"minimum-viable-4k",
}

// SeedExpectedContextBudgetTemplateUseCases is the closed set of use cases.
var SeedExpectedContextBudgetTemplateUseCases = []string{
	"general", "research", "conversation", "code",
}

// SeedExpectedContextBudgetTemplateModelFamilies is the closed set of
// recommended model families.
var SeedExpectedContextBudgetTemplateModelFamilies = []string{
	"mid_tier", "small_local", "large_context", "tiny_legacy",
}

// SeedRecommendedContextBudgetTemplateSlugs lists safe one-click defaults.
// minimum-viable-4k is excluded — it's a last-resort budget, not a
// recommended starting point.
var SeedRecommendedContextBudgetTemplateSlugs = []string{
	"balanced-default",
	"small-context-tight",
	"research-heavy-128k",
	"conversation-heavy-64k",
	"code-heavy-64k",
}

// SeedExpectedContextBudgetTemplateRowCount = 6.
const SeedExpectedContextBudgetTemplateRowCount = 6
