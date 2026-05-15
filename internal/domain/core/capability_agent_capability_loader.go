package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentCapability is a single per-agent capability declaration
// seeded by migration 000119. Each row specifies whether a capability agent
// supports a given capability (is_supported=true) or not (is_supported=false).
// Supported capabilities are shown as active badges in the frontend; unsupported
// ones are shown as greyed-out "not supported" badges. The orchestrator uses
// these rows to route requests to the most capable agent.
type CoreCapabilityAgentCapability struct {
	// ID is the auto-generated BIGSERIAL primary key.
	ID int64
	// AgentSlug identifies the capability agent that declares this capability
	// (e.g. "core-researcher", "core-analyst", "core-planner").
	AgentSlug string
	// CapabilityKey is the machine-readable capability identifier
	// (e.g. "web_search", "document_analysis", "code_generation").
	CapabilityKey string
	// IsSupported indicates whether the agent supports this capability.
	// FALSE rows are displayed as "not supported" badges in the frontend UI.
	IsSupported bool
	// DisplayLabel is the human-readable label shown in the frontend capability badge.
	DisplayLabel string
	// Description is a full sentence explaining why the agent supports or does not
	// support this capability.
	Description string
	// DisplayOrder is the ascending sort order for capabilities within an agent.
	DisplayOrder int
	// CreatedAt is the UTC timestamp when the row was created.
	CreatedAt time.Time
}

// CoreCapabilityAgentCapabilityLoader loads per-agent capability declarations
// from ah_core.capability_agent_capability. The table is seeded by migration
// 000119 with 12 rows (4 per capability agent). All methods are non-fatal when
// the table or schema is missing (supports fresh deployments before the
// migration runs).
type CoreCapabilityAgentCapabilityLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentCapabilityLoader creates a
// CoreCapabilityAgentCapabilityLoader backed by pool.
func NewCoreCapabilityAgentCapabilityLoader(pool *pgxpool.Pool) *CoreCapabilityAgentCapabilityLoader {
	return &CoreCapabilityAgentCapabilityLoader{pool: pool}
}

// LoadCapabilityAgentCapabilities returns all capability rows from
// ah_core.capability_agent_capability ordered by agent_slug, display_order asc.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentCapabilityLoader) LoadCapabilityAgentCapabilities(ctx context.Context) ([]CoreCapabilityAgentCapability, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, capability_key, is_supported, display_label,
		       description, display_order, created_at
		  FROM ah_core.capability_agent_capability
		 ORDER BY agent_slug, display_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_capability not accessible, agent capabilities unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent capabilities: %w", err)
	}
	defer rows.Close()

	var caps []CoreCapabilityAgentCapability
	for rows.Next() {
		var c CoreCapabilityAgentCapability
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.CapabilityKey, &c.IsSupported,
			&c.DisplayLabel, &c.Description, &c.DisplayOrder, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent capability: %w", err)
		}
		caps = append(caps, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_capability not accessible (post-iter), agent capabilities unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent capabilities: %w", err)
	}
	return caps, nil
}

// LoadCapabilitiesForAgent returns the capability rows from
// ah_core.capability_agent_capability WHERE agent_slug = $1, ordered by
// display_order asc. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentCapabilityLoader) LoadCapabilitiesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityAgentCapability, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, capability_key, is_supported, display_label,
		       description, display_order, created_at
		  FROM ah_core.capability_agent_capability
		 WHERE agent_slug = $1
		 ORDER BY display_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_capability not accessible, agent capabilities unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capabilities for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var caps []CoreCapabilityAgentCapability
	for rows.Next() {
		var c CoreCapabilityAgentCapability
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.CapabilityKey, &c.IsSupported,
			&c.DisplayLabel, &c.Description, &c.DisplayOrder, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability for agent %q: %w", agentSlug, err)
		}
		caps = append(caps, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_capability not accessible (post-iter), agent capabilities unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capabilities for agent %q: %w", agentSlug, err)
	}
	return caps, nil
}

// LoadSupportedCapabilitiesForAgent returns only the capability rows with
// is_supported=true from ah_core.capability_agent_capability WHERE
// agent_slug = $1, ordered by display_order asc. Returns nil, nil when the
// table is not accessible (non-fatal).
func (l *CoreCapabilityAgentCapabilityLoader) LoadSupportedCapabilitiesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityAgentCapability, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, capability_key, is_supported, display_label,
		       description, display_order, created_at
		  FROM ah_core.capability_agent_capability
		 WHERE agent_slug = $1
		   AND is_supported = TRUE
		 ORDER BY display_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_capability not accessible, supported capabilities unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query supported capabilities for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var caps []CoreCapabilityAgentCapability
	for rows.Next() {
		var c CoreCapabilityAgentCapability
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.CapabilityKey, &c.IsSupported,
			&c.DisplayLabel, &c.Description, &c.DisplayOrder, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan supported capability for agent %q: %w", agentSlug, err)
		}
		caps = append(caps, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_capability not accessible (post-iter), supported capabilities unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate supported capabilities for agent %q: %w", agentSlug, err)
	}
	return caps, nil
}

// ============================================================
// Seed catalog constants — migration 000119 (2026-05-11).
// ============================================================

// SeedAgentCapabilityCount is the expected total row count after migration
// 000119. Twelve capability rows — four per capability agent (researcher,
// analyst, planner) — covering both supported and unsupported capabilities.
const SeedAgentCapabilityCount = 12

// SeedAgentCapabilityAgentCount is the number of capability agents that have
// seeded capability rows in migration 000119 (researcher, analyst, planner).
const SeedAgentCapabilityAgentCount = 3

// Capability key constants — migration 000119.

// SeedCapKeyWebSearch is the capability_key for the web search capability
// ("web_search"). Supported by core-researcher, core-analyst, and core-planner.
const SeedCapKeyWebSearch = "web_search"

// SeedCapKeyDocAnalysis is the capability_key for the document analysis
// capability ("document_analysis"). Supported by core-researcher and
// core-analyst.
const SeedCapKeyDocAnalysis = "document_analysis"

// SeedCapKeyCodeGen is the capability_key for the code generation capability
// ("code_generation"). Not supported by any of the three core agents.
const SeedCapKeyCodeGen = "code_generation"

// SeedCapKeyTaskPlanning is the capability_key for the task planning capability
// ("task_planning"). Supported by core-planner; not supported by core-researcher.
const SeedCapKeyTaskPlanning = "task_planning"

// SeedCapKeyDataAnalysis is the capability_key for the data analysis capability
// ("data_analysis"). Supported by core-analyst only.
const SeedCapKeyDataAnalysis = "data_analysis"

// SeedCapKeySubagentDelegation is the capability_key for the subagent delegation
// capability ("subagent_delegation"). Supported by core-planner only.
const SeedCapKeySubagentDelegation = "subagent_delegation"

// Per-agent supported capability counts — migration 000119.

// SeedResearcherSupportedCount is the number of is_supported=true capability
// rows for core-researcher (web_search + document_analysis = 2).
const SeedResearcherSupportedCount = 2

// SeedAnalystSupportedCount is the number of is_supported=true capability
// rows for core-analyst (data_analysis + document_analysis + web_search = 3).
const SeedAnalystSupportedCount = 3

// SeedPlannerSupportedCount is the number of is_supported=true capability
// rows for core-planner (task_planning + subagent_delegation + web_search = 3).
const SeedPlannerSupportedCount = 3
