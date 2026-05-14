package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreAgent represents a platform-managed specialist agent from ah_core.agent.
type CoreAgent struct {
	ID               uuid.UUID
	Name             string
	Slug             string
	Description      string
	AgentType        string // ASSISTANT | SPECIALIST
	SystemPrompt     string
	ModelConfig      json.RawMessage
	EnableManagement bool
	IsActive         bool
}

// CoreAgentSkillBinding represents an agent→skill binding from
// ah_core.agent_skill (added during integration backfill 2026-05-10).
type CoreAgentSkillBinding struct {
	ID       uuid.UUID
	AgentID  uuid.UUID
	SkillID  uuid.UUID
	Priority int
}

// CoreAgentLoader loads platform-managed specialist agents from the ah_core schema.
type CoreAgentLoader struct {
	pool *pgxpool.Pool
}

// NewCoreAgentLoader creates a CoreAgentLoader.
func NewCoreAgentLoader(pool *pgxpool.Pool) *CoreAgentLoader {
	return &CoreAgentLoader{pool: pool}
}

// LoadAll returns all active agents from ah_core.agent.
// Returns an empty slice (not an error) if the ah_core schema does not exist.
func (l *CoreAgentLoader) LoadAll(ctx context.Context) ([]CoreAgent, error) {
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
		 WHERE is_active = true
		 ORDER BY name`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.agent not accessible, core agents unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query agents: %w", err)
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
			return nil, fmt.Errorf("core: scan agent: %w", err)
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
		return nil, fmt.Errorf("core: iterate agents: %w", err)
	}
	return agents, nil
}

// FindBySlug returns one agent by slug.
func (l *CoreAgentLoader) FindBySlug(ctx context.Context, slug string) (CoreAgent, bool, error) {
	all, err := l.LoadAll(ctx)
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

// ListSpecialists returns all active specialist agents (agent_type = 'SPECIALIST').
func (l *CoreAgentLoader) ListSpecialists(ctx context.Context) ([]CoreAgent, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var specialists []CoreAgent
	for _, a := range all {
		if a.AgentType == "SPECIALIST" {
			specialists = append(specialists, a)
		}
	}
	return specialists, nil
}

// LoadSkillBindings returns all agent→skill bindings ordered by
// agent_id, priority. Used to materialize each specialist's skill set.
func (l *CoreAgentLoader) LoadSkillBindings(ctx context.Context) ([]CoreAgentSkillBinding, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_id, skill_id, priority
		  FROM ah_core.agent_skill
		 ORDER BY agent_id, priority, id`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.agent_skill not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query agent_skill: %w", err)
	}
	defer rows.Close()

	var bindings []CoreAgentSkillBinding
	for rows.Next() {
		var b CoreAgentSkillBinding
		if err := rows.Scan(&b.ID, &b.AgentID, &b.SkillID, &b.Priority); err != nil {
			return nil, fmt.Errorf("core: scan agent_skill: %w", err)
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate agent_skill: %w", err)
	}
	return bindings, nil
}

// CoreAgentResponse is the DTO returned by GET /api/core/agents.
type CoreAgentResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	AgentType   string    `json:"agentType"`
}

// AgentResponseFrom converts a CoreAgent to CoreAgentResponse.
func AgentResponseFrom(a CoreAgent) CoreAgentResponse {
	return CoreAgentResponse{
		ID:          a.ID,
		Name:        a.Name,
		Slug:        a.Slug,
		Description: a.Description,
		AgentType:   a.AgentType,
	}
}

// ============================================================
// Seed catalog constants — backfill 2026-05-10.
// ============================================================

// SeedExpectedAgentSlugs is the canonical list of slugs the seed
// migration 000006_reset_and_seed_specialists installs (12 specialists
// per spec §21.4).
var SeedExpectedAgentSlugs = []string{
	"core-assistant",
	"core-agent-builder",
	"core-tool-builder",
	"core-skills-specialist",
	"core-tools-specialist",
	"core-kb-builder",
	"core-kb-specialist",
	"core-mcp-configurator",
	"core-api-importer",
	"core-agents-specialist",
	"core-pipeline-specialist",
	"core-execution-specialist",
}

// SeedExpectedAgentTypes is the closed set of agent_type values used
// by the seed.
var SeedExpectedAgentTypes = []string{
	"ASSISTANT",  // core-assistant only
	"SPECIALIST", // the other 11
}

// SeedExpectedAgentSlugPrefix — namespace contract for platform agents.
const SeedExpectedAgentSlugPrefix = "core-"

// SeedAssistantSlug — the single ASSISTANT-type agent slug.
const SeedAssistantSlug = "core-assistant"

// SeedDeprecatedAgentSlug — the deprecated pipeline specialist (kept
// in catalog as read-only — ADR-012 deprecated pipelines 2026-04-02).
const SeedDeprecatedAgentSlug = "core-pipeline-specialist"

// SeedExpectedAgentSkillBindingsCount documents the ACTUAL row count
// observed in DB after migrations 000003 + 000006 are applied.
//
// Only the core-assistant binding INSERT is a CROSS JOIN with no AND
// clause (`SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
// WHERE a.slug = 'core-assistant'`) — it produces 7 rows (1 agent ×
// 7 skills).
//
// The other 10 binding INSERTs reference skill slugs that DO NOT EXIST
// in ah_core.skill (they use `agent-management`, `skill-management`,
// `tool-management`, `knowledge-base-management`, `mcp-management`,
// `settings-management`, `execute-sql` — but the seeded skills use
// `core-agents-management`, `core-skills-management`, etc.). These
// produce ZERO rows each.
//
// Result: 7 bindings actually persist in DB. This is a KNOWN
// discrepancy in the migration; the integration test asserts the
// observed count so any future fix to the slug mismatch will trip
// the test (forcing the constant + ledger to update together).
const SeedExpectedAgentSkillBindingsCount = 7
