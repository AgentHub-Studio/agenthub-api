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

// CoreKairosHeartbeatStrategyTemplate represents a platform-managed proactive
// agent scheduling preset. Inspired by §11.6 KAIROS (arXiv:2604.14228v1):
// tick-based heartbeats, SleepTool economic throttling, terminal focus awareness.
type CoreKairosHeartbeatStrategyTemplate struct {
	ID                           uuid.UUID
	Slug                         string
	Label                        string
	Description                  string
	ScheduleType                 string // on-demand / heartbeat / cron
	TickIntervalSeconds          int
	PresenceWindowSeconds        int
	MaxActTicksBeforeSleep       int
	SleepDurationSeconds         int
	EconomicBudgetUSDPerSession  float64
	IsProactive                  bool
	IsBackground                 bool
	RecommendedFor               []string
	SourceKairosPattern          string
	SortOrder                    int
}

// SeedExpectedKairosHeartbeatStrategySlugs is the canonical closed set.
var SeedExpectedKairosHeartbeatStrategySlugs = []string{
	"on-demand",
	"heartbeat-5m",
	"heartbeat-15m",
	"heartbeat-hourly",
	"daily-digest",
	"weekly-report",
}

// SeedExpectedKairosHeartbeatStrategyRowCount matches the migration INSERT count.
const SeedExpectedKairosHeartbeatStrategyRowCount = 6

// SeedKairosScheduleTypes is the closed set of schedule_type values.
var SeedKairosScheduleTypes = []string{"on-demand", "heartbeat", "cron"}

// SeedKairosDefaultSlug is the preset returned to agents with no custom config.
const SeedKairosDefaultSlug = "on-demand"

// SeedKairosMinimumHeartbeatSlug is the 5-minute KAIROS economic minimum.
const SeedKairosMinimumHeartbeatSlug = "heartbeat-5m"

// SeedKairosRecommendedSlug is the balanced default for background agents.
const SeedKairosRecommendedSlug = "heartbeat-15m"

// SeedKairosStrategySlugRE validates kebab-case slug format.
var SeedKairosStrategySlugRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// CoreKairosHeartbeatStrategyDefaultTemplateLoader loads strategy presets from ah_core.
type CoreKairosHeartbeatStrategyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreKairosHeartbeatStrategyDefaultTemplateLoader creates a loader.
func NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreKairosHeartbeatStrategyDefaultTemplateLoader {
	return &CoreKairosHeartbeatStrategyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all strategy templates ordered by sort_order, slug.
func (l *CoreKairosHeartbeatStrategyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreKairosHeartbeatStrategyTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description, schedule_type,
		       tick_interval_seconds, presence_window_seconds,
		       max_act_ticks_before_sleep, sleep_duration_seconds,
		       economic_budget_usd_per_session,
		       is_proactive, is_background,
		       recommended_for, source_kairos_pattern, sort_order
		  FROM ah_core.kairos_heartbeat_strategy_template
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.kairos_heartbeat_strategy_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query kairos_heartbeat_strategy_template: %w", err)
	}
	defer rows.Close()

	var templates []CoreKairosHeartbeatStrategyTemplate
	for rows.Next() {
		var t CoreKairosHeartbeatStrategyTemplate
		var recommendedForRaw []byte
		var budget float64
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description, &t.ScheduleType,
			&t.TickIntervalSeconds, &t.PresenceWindowSeconds,
			&t.MaxActTicksBeforeSleep, &t.SleepDurationSeconds,
			&budget,
			&t.IsProactive, &t.IsBackground,
			&recommendedForRaw, &t.SourceKairosPattern, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan kairos_heartbeat_strategy_template: %w", err)
		}
		t.EconomicBudgetUSDPerSession = budget
		if len(recommendedForRaw) > 0 {
			if err := json.Unmarshal(recommendedForRaw, &t.RecommendedFor); err != nil {
				return nil, fmt.Errorf("core: unmarshal recommended_for: %w", err)
			}
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate kairos_heartbeat_strategy_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CoreKairosHeartbeatStrategyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreKairosHeartbeatStrategyTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreKairosHeartbeatStrategyTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreKairosHeartbeatStrategyTemplate{}, false, nil
}

// LoadByScheduleType returns templates of a given schedule type.
func (l *CoreKairosHeartbeatStrategyDefaultTemplateLoader) LoadByScheduleType(ctx context.Context, scheduleType string) ([]CoreKairosHeartbeatStrategyTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreKairosHeartbeatStrategyTemplate
	for _, t := range all {
		if t.ScheduleType == scheduleType {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadProactive returns all templates where is_proactive = true.
func (l *CoreKairosHeartbeatStrategyDefaultTemplateLoader) LoadProactive(ctx context.Context) ([]CoreKairosHeartbeatStrategyTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreKairosHeartbeatStrategyTemplate
	for _, t := range all {
		if t.IsProactive {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
