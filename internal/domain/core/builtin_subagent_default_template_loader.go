package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreBuiltinSubagentDefaultTemplate is a platform-managed blueprint
// paired with SUB-002 BuiltinSubagentRole. Each template defines one
// role's prompt + 3 policy slug refs (SUB-005/006/010) so fresh tenants
// can spawn ready-to-use subagents.
type CoreBuiltinSubagentDefaultTemplate struct {
	ID                         uuid.UUID
	Slug                       string
	Role                       string
	Name                       string
	Description                string
	SystemPromptTemplate       string
	DefaultToolsetPolicySlug   string
	DefaultInheritanceModeSlug string
	DefaultSummaryShapeSlug    string
	TypicalTaskClass           string
	RecommendedForTenantKind   string
	RequiresAdminReview        bool
	IsRecommended              bool
	IsActive                   bool
	SortOrder                  int
}

// CoreBuiltinSubagentDefaultTemplateLoader loads templates.
type CoreBuiltinSubagentDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreBuiltinSubagentDefaultTemplateLoader creates the loader.
func NewCoreBuiltinSubagentDefaultTemplateLoader(pool *pgxpool.Pool) *CoreBuiltinSubagentDefaultTemplateLoader {
	return &CoreBuiltinSubagentDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreBuiltinSubagentDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreBuiltinSubagentDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, role, name, description, system_prompt_template,
		       default_toolset_policy_slug, default_inheritance_mode_slug,
		       default_summary_shape_slug, typical_task_class,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.builtin_subagent_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.builtin_subagent_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query builtin_subagent_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreBuiltinSubagentDefaultTemplate
	for rows.Next() {
		var t CoreBuiltinSubagentDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Role, &t.Name, &t.Description, &t.SystemPromptTemplate,
			&t.DefaultToolsetPolicySlug, &t.DefaultInheritanceModeSlug,
			&t.DefaultSummaryShapeSlug, &t.TypicalTaskClass,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan builtin_subagent_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate builtin_subagent_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreBuiltinSubagentDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreBuiltinSubagentDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreBuiltinSubagentDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreBuiltinSubagentDefaultTemplate{}, false, nil
}

// LoadByRole filters by SUB-002 BuiltinSubagentRole.
func (l *CoreBuiltinSubagentDefaultTemplateLoader) LoadByRole(ctx context.Context, role string) ([]CoreBuiltinSubagentDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBuiltinSubagentDefaultTemplate
	for _, t := range all {
		if t.Role == role {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreBuiltinSubagentDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreBuiltinSubagentDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBuiltinSubagentDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedBSDTemplateSlugs is the closed canonical set (7 roles).
var SeedExpectedBSDTemplateSlugs = []string{
	"researcher-baseline",
	"coder-baseline",
	"reviewer-baseline",
	"explorer-baseline",
	"planner-baseline",
	"curator-baseline",
	"documenter-baseline",
}

// SeedExpectedBSDTemplateRoles matches SUB-002 BuiltinSubagentRole enum
// byte-for-byte (7 roles).
var SeedExpectedBSDTemplateRoles = []string{
	"researcher", "coder", "reviewer", "explorer",
	"planner", "curator", "documenter",
}

// SeedExpectedBSDTemplateToolsetSlugs is the closed set of SUB-005
// references used by builtins.
var SeedExpectedBSDTemplateToolsetSlugs = []string{
	"documentation-readonly-allowlist",
	"code-write-scoped",
	"docs-write-scoped",
}

// SeedExpectedBSDTemplateInheritanceSlugs is the closed set of SUB-006
// references used by builtins.
var SeedExpectedBSDTemplateInheritanceSlugs = []string{
	"extend-parent-rights",
	"restrict-to-readonly",
}

// SeedExpectedBSDTemplateSummarySlugs is the closed set of SUB-010
// references used by builtins.
var SeedExpectedBSDTemplateSummarySlugs = []string{
	"success-with-artifacts",
	"structured-findings",
	"plan-only",
}

// SeedExpectedBSDTemplateTaskClasses is the closed task-class set.
var SeedExpectedBSDTemplateTaskClasses = []string{
	"investigation", "implementation", "review", "exploration",
	"planning", "curation", "documentation",
}

// SeedRecommendedBSDTemplateSlugs — all 7 baselines are recommended.
var SeedRecommendedBSDTemplateSlugs = []string{
	"researcher-baseline", "coder-baseline", "reviewer-baseline",
	"explorer-baseline", "planner-baseline", "curator-baseline",
	"documenter-baseline",
}

// SeedAdminReviewBSDTemplateSlugs — none require admin review by
// default; coder/documenter have write access but stay within scoped
// policies.
var SeedAdminReviewBSDTemplateSlugs = []string{}

// SeedExpectedBSDTemplateRowCount = 7.
const SeedExpectedBSDTemplateRowCount = 7
