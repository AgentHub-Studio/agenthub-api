package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePathScopedRuleDefaultTemplate is a platform-managed blueprint
// paired with CTX-004 PathScopedRuleRegistry. Each row provides a
// canonical (scope, glob, content, priority) pattern fresh tenants
// instantiate without inventing globs.
type CorePathScopedRuleDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetScope              string
	PathGlob                 string
	RuleContent              string
	DefaultPriority          int
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CorePathScopedRuleDefaultTemplateLoader loads path-rule default templates.
type CorePathScopedRuleDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePathScopedRuleDefaultTemplateLoader creates the loader.
func NewCorePathScopedRuleDefaultTemplateLoader(pool *pgxpool.Pool) *CorePathScopedRuleDefaultTemplateLoader {
	return &CorePathScopedRuleDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePathScopedRuleDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePathScopedRuleDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_scope, path_glob,
		       rule_content, default_priority, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.path_scoped_rule_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.path_scoped_rule_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query path_scoped_rule_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePathScopedRuleDefaultTemplate
	for rows.Next() {
		var t CorePathScopedRuleDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetScope, &t.PathGlob,
			&t.RuleContent, &t.DefaultPriority, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan path_scoped_rule_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate path_scoped_rule_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePathScopedRuleDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePathScopedRuleDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePathScopedRuleDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePathScopedRuleDefaultTemplate{}, false, nil
}

// LoadByScope filters by CTX-004 scope label.
func (l *CorePathScopedRuleDefaultTemplateLoader) LoadByScope(ctx context.Context, scope string) ([]CorePathScopedRuleDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePathScopedRuleDefaultTemplate
	for _, t := range all {
		if t.TargetScope == scope {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePathScopedRuleDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePathScopedRuleDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePathScopedRuleDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPSRDTemplateSlugs is the closed canonical set.
var SeedExpectedPSRDTemplateSlugs = []string{
	"no-secrets-in-go-files",
	"no-pii-in-yaml-config",
	"docs-must-have-frontmatter",
	"execute-sql-prepared-statements",
	"shell-commands-no-rm-rf",
	"migrations-no-data-loss",
	"researcher-must-cite-sources",
	"global-no-prompt-injection",
}

// SeedExpectedPSRDTemplateScopes is the closed scope set (mirrors CTX-004).
var SeedExpectedPSRDTemplateScopes = []string{
	"global", "tool", "file", "directory", "agent",
}

// SeedExpectedPSRDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedPSRDTemplateTenantKinds = []string{
	"general", "regulated",
}

// SeedRecommendedPSRDTemplateSlugs is the safe one-click subset.
// All 8 are recommended because each implements a documented pattern.
var SeedRecommendedPSRDTemplateSlugs = []string{
	"no-secrets-in-go-files",
	"no-pii-in-yaml-config",
	"docs-must-have-frontmatter",
	"execute-sql-prepared-statements",
	"shell-commands-no-rm-rf",
	"migrations-no-data-loss",
	"researcher-must-cite-sources",
	"global-no-prompt-injection",
}

// SeedAdminReviewPSRDTemplateSlugs is the admin-review set: rules with
// regulated impact (PII), destructive impact (rm -rf, data loss).
var SeedAdminReviewPSRDTemplateSlugs = []string{
	"no-pii-in-yaml-config",
	"shell-commands-no-rm-rf",
	"migrations-no-data-loss",
}

// SeedExpectedPSRDTemplateRowCount = 8.
const SeedExpectedPSRDTemplateRowCount = 8
