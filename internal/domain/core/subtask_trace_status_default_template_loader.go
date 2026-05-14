package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreSubtaskTraceStatusDefaultTemplate is a platform-managed blueprint
// paired with OBS-006 SubtaskStatus + SubtaskCompleteData envelope.
// Each template documents the proven event sequence + cost propagation
// posture for one SubtaskStatus outcome.
type CoreSubtaskTraceStatusDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetStatus             string
	TargetUseCase            string
	ExpectedEventSequenceJSON string
	PropagateCostToParent    bool
	EmitErrorEnvelope        bool
	TypicalMinTurns          int
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// ExpectedEventSequence parses the JSONB array of event-type labels
// the trace should emit (matches OBS-001/OBS-006 event names).
func (t CoreSubtaskTraceStatusDefaultTemplate) ExpectedEventSequence() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.ExpectedEventSequenceJSON), &out); err != nil {
		return nil, fmt.Errorf("subtask_trace_status_template %s: invalid expected_event_sequence: %w", t.Slug, err)
	}
	return out, nil
}

// CoreSubtaskTraceStatusDefaultTemplateLoader loads templates.
type CoreSubtaskTraceStatusDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSubtaskTraceStatusDefaultTemplateLoader creates the loader.
func NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool *pgxpool.Pool) *CoreSubtaskTraceStatusDefaultTemplateLoader {
	return &CoreSubtaskTraceStatusDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreSubtaskTraceStatusDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreSubtaskTraceStatusDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_status, target_use_case,
		       expected_event_sequence::text, propagate_cost_to_parent,
		       emit_error_envelope, typical_min_turns,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.subtask_trace_status_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.subtask_trace_status_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query subtask_trace_status_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreSubtaskTraceStatusDefaultTemplate
	for rows.Next() {
		var t CoreSubtaskTraceStatusDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetStatus, &t.TargetUseCase,
			&t.ExpectedEventSequenceJSON, &t.PropagateCostToParent,
			&t.EmitErrorEnvelope, &t.TypicalMinTurns,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan subtask_trace_status_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate subtask_trace_status_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreSubtaskTraceStatusDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreSubtaskTraceStatusDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreSubtaskTraceStatusDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreSubtaskTraceStatusDefaultTemplate{}, false, nil
}

// LoadByStatus filters by OBS-006 SubtaskStatus.
func (l *CoreSubtaskTraceStatusDefaultTemplateLoader) LoadByStatus(ctx context.Context, status string) ([]CoreSubtaskTraceStatusDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubtaskTraceStatusDefaultTemplate
	for _, t := range all {
		if t.TargetStatus == status {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case.
func (l *CoreSubtaskTraceStatusDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreSubtaskTraceStatusDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubtaskTraceStatusDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreSubtaskTraceStatusDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreSubtaskTraceStatusDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubtaskTraceStatusDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedSTSDTemplateSlugs is the closed canonical set.
var SeedExpectedSTSDTemplateSlugs = []string{
	"completed-routine-trace",
	"failed-error-context-trace",
	"killed-budget-or-depth-trace",
}

// SeedExpectedSTSDTemplateStatuses matches OBS-006 SubtaskStatus enum
// byte-for-byte (3 statuses).
var SeedExpectedSTSDTemplateStatuses = []string{
	"completed", "failed", "killed",
}

// SeedExpectedSTSDTemplateUseCases is the closed use-case set.
var SeedExpectedSTSDTemplateUseCases = []string{
	"routine_completion", "error_diagnosis", "harness_kill",
}

// SeedExpectedSTSDTemplateEventTypes lists the canonical event types
// referenced across templates (matches AgentHub event envelope names).
var SeedExpectedSTSDTemplateEventTypes = []string{
	"subtask_start", "subtask_complete",
	"text_delta", "tool_call_start", "tool_result", "error",
}

// SeedExpectedSTSDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedSTSDTemplateTenantKinds = []string{"general"}

// SeedRecommendedSTSDTemplateSlugs — all 3 are recommended.
var SeedRecommendedSTSDTemplateSlugs = []string{
	"completed-routine-trace",
	"failed-error-context-trace",
	"killed-budget-or-depth-trace",
}

// SeedAdminReviewSTSDTemplateSlugs — failed + killed require admin
// review (signal misconfiguration or runaway behavior).
var SeedAdminReviewSTSDTemplateSlugs = []string{
	"failed-error-context-trace",
	"killed-budget-or-depth-trace",
}

// SeedExpectedSTSDTemplateRowCount = 3.
const SeedExpectedSTSDTemplateRowCount = 3
