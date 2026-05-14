package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePlatformCatalogManifestDefaultTemplate is the persisted manifest
// pointing at one ah_core catalog (paired with CORE-SEED-001).
type CorePlatformCatalogManifestDefaultTemplate struct {
	ID                    uuid.UUID
	Slug                  string
	Kind                  string
	TargetTableSlug       string
	MigrationNumber       int
	LoaderPackage         string
	ExpectedRowCount      int
	RequiresAdminApproval bool
	IsTenantShared        bool
	Description           string
	IsRecommended         bool
	IsActive              bool
	SortOrder             int
}

// CorePlatformCatalogManifestDefaultTemplateLoader loads manifests.
type CorePlatformCatalogManifestDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePlatformCatalogManifestDefaultTemplateLoader creates the loader.
func NewCorePlatformCatalogManifestDefaultTemplateLoader(pool *pgxpool.Pool) *CorePlatformCatalogManifestDefaultTemplateLoader {
	return &CorePlatformCatalogManifestDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active manifests ordered by sort_order then slug.
func (l *CorePlatformCatalogManifestDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePlatformCatalogManifestDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, kind, target_table_slug, migration_number,
		       loader_package, expected_row_count, requires_admin_approval,
		       is_tenant_shared, description, is_recommended,
		       is_active, sort_order
		  FROM ah_core.platform_catalog_manifest_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.platform_catalog_manifest_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query platform_catalog_manifest_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePlatformCatalogManifestDefaultTemplate
	for rows.Next() {
		var t CorePlatformCatalogManifestDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Kind, &t.TargetTableSlug, &t.MigrationNumber,
			&t.LoaderPackage, &t.ExpectedRowCount, &t.RequiresAdminApproval,
			&t.IsTenantShared, &t.Description, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan platform_catalog_manifest_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate platform_catalog_manifest_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one manifest.
func (l *CorePlatformCatalogManifestDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePlatformCatalogManifestDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePlatformCatalogManifestDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePlatformCatalogManifestDefaultTemplate{}, false, nil
}

// LoadByKind filters by CORE-SEED-001 PlatformCatalogKind.
func (l *CorePlatformCatalogManifestDefaultTemplateLoader) LoadByKind(ctx context.Context, kind string) ([]CorePlatformCatalogManifestDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePlatformCatalogManifestDefaultTemplate
	for _, t := range all {
		if t.Kind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPCMDTemplateSlugs is the closed canonical set.
var SeedExpectedPCMDTemplateSlugs = []string{
	"builtin_subagent_default_template",
	"custom_agent_definition_default_template",
	"subagent_toolset_policy_default_template",
	"subagent_inheritance_mode_default_template",
	"subagent_return_summary_default_template",
	"session_fork_strategy_default_template",
	"background_subagent_lane_default_template",
	"context_section_budget_template",
	"permission_audit_retention_default_template",
	"extension_descriptor_default_template",
	"multi_agent_coordination_plan_default_template",
}

// SeedExpectedPCMDTemplateKinds — matches CORE-SEED-001 PlatformCatalogKind
// enum byte-for-byte (11 kinds).
var SeedExpectedPCMDTemplateKinds = []string{
	"subagent_roster", "agent_definition",
	"toolset_policy", "inheritance_mode", "summary_shape",
	"fork_strategy", "background_lane",
	"context_policy", "permission_policy",
	"extension_descriptor", "operational_template",
}

// SeedExpectedPCMDTemplateLoaderPackages — closed loader-package set.
var SeedExpectedPCMDTemplateLoaderPackages = []string{"core"}

// SeedRecommendedPCMDTemplateSlugs — all 11 manifests are recommended.
var SeedRecommendedPCMDTemplateSlugs = SeedExpectedPCMDTemplateSlugs

// SeedExpectedPCMDTemplateRowCount = 11.
const SeedExpectedPCMDTemplateRowCount = 11
