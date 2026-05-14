package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreContentReferenceDefaultTemplate is a platform-managed blueprint
// paired with CTX-009 ContentReferenceRegistry. Each row encodes a
// reference policy: which kinds get @ref tokens + size threshold + GC.
type CoreContentReferenceDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	EnabledKinds             string
	MinBytesToReference      int
	IdleGCSeconds            int
	TargetUseCase            string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// EnabledKindsList parses comma-separated CTX-009 kind labels.
func (t CoreContentReferenceDefaultTemplate) EnabledKindsList() []string {
	if strings.TrimSpace(t.EnabledKinds) == "" {
		return nil
	}
	parts := strings.Split(t.EnabledKinds, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// IdleGCDuration returns IdleGCSeconds as a Duration. Zero = never sweep.
func (t CoreContentReferenceDefaultTemplate) IdleGCDuration() time.Duration {
	return time.Duration(t.IdleGCSeconds) * time.Second
}

// CoreContentReferenceDefaultTemplateLoader loads content-reference templates.
type CoreContentReferenceDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreContentReferenceDefaultTemplateLoader creates the loader.
func NewCoreContentReferenceDefaultTemplateLoader(pool *pgxpool.Pool) *CoreContentReferenceDefaultTemplateLoader {
	return &CoreContentReferenceDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreContentReferenceDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreContentReferenceDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, enabled_kinds,
		       min_bytes_to_reference, idle_gc_seconds,
		       target_use_case, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.content_reference_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.content_reference_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query content_reference_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreContentReferenceDefaultTemplate
	for rows.Next() {
		var t CoreContentReferenceDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.EnabledKinds,
			&t.MinBytesToReference, &t.IdleGCSeconds,
			&t.TargetUseCase, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan content_reference_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate content_reference_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreContentReferenceDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreContentReferenceDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreContentReferenceDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreContentReferenceDefaultTemplate{}, false, nil
}

// LoadByUseCase filters by use case.
func (l *CoreContentReferenceDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreContentReferenceDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContentReferenceDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreContentReferenceDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreContentReferenceDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContentReferenceDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedCRDTemplateSlugs is the closed canonical set.
var SeedExpectedCRDTemplateSlugs = []string{
	"balanced-default",
	"research-heavy",
	"kb-heavy-only",
	"code-heavy",
	"cost-strict",
	"dev-debug",
}

// SeedExpectedCRDTemplateUseCases is the closed use-case set.
var SeedExpectedCRDTemplateUseCases = []string{
	"general", "research", "code",
}

// SeedExpectedCRDTemplateTenantKinds is the closed audience set.
var SeedExpectedCRDTemplateTenantKinds = []string{
	"general", "dev_local",
}

// SeedExpectedCRDTemplateKindLabels is the CTX-009 ContentReferenceKind
// label set this seed references.
var SeedExpectedCRDTemplateKindLabels = []string{
	"kb_chunk", "tool_result", "file_blob", "web_fetch", "memory_snapshot",
}

// SeedRecommendedCRDTemplateSlugs is the safe one-click subset
// (excludes dev-debug).
var SeedRecommendedCRDTemplateSlugs = []string{
	"balanced-default",
	"research-heavy",
	"kb-heavy-only",
	"code-heavy",
	"cost-strict",
}

// SeedAdminReviewCRDTemplateSlugs lists templates requiring admin review.
var SeedAdminReviewCRDTemplateSlugs = []string{
	"cost-strict",
}

// SeedExpectedCRDTemplateRowCount = 6.
const SeedExpectedCRDTemplateRowCount = 6
