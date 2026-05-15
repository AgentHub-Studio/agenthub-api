package core

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePlatformSetting represents a platform-managed default setting.
// Settings are platform-wide tunables every tenant inherits unless they
// explicitly override them.
//
// Inspired by PDF arXiv:2604.14228v1 Section 6.1 (settings is one of 10
// plugin manifest component types) + Claude Code's user/team/org
// settings layering.
type CorePlatformSetting struct {
	ID             uuid.UUID
	Key            string
	Value          string
	ValueType      string // string | number | boolean | json
	Category       string // runner / evaluator / policy / checkpoint / ui / security / compaction
	Description    string
	IsOverridable  bool
	IsActive       bool
	SortOrder      int
}

// AsBool parses Value as a boolean. Returns (false, error) when not a
// boolean setting.
func (s CorePlatformSetting) AsBool() (bool, error) {
	if s.ValueType != "boolean" {
		return false, fmt.Errorf("setting %q is %s, not boolean", s.Key, s.ValueType)
	}
	return strconv.ParseBool(s.Value)
}

// AsFloat parses Value as a float. Returns (0, error) when not a number.
func (s CorePlatformSetting) AsFloat() (float64, error) {
	if s.ValueType != "number" {
		return 0, fmt.Errorf("setting %q is %s, not number", s.Key, s.ValueType)
	}
	return strconv.ParseFloat(s.Value, 64)
}

// AsInt parses Value as an int. Returns (0, error) when not a number.
func (s CorePlatformSetting) AsInt() (int, error) {
	if s.ValueType != "number" {
		return 0, fmt.Errorf("setting %q is %s, not number", s.Key, s.ValueType)
	}
	return strconv.Atoi(s.Value)
}

// CorePlatformSettingLoader loads platform-managed default settings.
// Like other core loaders, non-fatal when schema missing.
type CorePlatformSettingLoader struct {
	pool *pgxpool.Pool
}

// NewCorePlatformSettingLoader creates a CorePlatformSettingLoader.
func NewCorePlatformSettingLoader(pool *pgxpool.Pool) *CorePlatformSettingLoader {
	return &CorePlatformSettingLoader{pool: pool}
}

// LoadAll returns all active settings, ordered by sort_order then key.
func (l *CorePlatformSettingLoader) LoadAll(ctx context.Context) ([]CorePlatformSetting, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, key, value, value_type, category, description,
		       is_overridable, is_active, sort_order
		  FROM ah_core.platform_setting
		 WHERE is_active = true
		 ORDER BY sort_order, key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.platform_setting not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query platform_setting: %w", err)
	}
	defer rows.Close()

	var settings []CorePlatformSetting
	for rows.Next() {
		var s CorePlatformSetting
		if err := rows.Scan(
			&s.ID, &s.Key, &s.Value, &s.ValueType,
			&s.Category, &s.Description,
			&s.IsOverridable, &s.IsActive, &s.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan platform_setting: %w", err)
		}
		settings = append(settings, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate platform_setting: %w", err)
	}
	return settings, nil
}

// FindByKey returns one setting by dotted-path key.
func (l *CorePlatformSettingLoader) FindByKey(ctx context.Context, key string) (CorePlatformSetting, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePlatformSetting{}, false, err
	}
	for _, s := range all {
		if s.Key == key {
			return s, true, nil
		}
	}
	return CorePlatformSetting{}, false, nil
}

// LoadByCategory returns active settings in a specific category.
func (l *CorePlatformSettingLoader) LoadByCategory(ctx context.Context, category string) ([]CorePlatformSetting, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePlatformSetting
	for _, s := range all {
		if s.Category == category {
			matched = append(matched, s)
		}
	}
	return matched, nil
}

// SeedExpectedPlatformSettingKeys is the canonical list of keys the seed
// migration 000013_seed_platform_settings installs.
var SeedExpectedPlatformSettingKeys = []string{
	// runner (4)
	"runner.default_max_iterations",
	"runner.default_max_budget_usd",
	"runner.default_max_depth",
	"runner.elicitation_timeout_ms",
	// evaluator (3)
	"evaluator.default_engine",
	"evaluator.fail_threshold",
	"evaluator.warn_threshold",
	// policy (1)
	"policy.default_engine",
	// checkpoint (2)
	"checkpoint.default_deadline_seconds",
	"checkpoint.cost_threshold_usd",
	// ui (2)
	"ui.default_output_style_slug",
	"ui.show_run_progress",
	// security (2 — non-overridable)
	"security.deny_dangerous_tools",
	"security.audit_retention_days",
	// compaction (1)
	"compaction.context_threshold_pct",
}

// SeedExpectedPlatformSettingCategories is the closed set of categories.
var SeedExpectedPlatformSettingCategories = []string{
	"runner",
	"evaluator",
	"policy",
	"checkpoint",
	"ui",
	"security",
	"compaction",
}

// SeedExpectedPlatformSettingValueTypes is the closed set of value_type values.
var SeedExpectedPlatformSettingValueTypes = []string{
	"string",
	"number",
	"boolean",
	"json",
}

// SeedNonOverridablePlatformSettingKeys lists settings tenants CANNOT
// override (security baselines / global invariants).
var SeedNonOverridablePlatformSettingKeys = []string{
	"security.deny_dangerous_tools",
	"security.audit_retention_days",
}
