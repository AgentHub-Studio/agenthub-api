package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreMemoryHierarchyDefaultTemplate is a platform-managed blueprint
// paired with CTX-003 MemoryHierarchy. Each row seeds a baseline fact
// (scope + key + value + max_age) the tenant bootstrap pipeline writes
// into the hierarchy on first run.
type CoreMemoryHierarchyDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	TargetScope              string
	Key                      string
	DefaultValue             string
	MaxAgeSeconds            int
	Description              string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// MaxAge returns the staleness window as a Duration. Zero = never expire.
func (t CoreMemoryHierarchyDefaultTemplate) MaxAge() time.Duration {
	return time.Duration(t.MaxAgeSeconds) * time.Second
}

// CoreMemoryHierarchyDefaultTemplateLoader loads memory-hierarchy default templates.
type CoreMemoryHierarchyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreMemoryHierarchyDefaultTemplateLoader creates the loader.
func NewCoreMemoryHierarchyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreMemoryHierarchyDefaultTemplateLoader {
	return &CoreMemoryHierarchyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreMemoryHierarchyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreMemoryHierarchyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, target_scope, key, default_value, max_age_seconds,
		       description, recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.memory_hierarchy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.memory_hierarchy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query memory_hierarchy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreMemoryHierarchyDefaultTemplate
	for rows.Next() {
		var t CoreMemoryHierarchyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.TargetScope, &t.Key, &t.DefaultValue, &t.MaxAgeSeconds,
			&t.Description, &t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan memory_hierarchy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate memory_hierarchy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreMemoryHierarchyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreMemoryHierarchyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreMemoryHierarchyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreMemoryHierarchyDefaultTemplate{}, false, nil
}

// LoadByScope returns templates targeting a specific CTX-003 scope.
func (l *CoreMemoryHierarchyDefaultTemplateLoader) LoadByScope(ctx context.Context, scope string) ([]CoreMemoryHierarchyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMemoryHierarchyDefaultTemplate
	for _, t := range all {
		if t.TargetScope == scope {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreMemoryHierarchyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreMemoryHierarchyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMemoryHierarchyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedMemoryHierarchyDefaultTemplateSlugs is the closed canonical set.
var SeedExpectedMemoryHierarchyDefaultTemplateSlugs = []string{
	"global-platform-name",
	"global-platform-version",
	"global-default-locale",
	"global-support-email",
	"tenant-default-tone",
	"tenant-default-timezone",
	"tenant-support-window",
	"tenant-compliance-mode",
}

// SeedExpectedMemoryHierarchyDefaultTemplateScopes is the closed scope set
// (subset of CTX-003 MemoryHierarchyScope: only global+tenant make sense
// for fresh-tenant defaults; session/user/agent are subject-bound).
var SeedExpectedMemoryHierarchyDefaultTemplateScopes = []string{
	"global", "tenant",
}

// SeedExpectedMemoryHierarchyDefaultTemplateTenantKinds is the closed
// recommended_for_tenant_kind set.
var SeedExpectedMemoryHierarchyDefaultTemplateTenantKinds = []string{
	"general", "regulated", "dev_local",
}

// SeedRecommendedMemoryHierarchyDefaultTemplateSlugs is the safe one-click
// subset (compliance-mode excluded — admin-review).
var SeedRecommendedMemoryHierarchyDefaultTemplateSlugs = []string{
	"global-platform-name",
	"global-platform-version",
	"global-default-locale",
	"global-support-email",
	"tenant-default-tone",
	"tenant-default-timezone",
	"tenant-support-window",
}

// SeedAdminReviewMemoryHierarchyDefaultTemplateSlugs lists templates
// requiring admin review (compliance-mode has audit implications).
var SeedAdminReviewMemoryHierarchyDefaultTemplateSlugs = []string{
	"tenant-compliance-mode",
}

// SeedExpectedMemoryHierarchyDefaultTemplateRowCount = 8.
const SeedExpectedMemoryHierarchyDefaultTemplateRowCount = 8
