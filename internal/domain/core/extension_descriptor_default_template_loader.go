package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreExtensionDescriptorDefaultTemplate is a platform-managed blueprint
// paired with EXT-001 ExtensionRegistry. Each template captures a
// pre-vetted extension shape (source + version + components +
// checksum/auto-enable posture) tenants can adopt directly.
type CoreExtensionDescriptorDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetSource             string
	TargetVersion            string
	TargetUseCase            string
	SafetyPosture            string
	ComponentsOfferedJSON    string
	ChecksumRequired         bool
	AutoEnableAfterInstall   bool
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// ComponentsOffered parses the JSONB array of ExtensionComponentKind
// labels (matches EXT-001 byte-for-byte).
func (t CoreExtensionDescriptorDefaultTemplate) ComponentsOffered() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.ComponentsOfferedJSON), &out); err != nil {
		return nil, fmt.Errorf("extension_descriptor_template %s: invalid components_offered: %w", t.Slug, err)
	}
	return out, nil
}

// CoreExtensionDescriptorDefaultTemplateLoader loads templates.
type CoreExtensionDescriptorDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreExtensionDescriptorDefaultTemplateLoader creates the loader.
func NewCoreExtensionDescriptorDefaultTemplateLoader(pool *pgxpool.Pool) *CoreExtensionDescriptorDefaultTemplateLoader {
	return &CoreExtensionDescriptorDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreExtensionDescriptorDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreExtensionDescriptorDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_source, target_version, target_use_case,
		       safety_posture, components_offered::text, checksum_required,
		       auto_enable_after_install, recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.extension_descriptor_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.extension_descriptor_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query extension_descriptor_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreExtensionDescriptorDefaultTemplate
	for rows.Next() {
		var t CoreExtensionDescriptorDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetSource, &t.TargetVersion, &t.TargetUseCase,
			&t.SafetyPosture, &t.ComponentsOfferedJSON, &t.ChecksumRequired,
			&t.AutoEnableAfterInstall, &t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan extension_descriptor_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate extension_descriptor_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreExtensionDescriptorDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreExtensionDescriptorDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreExtensionDescriptorDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreExtensionDescriptorDefaultTemplate{}, false, nil
}

// LoadBySource filters by EXT-001 ExtensionSource.
func (l *CoreExtensionDescriptorDefaultTemplateLoader) LoadBySource(ctx context.Context, source string) ([]CoreExtensionDescriptorDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionDescriptorDefaultTemplate
	for _, t := range all {
		if t.TargetSource == source {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case.
func (l *CoreExtensionDescriptorDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreExtensionDescriptorDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionDescriptorDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreExtensionDescriptorDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreExtensionDescriptorDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreExtensionDescriptorDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedEDDTemplateSlugs is the closed canonical set.
var SeedExpectedEDDTemplateSlugs = []string{
	"builtin-essentials",
	"marketplace-rag-pack",
	"marketplace-engineering-pack",
	"git-internal-tools",
	"url-vendor-skills",
}

// SeedExpectedEDDTemplateSources matches EXT-001 ExtensionSource enum
// byte-for-byte.
var SeedExpectedEDDTemplateSources = []string{
	"builtin", "marketplace", "git", "url",
}

// SeedExpectedEDDTemplateUseCases is the closed use-case set.
var SeedExpectedEDDTemplateUseCases = []string{
	"platform_baseline", "rag_search",
	"engineering", "internal_tooling", "vendor_delivery",
}

// SeedExpectedEDDTemplateSafetyPostures is the closed posture set.
var SeedExpectedEDDTemplateSafetyPostures = []string{
	"balanced", "conservative", "strict",
}

// SeedExpectedEDDTemplateComponents matches EXT-001 ExtensionComponentKind
// enum byte-for-byte (8 web-applicable types).
var SeedExpectedEDDTemplateComponents = []string{
	"agents", "tools", "skills", "commands",
	"hooks", "rules", "mcp_servers", "output_styles",
}

// SeedExpectedEDDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedEDDTemplateTenantKinds = []string{"general"}

// SeedRecommendedEDDTemplateSlugs — all 5 are recommended.
var SeedRecommendedEDDTemplateSlugs = []string{
	"builtin-essentials",
	"marketplace-rag-pack",
	"marketplace-engineering-pack",
	"git-internal-tools",
	"url-vendor-skills",
}

// SeedAdminReviewEDDTemplateSlugs — non-marketplace sources + the
// broader-surface engineering pack require admin review.
var SeedAdminReviewEDDTemplateSlugs = []string{
	"marketplace-engineering-pack",
	"git-internal-tools",
	"url-vendor-skills",
}

// SeedExpectedEDDTemplateRowCount = 5.
const SeedExpectedEDDTemplateRowCount = 5
