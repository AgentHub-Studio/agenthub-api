package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityKnowledgeSource is a single per-agent knowledge source
// declaration seeded by migration 000117. Knowledge source rows are
// declarative hints that tell each capability agent where to look for
// information when fulfilling a request. Each agent has three priority
// slots (primary, secondary, fallback) — lower priority numbers are
// consulted first.
type CoreCapabilityKnowledgeSource struct {
	// ID is the auto-generated BIGSERIAL primary key.
	ID int64
	// AgentSlug identifies the capability agent that owns this source
	// (e.g. "core-researcher", "core-analyst", "core-planner").
	AgentSlug string
	// SourceKey names the priority slot for this source within the agent
	// (e.g. "primary", "secondary", "fallback").
	SourceKey string
	// SourceType is the category of access mechanism used to reach this source
	// (e.g. "web_search", "document_fetch", "knowledge_base", "conversation_context").
	SourceType string
	// SourceRef is the specific source reference within the source type
	// (e.g. "web_search_index", "web_page_content", "tenant_knowledge_base", "session_context").
	SourceRef string
	// Priority is the numeric priority of this source within the agent; 1 = highest.
	// Maps to primary=1, secondary=2, fallback=3.
	Priority int
	// Description is a human-readable explanation of when this source is used
	// and what kind of information it provides.
	Description string
	// IsActive indicates whether this source declaration is currently active.
	// Inactive sources are skipped during source selection at runtime.
	IsActive bool
	// CreatedAt is the UTC timestamp when the row was created.
	CreatedAt time.Time
}

// CoreCapabilityKnowledgeSourceLoader loads per-agent knowledge source
// declarations from ah_core.capability_knowledge_source. The table is seeded
// by migration 000117 with 9 rows (3 per capability agent). All methods are
// non-fatal when the table or schema is missing (supports fresh deployments
// before the migration runs).
type CoreCapabilityKnowledgeSourceLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityKnowledgeSourceLoader creates a
// CoreCapabilityKnowledgeSourceLoader backed by pool.
func NewCoreCapabilityKnowledgeSourceLoader(pool *pgxpool.Pool) *CoreCapabilityKnowledgeSourceLoader {
	return &CoreCapabilityKnowledgeSourceLoader{pool: pool}
}

// LoadCapabilityKnowledgeSources returns all knowledge source rows from
// ah_core.capability_knowledge_source ordered by agent_slug, priority asc.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityKnowledgeSourceLoader) LoadCapabilityKnowledgeSources(ctx context.Context) ([]CoreCapabilityKnowledgeSource, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, source_key, source_type, source_ref, priority, description, is_active, created_at
		  FROM ah_core.capability_knowledge_source
		 ORDER BY agent_slug, priority`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_knowledge_source not accessible, knowledge sources unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability knowledge sources: %w", err)
	}
	defer rows.Close()

	var sources []CoreCapabilityKnowledgeSource
	for rows.Next() {
		var s CoreCapabilityKnowledgeSource
		if err := rows.Scan(
			&s.ID, &s.AgentSlug, &s.SourceKey, &s.SourceType, &s.SourceRef,
			&s.Priority, &s.Description, &s.IsActive, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability knowledge source: %w", err)
		}
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_knowledge_source not accessible (post-iter), knowledge sources unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability knowledge sources: %w", err)
	}
	return sources, nil
}

// LoadKnowledgeSourcesForAgent returns the knowledge source rows from
// ah_core.capability_knowledge_source WHERE agent_slug = $1, ordered by
// priority asc. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityKnowledgeSourceLoader) LoadKnowledgeSourcesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityKnowledgeSource, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, source_key, source_type, source_ref, priority, description, is_active, created_at
		  FROM ah_core.capability_knowledge_source
		 WHERE agent_slug = $1
		 ORDER BY priority`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_knowledge_source not accessible, knowledge sources unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query knowledge sources for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var sources []CoreCapabilityKnowledgeSource
	for rows.Next() {
		var s CoreCapabilityKnowledgeSource
		if err := rows.Scan(
			&s.ID, &s.AgentSlug, &s.SourceKey, &s.SourceType, &s.SourceRef,
			&s.Priority, &s.Description, &s.IsActive, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan knowledge source for agent %q: %w", agentSlug, err)
		}
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_knowledge_source not accessible (post-iter), knowledge sources unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate knowledge sources for agent %q: %w", agentSlug, err)
	}
	return sources, nil
}

// GetPrimaryKnowledgeSource returns the priority=1 (primary) knowledge source
// for the given agentSlug. Returns nil, nil when no row exists or the table is
// not accessible (non-fatal).
func (l *CoreCapabilityKnowledgeSourceLoader) GetPrimaryKnowledgeSource(ctx context.Context, agentSlug string) (*CoreCapabilityKnowledgeSource, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, source_key, source_type, source_ref, priority, description, is_active, created_at
		  FROM ah_core.capability_knowledge_source
		 WHERE agent_slug = $1
		   AND priority = 1
		 LIMIT 1`

	var s CoreCapabilityKnowledgeSource
	err = conn.QueryRow(ctx, query, agentSlug).Scan(
		&s.ID, &s.AgentSlug, &s.SourceKey, &s.SourceType, &s.SourceRef,
		&s.Priority, &s.Description, &s.IsActive, &s.CreatedAt,
	)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_knowledge_source not accessible, returning nil",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		// pgx returns "no rows in result set" when zero rows match.
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("core: get primary knowledge source for agent %q: %w", agentSlug, err)
	}
	return &s, nil
}

// ============================================================
// Seed catalog constants — migration 000117 (2026-05-11).
// ============================================================

// SeedKnowledgeSourceCount is the expected total row count after migration
// 000117. Nine knowledge source rows — three per capability agent (researcher,
// analyst, planner) — covering primary, secondary, and fallback priority slots.
const SeedKnowledgeSourceCount = 9

// SeedKnowledgeSourceAgentCount is the number of capability agents that have
// seeded knowledge source rows in migration 000117 (researcher, analyst, planner).
const SeedKnowledgeSourceAgentCount = 3

// Source type constants — migration 000117.

// SeedSourceTypeWebSearch is the source_type value for live web search access
// ("web_search"). Used as primary source by core-researcher and fallback by
// core-analyst and core-planner.
const SeedSourceTypeWebSearch = "web_search"

// SeedSourceTypeDocumentFetch is the source_type value for fetching and reading
// full web page content ("document_fetch"). Used as secondary source by
// core-researcher when search results are insufficient.
const SeedSourceTypeDocumentFetch = "document_fetch"

// SeedSourceTypeKnowledgeBase is the source_type value for tenant-uploaded
// knowledge base documents ("knowledge_base"). Used as primary source by
// core-analyst and as secondary/fallback by core-researcher and core-planner.
const SeedSourceTypeKnowledgeBase = "knowledge_base"

// SeedSourceTypeConversationContext is the source_type value for data and
// context shared directly in the conversation ("conversation_context"). Used as
// primary source by core-planner and as secondary source by core-analyst.
const SeedSourceTypeConversationContext = "conversation_context"

// Source key constants — migration 000117.

// SeedSourceKeyPrimary is the source_key value for the highest-priority
// knowledge source slot ("primary"). Priority = 1.
const SeedSourceKeyPrimary = "primary"

// SeedSourceKeySecondary is the source_key value for the second-priority
// knowledge source slot ("secondary"). Priority = 2.
const SeedSourceKeySecondary = "secondary"

// SeedSourceKeyFallback is the source_key value for the lowest-priority
// knowledge source slot, consulted when higher-priority sources are
// exhausted ("fallback"). Priority = 3.
const SeedSourceKeyFallback = "fallback"

// Per-agent primary source type constants — migration 000117.

// SeedResearcherPrimaryType is the source_type of the core-researcher primary
// knowledge source. The researcher consults the web search index first.
const SeedResearcherPrimaryType = SeedSourceTypeWebSearch

// SeedAnalystPrimaryType is the source_type of the core-analyst primary
// knowledge source. The analyst consults tenant-uploaded knowledge base data first.
const SeedAnalystPrimaryType = SeedSourceTypeKnowledgeBase

// SeedPlannerPrimaryType is the source_type of the core-planner primary
// knowledge source. The planner draws from conversation context first.
const SeedPlannerPrimaryType = SeedSourceTypeConversationContext
