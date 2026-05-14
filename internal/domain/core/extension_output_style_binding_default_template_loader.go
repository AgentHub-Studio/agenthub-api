package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreExtensionOutputStyleBindingDefaultTemplate is a platform-managed
// blueprint paired with EXT-009 ExtensionOutputStyleRegistry. Each row
// encodes a (scope + format + style_slug) preset.
type CoreExtensionOutputStyleBindingDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetScope              string
	TargetFormat             string
	StyleSlug                string
	DefaultPriority          int
	TargetUseCase            string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreExtensionOutputStyleBindingDefaultTemplateLoader loads binding templates.
type CoreExtensionOutputStyleBindingDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreExtensionOutputStyleBindingDefaultTemplateLoader creates the loader.
func NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool *pgxpool.Pool) *CoreExtensionOutputStyleBindingDefaultTemplateLoader {
	return &CoreExtensionOutputStyleBindingDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreExtensionOutputStyleBindingDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreExtensionOutputStyleBindingDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_scope, target_format, style_slug,
		       default_priority, target_use_case, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.extension_output_style_binding_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.extension_output_style_binding_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query extension_output_style_binding_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreExtensionOutputStyleBindingDefaultTemplate
	for rows.Next() {
		var t CoreExtensionOutputStyleBindingDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetScope, &t.TargetFormat, &t.StyleSlug,
			&t.DefaultPriority, &t.TargetUseCase, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan extension_output_style_binding_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate extension_output_style_binding_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreExtensionOutputStyleBindingDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreExtensionOutputStyleBindingDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreExtensionOutputStyleBindingDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreExtensionOutputStyleBindingDefaultTemplate{}, false, nil
}

// LoadByScope filters by EXT-009 scope label.
func (l *CoreExtensionOutputStyleBindingDefaultTemplateLoader) LoadByScope(ctx context.Context, scope string) ([]CoreExtensionOutputStyleBindingDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionOutputStyleBindingDefaultTemplate
	for _, t := range all {
		if t.TargetScope == scope {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByFormat filters by EXT-009 format label.
func (l *CoreExtensionOutputStyleBindingDefaultTemplateLoader) LoadByFormat(ctx context.Context, format string) ([]CoreExtensionOutputStyleBindingDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionOutputStyleBindingDefaultTemplate
	for _, t := range all {
		if t.TargetFormat == format {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreExtensionOutputStyleBindingDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreExtensionOutputStyleBindingDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionOutputStyleBindingDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedEOSBDTemplateSlugs is the closed canonical set.
var SeedExpectedEOSBDTemplateSlugs = []string{
	"platform-conversational-default",
	"tenant-technical-default",
	"agent-extractor-json",
	"explicit-debug-verbose",
	"tenant-html-sanitized-ui",
	"platform-plain-fallback",
}

// SeedExpectedEOSBDTemplateScopes matches EXT-009 OutputStyleScope enum.
var SeedExpectedEOSBDTemplateScopes = []string{
	"explicit", "agent", "tenant", "platform",
}

// SeedExpectedEOSBDTemplateFormats matches EXT-009 OutputStyleFormat enum.
var SeedExpectedEOSBDTemplateFormats = []string{
	"markdown", "json", "plain", "html_sanitized",
}

// SeedExpectedEOSBDTemplateUseCases is the closed use-case set.
var SeedExpectedEOSBDTemplateUseCases = []string{
	"general", "engineering", "data_extraction",
	"debugging", "user_facing_ui", "fallback",
}

// SeedExpectedEOSBDTemplateTenantKinds is the closed audience set.
var SeedExpectedEOSBDTemplateTenantKinds = []string{
	"general",
}

// SeedRecommendedEOSBDTemplateSlugs lists the safe one-click subset.
var SeedRecommendedEOSBDTemplateSlugs = []string{
	"platform-conversational-default",
	"tenant-technical-default",
	"agent-extractor-json",
	"explicit-debug-verbose",
	"tenant-html-sanitized-ui",
	"platform-plain-fallback",
}

// SeedAdminReviewEOSBDTemplateSlugs is the admin-review set.
var SeedAdminReviewEOSBDTemplateSlugs = []string{
	"tenant-html-sanitized-ui",
}

// SeedExpectedEOSBDTemplateRowCount = 6.
const SeedExpectedEOSBDTemplateRowCount = 6
