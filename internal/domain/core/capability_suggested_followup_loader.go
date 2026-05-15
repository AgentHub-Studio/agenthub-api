package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilitySuggestedFollowup is a suggested next prompt for a core
// capability agent, loaded from ah_core.capability_suggested_followup.
type CoreCapabilitySuggestedFollowup struct {
	ID           int64
	AgentSlug    string
	FollowupSlug string
	PromptText   string
	DisplayOrder int
	Description  string
	CreatedAt    time.Time
}

// ============================================================
// Seed catalog constants - migration 000125 (2026-05-12).
// ============================================================

// SeedSuggestedFollowupCount is the total number of seeded follow-up prompts:
// 9 rows (3 prompts x 3 agents).
const SeedSuggestedFollowupCount = 9

// SeedSuggestedFollowupAgentCount is the number of agents with seeded prompts.
const SeedSuggestedFollowupAgentCount = 3

// SeedSuggestedFollowupPerAgentCount is the number of prompts seeded per agent.
const SeedSuggestedFollowupPerAgentCount = 3

// Display-order bounds used by migration 000125.
const (
	SeedSuggestedFollowupMinDisplayOrder = 1
	SeedSuggestedFollowupMaxDisplayOrder = 3
)

// Follow-up slug constants for the seeded researcher prompts.
const (
	SeedResearcherFollowupCurrentSources = "research-current-sources"
	SeedResearcherFollowupCompareRecent  = "compare-official-and-recent"
	SeedResearcherFollowupFactCheckClaim = "fact-check-claim"
)

// Follow-up slug constants for the seeded analyst prompts.
const (
	SeedAnalystFollowupExtractPatterns     = "extract-key-patterns"
	SeedAnalystFollowupCompareOptions      = "compare-options"
	SeedAnalystFollowupFindingsIntoActions = "turn-findings-into-actions"
)

// Follow-up slug constants for the seeded planner prompts.
const (
	SeedPlannerFollowupBreakIntoMilestones = "break-into-milestones"
	SeedPlannerFollowupRisksAndOwners      = "identify-risks-and-owners"
	SeedPlannerFollowupWeeklyChecklist     = "weekly-checklist"
)

// Semantic prompt constants used by tests and callers that need canonical text.
const (
	SeedResearcherFactCheckPrompt   = "Fact-check this claim, list supporting and conflicting evidence, and include citations."
	SeedAnalystCompareOptionsPrompt = "Compare these options with trade-offs, confidence levels, and a recommended choice."
	SeedPlannerMilestonesPrompt     = "Break this goal into sequenced milestones with dependencies and acceptance criteria."
)

// CoreCapabilitySuggestedFollowupLoader loads suggested follow-up prompts from
// ah_core.capability_suggested_followup. All methods are non-fatal when the
// schema or table is missing, allowing fresh deployments to start before this
// seed migration is applied.
type CoreCapabilitySuggestedFollowupLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilitySuggestedFollowupLoader creates a loader backed by pool.
func NewCoreCapabilitySuggestedFollowupLoader(pool *pgxpool.Pool) *CoreCapabilitySuggestedFollowupLoader {
	return &CoreCapabilitySuggestedFollowupLoader{pool: pool}
}

// LoadAllSuggestedFollowups returns every seeded suggested follow-up prompt,
// ordered by agent_slug, display_order, followup_slug.
func (l *CoreCapabilitySuggestedFollowupLoader) LoadAllSuggestedFollowups(ctx context.Context) ([]CoreCapabilitySuggestedFollowup, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, followup_slug, prompt_text, display_order, description, created_at
		  FROM ah_core.capability_suggested_followup
		 ORDER BY agent_slug, display_order, followup_slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_suggested_followup not accessible, suggested followups unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_suggested_followup: %w", err)
	}
	defer rows.Close()

	followups, err := scanSuggestedFollowupRows(rows)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_suggested_followup not accessible during iteration, suggested followups unavailable", "err", err)
			return nil, nil
		}
		return nil, err
	}
	return followups, nil
}

// LoadSuggestedFollowupsForAgent returns suggested follow-up prompts for one
// agent, ordered by display_order and followup_slug.
func (l *CoreCapabilitySuggestedFollowupLoader) LoadSuggestedFollowupsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilitySuggestedFollowup, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, followup_slug, prompt_text, display_order, description, created_at
		  FROM ah_core.capability_suggested_followup
		 WHERE agent_slug = $1
		 ORDER BY display_order, followup_slug`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_suggested_followup not accessible, agent suggested followups unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_suggested_followup for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	followups, err := scanSuggestedFollowupRows(rows)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_suggested_followup not accessible during iteration, agent suggested followups unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, err
	}
	return followups, nil
}

// GetSuggestedFollowup returns a single suggested follow-up prompt by
// (agentSlug, followupSlug). It returns (nil, false, nil) when the row is absent
// or when the table does not exist yet.
func (l *CoreCapabilitySuggestedFollowupLoader) GetSuggestedFollowup(ctx context.Context, agentSlug, followupSlug string) (*CoreCapabilitySuggestedFollowup, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, followup_slug, prompt_text, display_order, description, created_at
		  FROM ah_core.capability_suggested_followup
		 WHERE agent_slug = $1
		   AND followup_slug = $2
		 LIMIT 1`

	var f CoreCapabilitySuggestedFollowup
	err = conn.QueryRow(ctx, query, agentSlug, followupSlug).Scan(
		&f.ID,
		&f.AgentSlug,
		&f.FollowupSlug,
		&f.PromptText,
		&f.DisplayOrder,
		&f.Description,
		&f.CreatedAt,
	)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_suggested_followup not accessible, suggested followup unavailable",
				"agent_slug", agentSlug, "followup_slug", followupSlug, "err", err)
			return nil, false, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("core: get suggested followup for agent %q slug %q: %w", agentSlug, followupSlug, err)
	}
	return &f, true, nil
}

func scanSuggestedFollowupRows(rows pgx.Rows) ([]CoreCapabilitySuggestedFollowup, error) {
	var followups []CoreCapabilitySuggestedFollowup
	for rows.Next() {
		var f CoreCapabilitySuggestedFollowup
		if err := rows.Scan(
			&f.ID,
			&f.AgentSlug,
			&f.FollowupSlug,
			&f.PromptText,
			&f.DisplayOrder,
			&f.Description,
			&f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_suggested_followup: %w", err)
		}
		followups = append(followups, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("core: iterate capability_suggested_followup: %w", err)
	}
	return followups, nil
}
