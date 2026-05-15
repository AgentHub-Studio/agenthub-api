package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreBackgroundSubagentLaneDefaultTemplate is an operational lane
// configuration paired with SUB-008 BackgroundSubagentJob. Each lane
// captures timeout/concurrency/watchdog budgets proven for a task
// class so tenants don't tune from scratch.
type CoreBackgroundSubagentLaneDefaultTemplate struct {
	ID                            uuid.UUID
	Slug                          string
	LaneName                      string
	Description                   string
	TypicalTaskClass              string
	TimeoutBudgetSeconds          int
	WatchdogCheckIntervalSeconds  int
	MaxConcurrentPerParent        int
	Priority                      string
	AutoCancelOnParentTerminate   bool
	RetryPosture                  string
	RecommendedForTenantKind      string
	IsRecommended                 bool
	IsActive                      bool
	SortOrder                     int
}

// CoreBackgroundSubagentLaneDefaultTemplateLoader loads templates.
type CoreBackgroundSubagentLaneDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreBackgroundSubagentLaneDefaultTemplateLoader creates the loader.
func NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool *pgxpool.Pool) *CoreBackgroundSubagentLaneDefaultTemplateLoader {
	return &CoreBackgroundSubagentLaneDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreBackgroundSubagentLaneDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreBackgroundSubagentLaneDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, lane_name, description, typical_task_class,
		       timeout_budget_seconds, watchdog_check_interval_seconds,
		       max_concurrent_per_parent, priority,
		       auto_cancel_on_parent_terminate, retry_posture,
		       recommended_for_tenant_kind, is_recommended,
		       is_active, sort_order
		  FROM ah_core.background_subagent_lane_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.background_subagent_lane_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query background_subagent_lane_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreBackgroundSubagentLaneDefaultTemplate
	for rows.Next() {
		var t CoreBackgroundSubagentLaneDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.LaneName, &t.Description, &t.TypicalTaskClass,
			&t.TimeoutBudgetSeconds, &t.WatchdogCheckIntervalSeconds,
			&t.MaxConcurrentPerParent, &t.Priority,
			&t.AutoCancelOnParentTerminate, &t.RetryPosture,
			&t.RecommendedForTenantKind, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan background_subagent_lane_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate background_subagent_lane_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreBackgroundSubagentLaneDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreBackgroundSubagentLaneDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreBackgroundSubagentLaneDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreBackgroundSubagentLaneDefaultTemplate{}, false, nil
}

// LoadByTaskClass filters by SUB-002 task class.
func (l *CoreBackgroundSubagentLaneDefaultTemplateLoader) LoadByTaskClass(ctx context.Context, taskClass string) ([]CoreBackgroundSubagentLaneDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBackgroundSubagentLaneDefaultTemplate
	for _, t := range all {
		if t.TypicalTaskClass == taskClass {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByPriority filters by priority lane.
func (l *CoreBackgroundSubagentLaneDefaultTemplateLoader) LoadByPriority(ctx context.Context, priority string) ([]CoreBackgroundSubagentLaneDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBackgroundSubagentLaneDefaultTemplate
	for _, t := range all {
		if t.Priority == priority {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedBSLDTemplateSlugs is the closed canonical set.
var SeedExpectedBSLDTemplateSlugs = []string{
	"quick-glance",
	"planner-lane",
	"research-lane",
	"coder-lane",
}

// SeedExpectedBSLDTemplateTaskClasses — refs SUB-002 task_class vocabulary.
var SeedExpectedBSLDTemplateTaskClasses = []string{
	"exploration", "planning", "investigation", "implementation",
}

// SeedExpectedBSLDTemplatePriorities — closed priority set.
var SeedExpectedBSLDTemplatePriorities = []string{"high", "normal"}

// SeedExpectedBSLDTemplateRetryPostures — closed retry posture set.
var SeedExpectedBSLDTemplateRetryPostures = []string{"none"}

// SeedRecommendedBSLDTemplateSlugs — all 4 lanes are recommended.
var SeedRecommendedBSLDTemplateSlugs = []string{
	"quick-glance", "planner-lane", "research-lane", "coder-lane",
}

// SeedExpectedBSLDTemplateRowCount = 4.
const SeedExpectedBSLDTemplateRowCount = 4

// Budget bounds reflect operational tuning baked into the seed.
const (
	SeedMinBSLDTimeoutBudgetSeconds = 30
	SeedMaxBSLDTimeoutBudgetSeconds = 600
)

// SeedSingleConcurrencyBSLDTemplateSlugs — lanes that require serial
// execution per parent (planning + writes).
var SeedSingleConcurrencyBSLDTemplateSlugs = []string{
	"planner-lane", "coder-lane",
}
