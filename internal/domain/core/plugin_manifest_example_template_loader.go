package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePluginManifestExampleTemplate is a platform-managed skeleton
// manifest paired with EXT-004 PluginManifest. Each row provides a
// JSON example fresh extension authors copy + customize.
type CorePluginManifestExampleTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	ManifestKind             string
	ExampleManifestJSON      string
	TargetAudience           string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// ParseExample returns the example as a parsed map (validates JSON).
func (t CorePluginManifestExampleTemplate) ParseExample() (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(t.ExampleManifestJSON), &out); err != nil {
		return nil, fmt.Errorf("manifest example %s: invalid JSON: %w", t.Slug, err)
	}
	return out, nil
}

// CorePluginManifestExampleTemplateLoader loads manifest example templates.
type CorePluginManifestExampleTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePluginManifestExampleTemplateLoader creates the loader.
func NewCorePluginManifestExampleTemplateLoader(pool *pgxpool.Pool) *CorePluginManifestExampleTemplateLoader {
	return &CorePluginManifestExampleTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePluginManifestExampleTemplateLoader) LoadAll(ctx context.Context) ([]CorePluginManifestExampleTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, manifest_kind, example_manifest_json,
		       target_audience, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.plugin_manifest_example_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.plugin_manifest_example_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query plugin_manifest_example_template: %w", err)
	}
	defer rows.Close()

	var out []CorePluginManifestExampleTemplate
	for rows.Next() {
		var t CorePluginManifestExampleTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.ManifestKind, &t.ExampleManifestJSON,
			&t.TargetAudience, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan plugin_manifest_example_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate plugin_manifest_example_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePluginManifestExampleTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePluginManifestExampleTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePluginManifestExampleTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePluginManifestExampleTemplate{}, false, nil
}

// LoadByKind filters by EXT-004 manifest kind.
func (l *CorePluginManifestExampleTemplateLoader) LoadByKind(ctx context.Context, kind string) ([]CorePluginManifestExampleTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePluginManifestExampleTemplate
	for _, t := range all {
		if t.ManifestKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePluginManifestExampleTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePluginManifestExampleTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePluginManifestExampleTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPMETemplateSlugs is the closed canonical set (one per kind).
var SeedExpectedPMETemplateSlugs = []string{
	"example-agent-pack",
	"example-skill-pack",
	"example-tool-pack",
	"example-hook-pack",
	"example-rule-pack",
	"example-theme-pack",
	"example-bundle-meta",
}

// SeedExpectedPMETemplateKinds matches EXT-004 PluginManifestKind enum.
var SeedExpectedPMETemplateKinds = []string{
	"agent", "skill_pack", "tool_pack", "hook_pack",
	"rule_pack", "theme_pack", "bundle",
}

// SeedExpectedPMETemplateAudiences is the closed audience set.
var SeedExpectedPMETemplateAudiences = []string{
	"extension_author",
}

// SeedRecommendedPMETemplateSlugs lists the safe one-click subset.
// All 7 are recommended (skeletons are educational; no risk).
var SeedRecommendedPMETemplateSlugs = []string{
	"example-agent-pack",
	"example-skill-pack",
	"example-tool-pack",
	"example-hook-pack",
	"example-rule-pack",
	"example-theme-pack",
	"example-bundle-meta",
}

// SeedExpectedPMETemplateRowCount = 7 (one per kind).
const SeedExpectedPMETemplateRowCount = 7
