package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreRelationshipMilestoneTemplate is a platform-managed blueprint for
// FUTURE-002 user-agent relationship state transitions. Each template
// describes a milestone (trust-level promotion or event-recording
// automation) tenants can opt into.
type CoreRelationshipMilestoneTemplate struct {
	ID                   uuid.UUID
	Slug                 string
	Name                 string
	Description          string
	TargetTrustLevel     string
	MinInteractionCount  int
	MinRapportScore      float64
	TriggersEventKinds   string
	OnReachAction        string
	RequiresAdminReview  bool
	IsRecommended        bool
	IsActive             bool
	SortOrder            int
}

// TriggersEventKindsList parses the comma-separated event-kinds column.
func (t CoreRelationshipMilestoneTemplate) TriggersEventKindsList() []string {
	if strings.TrimSpace(t.TriggersEventKinds) == "" {
		return nil
	}
	parts := strings.Split(t.TriggersEventKinds, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CoreRelationshipMilestoneTemplateLoader loads relationship-milestone templates.
type CoreRelationshipMilestoneTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreRelationshipMilestoneTemplateLoader creates the loader.
func NewCoreRelationshipMilestoneTemplateLoader(pool *pgxpool.Pool) *CoreRelationshipMilestoneTemplateLoader {
	return &CoreRelationshipMilestoneTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreRelationshipMilestoneTemplateLoader) LoadAll(ctx context.Context) ([]CoreRelationshipMilestoneTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_trust_level,
		       min_interaction_count, min_rapport_score, triggers_event_kinds,
		       on_reach_action, requires_admin_review, is_recommended,
		       is_active, sort_order
		  FROM ah_core.relationship_milestone_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.relationship_milestone_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query relationship_milestone_template: %w", err)
	}
	defer rows.Close()

	var out []CoreRelationshipMilestoneTemplate
	for rows.Next() {
		var t CoreRelationshipMilestoneTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetTrustLevel,
			&t.MinInteractionCount, &t.MinRapportScore, &t.TriggersEventKinds,
			&t.OnReachAction, &t.RequiresAdminReview, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan relationship_milestone_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate relationship_milestone_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template by slug.
func (l *CoreRelationshipMilestoneTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreRelationshipMilestoneTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreRelationshipMilestoneTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreRelationshipMilestoneTemplate{}, false, nil
}

// LoadByTargetTrustLevel filters by target_trust_level.
func (l *CoreRelationshipMilestoneTemplateLoader) LoadByTargetTrustLevel(ctx context.Context, level string) ([]CoreRelationshipMilestoneTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreRelationshipMilestoneTemplate
	for _, t := range all {
		if t.TargetTrustLevel == level {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByOnReachAction filters by on_reach_action.
func (l *CoreRelationshipMilestoneTemplateLoader) LoadByOnReachAction(ctx context.Context, action string) ([]CoreRelationshipMilestoneTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreRelationshipMilestoneTemplate
	for _, t := range all {
		if t.OnReachAction == action {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreRelationshipMilestoneTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreRelationshipMilestoneTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreRelationshipMilestoneTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedRelMilestoneTemplateSlugs is the closed canonical set.
var SeedExpectedRelMilestoneTemplateSlugs = []string{
	"first-interaction-probationary",
	"established-after-ten-positive",
	"trusted-after-fifty-strong-rapport",
	"mistrusted-on-repeated-escalation",
	"record-positive-acknowledgements",
	"record-conflict-resolutions",
	"record-negative-and-alert",
	"record-escalation-and-open-ticket",
}

// SeedExpectedRelMilestoneTemplateTrustLevels is the closed set of
// target_trust_level values used in the seed (matches FUTURE-002
// TrustLevel enum minus "unknown" — never a target since users start
// there by default).
var SeedExpectedRelMilestoneTemplateTrustLevels = []string{
	"probationary", "established", "trusted", "mistrusted",
}

// SeedExpectedRelMilestoneTemplateEventKinds is the closed FUTURE-002
// RapportEvent set referenced in any template's triggers_event_kinds.
var SeedExpectedRelMilestoneTemplateEventKinds = []string{
	"positive", "neutral", "negative", "conflict_resolved", "escalation",
}

// SeedExpectedRelMilestoneTemplateActions is the closed set of
// on_reach_action values.
var SeedExpectedRelMilestoneTemplateActions = []string{
	"no_action", "elevate_communication_style",
	"send_admin_notification", "open_review_ticket",
}

// SeedRecommendedRelMilestoneTemplateSlugs lists the safe one-click
// defaults — the entire set is recommended because every milestone
// follows FUTURE-002 documented promotion rules byte-for-byte.
var SeedRecommendedRelMilestoneTemplateSlugs = []string{
	"first-interaction-probationary",
	"established-after-ten-positive",
	"trusted-after-fifty-strong-rapport",
	"mistrusted-on-repeated-escalation",
	"record-positive-acknowledgements",
	"record-conflict-resolutions",
	"record-negative-and-alert",
	"record-escalation-and-open-ticket",
}

// SeedAdminReviewRelMilestoneTemplateSlugs is the closed admin-review set.
var SeedAdminReviewRelMilestoneTemplateSlugs = []string{
	"trusted-after-fifty-strong-rapport",
	"mistrusted-on-repeated-escalation",
	"record-escalation-and-open-ticket",
}

// SeedExpectedRelMilestoneTemplateRowCount = 8.
const SeedExpectedRelMilestoneTemplateRowCount = 8
