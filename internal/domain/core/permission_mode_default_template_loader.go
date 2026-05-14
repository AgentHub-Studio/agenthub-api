package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePermissionModeTemplate is a platform-managed permission mode preset.
// §11.3: five modes form a strictly decreasing safety gradient.
type CorePermissionModeTemplate struct {
	ID                   uuid.UUID
	Slug                 string
	Label                string
	Description          string
	GradientIndex        int      // 0=plan (safest) → 4=bypassPermissions (most autonomous)
	SafetyScore          int      // 0–100; strictly decreasing along the gradient
	RequiresConfirmation bool
	AutoAcceptsEdits     bool
	AllowsBackgroundRun  bool
	AllowsToolBypass     bool
	RecommendedFor       []string
	SortOrder            int
}

// SeedExpectedPermissionModeSlugs is the canonical closed set.
// Slugs use camelCase to match Claude Code's PermissionMode.ts conventions.
var SeedExpectedPermissionModeSlugs = []string{
	"plan", "default", "acceptEdits", "auto", "bypassPermissions",
}

// SeedExpectedPermissionModeRowCount matches the migration INSERT count.
const SeedExpectedPermissionModeRowCount = 5

// SeedPermissionModeDefaultSlug is used when no explicit mode is configured.
const SeedPermissionModeDefaultSlug = "default"

// SeedPermissionModeSafestSlug is the maximum-oversight mode.
const SeedPermissionModeSafestSlug = "plan"

// SeedPermissionModeBypassSlug is the most-autonomous mode.
const SeedPermissionModeBypassSlug = "bypassPermissions"

// SeedPermissionModeSlugRE validates slug format (camelCase and kebab-case permitted).
var SeedPermissionModeSlugRE = regexp.MustCompile(`^[a-z][a-zA-Z0-9-]*$`)

// CorePermissionModeDefaultTemplateLoader loads permission mode presets from ah_core.
type CorePermissionModeDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePermissionModeDefaultTemplateLoader creates a loader.
func NewCorePermissionModeDefaultTemplateLoader(pool *pgxpool.Pool) *CorePermissionModeDefaultTemplateLoader {
	return &CorePermissionModeDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all modes ordered by gradient_index, slug.
func (l *CorePermissionModeDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePermissionModeTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       gradient_index, safety_score,
		       requires_confirmation, auto_accepts_edits,
		       allows_background_run, allows_tool_bypass,
		       recommended_for, sort_order
		  FROM ah_core.permission_mode_template
		 ORDER BY gradient_index, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.permission_mode_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query permission_mode_template: %w", err)
	}
	defer rows.Close()

	var modes []CorePermissionModeTemplate
	for rows.Next() {
		var t CorePermissionModeTemplate
		var recommendedForRaw []byte
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.GradientIndex, &t.SafetyScore,
			&t.RequiresConfirmation, &t.AutoAcceptsEdits,
			&t.AllowsBackgroundRun, &t.AllowsToolBypass,
			&recommendedForRaw, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan permission_mode_template: %w", err)
		}
		if len(recommendedForRaw) > 0 {
			if err := json.Unmarshal(recommendedForRaw, &t.RecommendedFor); err != nil {
				return nil, fmt.Errorf("core: unmarshal recommended_for: %w", err)
			}
		}
		modes = append(modes, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate permission_mode_template: %w", err)
	}
	return modes, nil
}

// FindBySlug returns one mode by slug.
func (l *CorePermissionModeDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePermissionModeTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePermissionModeTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePermissionModeTemplate{}, false, nil
}

// LoadBackgroundCapable returns modes where AllowsBackgroundRun is true.
func (l *CorePermissionModeDefaultTemplateLoader) LoadBackgroundCapable(ctx context.Context) ([]CorePermissionModeTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionModeTemplate
	for _, t := range all {
		if t.AllowsBackgroundRun {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadModesRequiringConfirmation returns modes where RequiresConfirmation is true.
func (l *CorePermissionModeDefaultTemplateLoader) LoadModesRequiringConfirmation(ctx context.Context) ([]CorePermissionModeTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionModeTemplate
	for _, t := range all {
		if t.RequiresConfirmation {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
