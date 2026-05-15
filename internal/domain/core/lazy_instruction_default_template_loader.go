package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreLazyInstructionDefaultTemplate is a platform-managed blueprint
// paired with CTX-005 LazyInstructionLoader. Each row encodes a TTL +
// source-kind profile tenants pick instead of inventing thresholds.
type CoreLazyInstructionDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	SourceKind               string
	TTLSeconds               int
	TargetUseCase            string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// TTLDuration returns TTLSeconds as a Duration.
func (t CoreLazyInstructionDefaultTemplate) TTLDuration() time.Duration {
	return time.Duration(t.TTLSeconds) * time.Second
}

// CoreLazyInstructionDefaultTemplateLoader loads lazy-instruction templates.
type CoreLazyInstructionDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreLazyInstructionDefaultTemplateLoader creates the loader.
func NewCoreLazyInstructionDefaultTemplateLoader(pool *pgxpool.Pool) *CoreLazyInstructionDefaultTemplateLoader {
	return &CoreLazyInstructionDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreLazyInstructionDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreLazyInstructionDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, source_kind, ttl_seconds,
		       target_use_case, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.lazy_instruction_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.lazy_instruction_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query lazy_instruction_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreLazyInstructionDefaultTemplate
	for rows.Next() {
		var t CoreLazyInstructionDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.SourceKind, &t.TTLSeconds,
			&t.TargetUseCase, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan lazy_instruction_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate lazy_instruction_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreLazyInstructionDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreLazyInstructionDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreLazyInstructionDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreLazyInstructionDefaultTemplate{}, false, nil
}

// LoadBySourceKind filters by source kind.
func (l *CoreLazyInstructionDefaultTemplateLoader) LoadBySourceKind(ctx context.Context, kind string) ([]CoreLazyInstructionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreLazyInstructionDefaultTemplate
	for _, t := range all {
		if t.SourceKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreLazyInstructionDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreLazyInstructionDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreLazyInstructionDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedLIDTemplateSlugs is the closed canonical set.
var SeedExpectedLIDTemplateSlugs = []string{
	"stable-rule-catalog",
	"tenant-config-medium-ttl",
	"external-kb-short-ttl",
	"regulated-policy-strict-ttl",
	"user-memory-session-ttl",
	"dev-debug-no-cache",
}

// SeedExpectedLIDTemplateSourceKinds is the closed source-kind set.
var SeedExpectedLIDTemplateSourceKinds = []string{
	"ah_core_seed", "tenant_db", "external_http",
	"compliance_store", "memory_hierarchy",
}

// SeedExpectedLIDTemplateUseCases is the closed use-case set.
var SeedExpectedLIDTemplateUseCases = []string{
	"rule_lookup", "config_lookup", "kb_lookup",
	"policy_lookup", "user_lookup",
}

// SeedExpectedLIDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedLIDTemplateTenantKinds = []string{
	"general", "regulated", "dev_local",
}

// SeedRecommendedLIDTemplateSlugs is the safe one-click subset.
var SeedRecommendedLIDTemplateSlugs = []string{
	"stable-rule-catalog",
	"tenant-config-medium-ttl",
	"external-kb-short-ttl",
	"regulated-policy-strict-ttl",
	"user-memory-session-ttl",
}

// SeedAdminReviewLIDTemplateSlugs is the admin-review set.
var SeedAdminReviewLIDTemplateSlugs = []string{
	"regulated-policy-strict-ttl",
}

// SeedExpectedLIDTemplateRowCount = 6.
const SeedExpectedLIDTemplateRowCount = 6
