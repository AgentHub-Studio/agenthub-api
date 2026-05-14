package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentDescription is a single per-agent user-facing description row
// loaded from ah_core.capability_agent_description. Each row encodes one
// description entry for a core capability agent: the desc_key identifies the
// type of description (tagline, long_description, use_cases) and desc_value
// holds the actual content.
type CoreCapabilityAgentDescription struct {
	ID          int64
	AgentSlug   string
	DescKey     string
	DescValue   string
	Description string
	CreatedAt   time.Time
}

// ============================================================
// Seed catalog constants — migration 000124 (2026-05-12).
// ============================================================

// SeedAgentDescriptionCount is the total number of capability_agent_description
// rows seeded by migration 000124: 9 rows (3 description types × 3 agents).
const SeedAgentDescriptionCount = 9

// SeedAgentDescriptionAgentCount is the number of capability agents that have
// seeded description rows in migration 000124 (researcher, analyst, planner).
const SeedAgentDescriptionAgentCount = 3

// Description key constants for well-known capability_agent_description.desc_key values.
const (
	// SeedDescKeyTagline is the desc_key for the short one-line description
	// shown in the agent picker card UI.
	SeedDescKeyTagline = "tagline"

	// SeedDescKeyLongDescription is the desc_key for the full description
	// shown on the agent detail page.
	SeedDescKeyLongDescription = "long_description"

	// SeedDescKeyUseCases is the desc_key for the pipe-separated list of
	// example use cases shown as badges in the frontend UI.
	SeedDescKeyUseCases = "use_cases"
)

// SeedUseCaseSeparator is the pipe character used to separate individual use
// case entries in the use_cases desc_value field. Frontend consumers must split
// on this separator to render individual badge items.
const SeedUseCaseSeparator = "|"

// Per-agent tagline constants reflecting the seeded tagline values from
// migration 000124. Used by tests and callers that need to verify or display
// the canonical tagline without a DB round-trip.
const (
	// SeedResearcherTagline is the seeded tagline for core-researcher.
	SeedResearcherTagline = "Search the web and synthesize findings instantly"

	// SeedAnalystTagline is the seeded tagline for core-analyst.
	SeedAnalystTagline = "Analyze data and documents with structured reasoning"

	// SeedPlannerTagline is the seeded tagline for core-planner.
	SeedPlannerTagline = "Break down complex goals into actionable step-by-step plans"
)

// CoreCapabilityAgentDescriptionLoader loads per-agent user-facing description
// rows from ah_core.capability_agent_description (seeded by migration 000124).
// All methods are non-fatal when the ah_core schema or the
// capability_agent_description table is not yet accessible — supports fresh
// deployments where migration 000124 has not yet run.
type CoreCapabilityAgentDescriptionLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentDescriptionLoader creates a
// CoreCapabilityAgentDescriptionLoader backed by pool.
func NewCoreCapabilityAgentDescriptionLoader(pool *pgxpool.Pool) *CoreCapabilityAgentDescriptionLoader {
	return &CoreCapabilityAgentDescriptionLoader{pool: pool}
}

// LoadCapabilityAgentDescriptions returns all rows from
// ah_core.capability_agent_description ordered by agent_slug, desc_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentDescriptionLoader) LoadCapabilityAgentDescriptions(ctx context.Context) ([]CoreCapabilityAgentDescription, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, desc_key, desc_value, description, created_at
		  FROM ah_core.capability_agent_description
		 ORDER BY agent_slug, desc_key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_description not accessible, agent descriptions unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_agent_description: %w", err)
	}
	defer rows.Close()

	var descriptions []CoreCapabilityAgentDescription
	for rows.Next() {
		var d CoreCapabilityAgentDescription
		if err := rows.Scan(
			&d.ID, &d.AgentSlug, &d.DescKey, &d.DescValue, &d.Description, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_agent_description: %w", err)
		}
		descriptions = append(descriptions, d)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_description not accessible (post-iter), agent descriptions unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_agent_description: %w", err)
	}
	return descriptions, nil
}

// LoadDescriptionsForAgent returns all description rows for the given agentSlug
// from ah_core.capability_agent_description, ordered by desc_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentDescriptionLoader) LoadDescriptionsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityAgentDescription, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, desc_key, desc_value, description, created_at
		  FROM ah_core.capability_agent_description
		 WHERE agent_slug = $1
		 ORDER BY desc_key`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_description not accessible, agent descriptions unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_agent_description for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var descriptions []CoreCapabilityAgentDescription
	for rows.Next() {
		var d CoreCapabilityAgentDescription
		if err := rows.Scan(
			&d.ID, &d.AgentSlug, &d.DescKey, &d.DescValue, &d.Description, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_agent_description for agent %q: %w", agentSlug, err)
		}
		descriptions = append(descriptions, d)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_description not accessible (post-iter), agent descriptions unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_agent_description for agent %q: %w", agentSlug, err)
	}
	return descriptions, nil
}

// GetDescriptionValue returns the desc_value for the given agentSlug and
// descKey from ah_core.capability_agent_description.
// Returns ("", false, nil) when the row does not exist.
// Returns ("", false, nil) (non-fatal) when the table is not accessible.
func (l *CoreCapabilityAgentDescriptionLoader) GetDescriptionValue(ctx context.Context, agentSlug, descKey string) (string, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return "", false, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT desc_value
		  FROM ah_core.capability_agent_description
		 WHERE agent_slug = $1
		   AND desc_key   = $2`

	var value string
	err = conn.QueryRow(ctx, query, agentSlug, descKey).Scan(&value)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_description not accessible, description value unavailable",
				"agent_slug", agentSlug, "desc_key", descKey, "err", err)
			return "", false, nil
		}
		// pgx returns pgx.ErrNoRows when no row found; treat as not-found.
		if err.Error() == "no rows in result set" {
			return "", false, nil
		}
		return "", false, fmt.Errorf("core: query capability_agent_description value for agent %q key %q: %w", agentSlug, descKey, err)
	}
	return value, true, nil
}
