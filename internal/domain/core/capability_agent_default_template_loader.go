package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentLoader loads capability-oriented agent templates from ah_core.
// These agents are adapted from Claude Code's general-purpose, Explore, and Plan
// agent types and give tenants real AI capability without custom configuration.
// Seeded by migration 000091_seed_capability_agent_templates.
type CoreCapabilityAgentLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentLoader creates a CoreCapabilityAgentLoader.
func NewCoreCapabilityAgentLoader(pool *pgxpool.Pool) *CoreCapabilityAgentLoader {
	return &CoreCapabilityAgentLoader{pool: pool}
}

// LoadCapabilityAgents returns active agents whose slugs are in
// SeedCapabilityAgentSlugs. Returns nil, nil when ah_core.agent is missing
// (non-fatal — fresh deployment without the seed migration applied).
func (l *CoreCapabilityAgentLoader) LoadCapabilityAgents(ctx context.Context) ([]CoreAgent, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       agent_type, system_prompt, model_config,
		       enable_management, is_active
		  FROM ah_core.agent
		 WHERE slug = ANY($1)
		   AND is_active = true
		 ORDER BY name`

	rows, err := conn.Query(ctx, query, SeedCapabilityAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.agent not accessible, capability agents unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agents: %w", err)
	}
	defer rows.Close()

	var agents []CoreAgent
	for rows.Next() {
		var a CoreAgent
		var sp *string
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Slug, &a.Description,
			&a.AgentType, &sp, &a.ModelConfig,
			&a.EnableManagement, &a.IsActive,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent: %w", err)
		}
		if sp != nil {
			a.SystemPrompt = *sp
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agents: %w", err)
	}
	return agents, nil
}

// FindCapabilityAgentBySlug returns a single capability agent by slug.
// Returns (agent, true, nil) when found, (zero, false, nil) when not found.
func (l *CoreCapabilityAgentLoader) FindCapabilityAgentBySlug(ctx context.Context, slug string) (CoreAgent, bool, error) {
	all, err := l.LoadCapabilityAgents(ctx)
	if err != nil {
		return CoreAgent{}, false, err
	}
	for _, a := range all {
		if a.Slug == slug {
			return a, true, nil
		}
	}
	return CoreAgent{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000091 (2026-05-11).
// ============================================================

// SeedCapabilityAgentSlugs is the canonical closed set of capability agent slugs
// seeded in migration 000091. Each agent wraps one capability skill from 000090.
// Agents are adapted from Claude Code's built-in agent types:
//   - core-researcher  → general-purpose agent type
//   - core-analyst     → Explore agent type
//   - core-planner     → Plan agent type
var SeedCapabilityAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedCapabilityAgentCount is the expected row count after migration 000091.
const SeedCapabilityAgentCount = 3

// SeedCapabilityAgentType is the agent_type value for all capability agents.
// All three are ASSISTANT-type (interactive, user-facing) not SPECIALIST.
const SeedCapabilityAgentType = "ASSISTANT"

// SeedCapabilityAgentSkillBindingCount is the total agent→skill binding count
// installed by migration 000091 (1 binding per agent × 3 agents = 3).
const SeedCapabilityAgentSkillBindingCount = 3

// SeedResearcherSkillSlug is the skill bound to core-researcher.
const SeedResearcherSkillSlug = "core-web-research"

// SeedAnalystSkillSlug is the skill bound to core-analyst.
const SeedAnalystSkillSlug = "core-doc-analysis"

// SeedPlannerSkillSlug is the skill bound to core-planner.
const SeedPlannerSkillSlug = "core-task-workflow"
