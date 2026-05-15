package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePluginSettingsFieldTypeTemplate is a platform-managed preset for one of
// the five §6.1 plugin settings field types.
type CorePluginSettingsFieldTypeTemplate struct {
	ID              uuid.UUID
	Slug            string
	Label           string
	Description     string
	IsMaskable      bool   // true = value hidden in UI/logs (secret only)
	RequiresOptions bool   // true = manifest must provide an options list (select only)
	IsNumeric       bool   // true = value must parse as a number (number only)
	UIWidget        string // frontend widget hint
	SortOrder       int
}

// SeedExpectedPluginSettingsFieldTypeSlugs is the canonical closed set from §6.1.
var SeedExpectedPluginSettingsFieldTypeSlugs = []string{
	"string",
	"boolean",
	"number",
	"select",
	"secret",
}

// SeedExpectedPluginSettingsFieldTypeRowCount matches the migration INSERT count.
const SeedExpectedPluginSettingsFieldTypeRowCount = 5

// SeedPluginSettingsMaskableFieldTypeSlugs are field types whose values must be masked.
var SeedPluginSettingsMaskableFieldTypeSlugs = []string{"secret"}

// SeedPluginSettingsOptionsRequiredFieldTypeSlugs are field types requiring an options list.
var SeedPluginSettingsOptionsRequiredFieldTypeSlugs = []string{"select"}

// SeedPluginSettingsNumericFieldTypeSlugs are field types that store numeric values.
var SeedPluginSettingsNumericFieldTypeSlugs = []string{"number"}

// CorePluginSettingsFieldTypeDefaultTemplateLoader loads field type presets from ah_core.
type CorePluginSettingsFieldTypeDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePluginSettingsFieldTypeDefaultTemplateLoader creates a loader.
func NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool *pgxpool.Pool) *CorePluginSettingsFieldTypeDefaultTemplateLoader {
	return &CorePluginSettingsFieldTypeDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all field types ordered by sort_order.
func (l *CorePluginSettingsFieldTypeDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePluginSettingsFieldTypeTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       is_maskable, requires_options, is_numeric, ui_widget, sort_order
		  FROM ah_core.plugin_settings_field_type_template
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.plugin_settings_field_type_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query plugin_settings_field_type_template: %w", err)
	}
	defer rows.Close()

	var types []CorePluginSettingsFieldTypeTemplate
	for rows.Next() {
		var t CorePluginSettingsFieldTypeTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.IsMaskable, &t.RequiresOptions, &t.IsNumeric, &t.UIWidget, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan plugin_settings_field_type_template: %w", err)
		}
		types = append(types, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate plugin_settings_field_type_template: %w", err)
	}
	return types, nil
}

// FindBySlug returns one field type template by slug.
func (l *CorePluginSettingsFieldTypeDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePluginSettingsFieldTypeTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePluginSettingsFieldTypeTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePluginSettingsFieldTypeTemplate{}, false, nil
}

// LoadMaskable returns field types whose values must be hidden in the UI and excluded from logs.
func (l *CorePluginSettingsFieldTypeDefaultTemplateLoader) LoadMaskable(ctx context.Context) ([]CorePluginSettingsFieldTypeTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePluginSettingsFieldTypeTemplate
	for _, t := range all {
		if t.IsMaskable {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadOptionsRequired returns field types that require a manifest options list.
func (l *CorePluginSettingsFieldTypeDefaultTemplateLoader) LoadOptionsRequired(ctx context.Context) ([]CorePluginSettingsFieldTypeTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePluginSettingsFieldTypeTemplate
	for _, t := range all {
		if t.RequiresOptions {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
