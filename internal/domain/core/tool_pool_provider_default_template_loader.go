package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreToolPoolProviderDefaultTemplate is a platform-managed blueprint
// paired with TOOL-003 tool pool assembly. Each template describes one
// provider configuration tenants can register so fresh installs have a
// working pool without invented semantics.
type CoreToolPoolProviderDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetSource             string
	TargetUseCase            string
	ExposesToolNamesJSON     string
	DefaultPriority          int
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// ExposesToolNames parses the JSONB array.
func (t CoreToolPoolProviderDefaultTemplate) ExposesToolNames() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.ExposesToolNamesJSON), &out); err != nil {
		return nil, fmt.Errorf("tool_pool_provider_template %s: invalid exposes_tool_names: %w", t.Slug, err)
	}
	return out, nil
}

// CoreToolPoolProviderDefaultTemplateLoader loads provider templates.
type CoreToolPoolProviderDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreToolPoolProviderDefaultTemplateLoader creates the loader.
func NewCoreToolPoolProviderDefaultTemplateLoader(pool *pgxpool.Pool) *CoreToolPoolProviderDefaultTemplateLoader {
	return &CoreToolPoolProviderDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreToolPoolProviderDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreToolPoolProviderDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_source, target_use_case,
		       exposes_tool_names::text, default_priority,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.tool_pool_provider_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.tool_pool_provider_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query tool_pool_provider_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreToolPoolProviderDefaultTemplate
	for rows.Next() {
		var t CoreToolPoolProviderDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetSource, &t.TargetUseCase,
			&t.ExposesToolNamesJSON, &t.DefaultPriority,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan tool_pool_provider_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate tool_pool_provider_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreToolPoolProviderDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreToolPoolProviderDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreToolPoolProviderDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreToolPoolProviderDefaultTemplate{}, false, nil
}

// LoadBySource filters by TOOL-003 ToolSource label.
func (l *CoreToolPoolProviderDefaultTemplateLoader) LoadBySource(ctx context.Context, source string) ([]CoreToolPoolProviderDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolPoolProviderDefaultTemplate
	for _, t := range all {
		if t.TargetSource == source {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by template use case.
func (l *CoreToolPoolProviderDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreToolPoolProviderDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolPoolProviderDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreToolPoolProviderDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreToolPoolProviderDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolPoolProviderDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedTPPDTemplateSlugs is the closed canonical set.
var SeedExpectedTPPDTemplateSlugs = []string{
	"builtin-readonly-core",
	"builtin-mutating-core",
	"skill-document-search",
	"mcp-filesystem-default",
	"subagent-readonly-allowlist",
	"extension-platform-utilities",
}

// SeedExpectedTPPDTemplateSources matches TOOL-003 ToolSource enum
// byte-for-byte (5 sources: builtin/skill/mcp/subagent/extension).
var SeedExpectedTPPDTemplateSources = []string{
	"builtin", "skill", "mcp", "subagent", "extension",
}

// SeedExpectedTPPDTemplateUseCases is the closed use-case set.
var SeedExpectedTPPDTemplateUseCases = []string{
	"inspection", "mutation", "rag_search", "external_integration",
	"delegation_safety", "observability",
}

// SeedExpectedTPPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedTPPDTemplateTenantKinds = []string{"general"}

// SeedRecommendedTPPDTemplateSlugs is the safe one-click subset.
// All 6 are recommended because each demonstrates a documented
// TOOL-003 source pattern.
var SeedRecommendedTPPDTemplateSlugs = []string{
	"builtin-readonly-core",
	"builtin-mutating-core",
	"skill-document-search",
	"mcp-filesystem-default",
	"subagent-readonly-allowlist",
	"extension-platform-utilities",
}

// SeedAdminReviewTPPDTemplateSlugs is the closed admin-review set
// (mutating builtins gate Bash/Edit/Write — destructive surface).
var SeedAdminReviewTPPDTemplateSlugs = []string{
	"builtin-mutating-core",
}

// SeedExpectedTPPDTemplateRowCount = 6.
const SeedExpectedTPPDTemplateRowCount = 6
