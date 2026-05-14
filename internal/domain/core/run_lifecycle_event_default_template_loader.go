package core

import (
	"context"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunLifecycleEventDefaultTemplate is one row from
// ah_core.run_lifecycle_event_template.
type RunLifecycleEventDefaultTemplate struct {
	ID               string
	Slug             string
	HookEvent        string
	HandlerKind      string
	Description      string
	HandlerConfig    map[string]any
	EnabledByDefault bool
	SortOrder        int
}

// Seed-time canonical constants.

const SeedExpectedRLETRowCount = 6

var SeedExpectedRLETSlugs = []string{
	"run-started-audit-log",
	"run-complete-webhook-notify",
	"run-complete-notification-slack",
	"context-window-alert-compact",
	"knowledge-base-queried-audit",
	"knowledge-base-queried-webhook",
}

// SeedRLETHookEvents is the closed set of hook_events present in the seed.
var SeedRLETHookEvents = []string{
	"RunStarted",
	"RunComplete",
	"ContextWindowAlert",
	"KnowledgeBaseQueried",
}

// SeedRLETHandlerKinds is the closed set of handler_kinds present in the seed.
var SeedRLETHandlerKinds = []string{
	"webhook",
	"notification",
	"audit",
}

// SeedRLETEnabledByDefaultSlugs are the three templates enabled for fresh tenants.
var SeedRLETEnabledByDefaultSlugs = []string{
	"run-started-audit-log",
	"context-window-alert-compact",
	"knowledge-base-queried-audit",
}

// RLETSlugRE is the kebab-case regex every slug must satisfy.
var RLETSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// CoreRunLifecycleEventDefaultTemplateLoader loads templates from ah_core.
type CoreRunLifecycleEventDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreRunLifecycleEventDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreRunLifecycleEventDefaultTemplateLoader(pool *pgxpool.Pool) *CoreRunLifecycleEventDefaultTemplateLoader {
	return &CoreRunLifecycleEventDefaultTemplateLoader{pool: pool}
}

const rletLoadAllQuery = `
SELECT id, slug, hook_event, handler_kind, description,
       handler_config, enabled_by_default, sort_order
FROM ah_core.run_lifecycle_event_template
ORDER BY sort_order ASC, slug ASC
`

// LoadAll returns every template ordered by sort_order ASC, slug ASC.
// Returns an empty slice (not an error) when the table does not exist.
func (l *CoreRunLifecycleEventDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*RunLifecycleEventDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, rletLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*RunLifecycleEventDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*RunLifecycleEventDefaultTemplate
	for rows.Next() {
		t := &RunLifecycleEventDefaultTemplate{}
		if err := rows.Scan(&t.ID, &t.Slug, &t.HookEvent, &t.HandlerKind,
			&t.Description, &t.HandlerConfig, &t.EnabledByDefault, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindBySlug returns the template matching slug, or (nil, false, nil) when not found.
func (l *CoreRunLifecycleEventDefaultTemplateLoader) FindBySlug(
	ctx context.Context, slug string,
) (*RunLifecycleEventDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return nil, false, nil
}

// LoadByHookEvent returns all templates matching the given hook_event value.
func (l *CoreRunLifecycleEventDefaultTemplateLoader) LoadByHookEvent(
	ctx context.Context, hookEvent string,
) ([]*RunLifecycleEventDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*RunLifecycleEventDefaultTemplate
	for _, t := range all {
		if t.HookEvent == hookEvent {
			out = append(out, t)
		}
	}
	return out, nil
}

// LoadByHandlerKind returns all templates matching the given handler_kind.
func (l *CoreRunLifecycleEventDefaultTemplateLoader) LoadByHandlerKind(
	ctx context.Context, handlerKind string,
) ([]*RunLifecycleEventDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*RunLifecycleEventDefaultTemplate
	for _, t := range all {
		if t.HandlerKind == handlerKind {
			out = append(out, t)
		}
	}
	return out, nil
}

// LoadEnabledByDefault returns all templates with enabled_by_default = true.
func (l *CoreRunLifecycleEventDefaultTemplateLoader) LoadEnabledByDefault(
	ctx context.Context,
) ([]*RunLifecycleEventDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*RunLifecycleEventDefaultTemplate
	for _, t := range all {
		if t.EnabledByDefault {
			out = append(out, t)
		}
	}
	return out, nil
}
