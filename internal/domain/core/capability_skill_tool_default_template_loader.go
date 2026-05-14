package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilitySkillToolLoader loads capability-category tools and skills
// adapted from Claude Code for AgentHub web (migration 000090).
// These give fresh tenants functional AI capabilities without any custom
// configuration — seeded in ah_core.tool and ah_core.skill with category
// 'capability' (distinct from the 'platform' management layer).
type CoreCapabilitySkillToolLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilitySkillToolLoader creates the loader.
func NewCoreCapabilitySkillToolLoader(pool *pgxpool.Pool) *CoreCapabilitySkillToolLoader {
	return &CoreCapabilitySkillToolLoader{pool: pool}
}

// LoadCapabilityTools returns active tools whose slugs are in SeedCapabilityToolSlugs.
// Returns nil, nil when ah_core.tool is missing (non-fatal — fresh deployment).
func (l *CoreCapabilitySkillToolLoader) LoadCapabilityTools(ctx context.Context) ([]CoreTool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug, COALESCE(description,'') AS description,
		       type, config, is_active
		  FROM ah_core.tool
		 WHERE is_active = true
		   AND slug = ANY($1)
		 ORDER BY name`

	rows, err := conn.Query(ctx, query, SeedCapabilityToolSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.tool not accessible, capability tools unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability tools: %w", err)
	}
	defer rows.Close()

	var tools []CoreTool
	for rows.Next() {
		var t CoreTool
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.Type, &t.Config, &t.IsActive); err != nil {
			return nil, fmt.Errorf("core: scan capability tool: %w", err)
		}
		tools = append(tools, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability tools: %w", err)
	}
	return tools, nil
}

// LoadCapabilitySkills returns active skills in category 'capability'.
// Returns nil, nil when ah_core.skill is missing (non-fatal — fresh deployment).
func (l *CoreCapabilitySkillToolLoader) LoadCapabilitySkills(ctx context.Context) ([]CoreSkill, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       COALESCE(instructions, '') AS instructions,
		       category, disable_model_invocation, context_mode,
		       COALESCE(when_to_use, '') AS when_to_use
		  FROM ah_core.skill
		 WHERE category = 'capability'
		 ORDER BY name`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.skill not accessible, capability skills unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability skills: %w", err)
	}
	defer rows.Close()

	var skills []CoreSkill
	for rows.Next() {
		var s CoreSkill
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Slug, &s.Description, &s.Instructions,
			&s.Category, &s.DisableModelInvocation, &s.ContextMode, &s.WhenToUse,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability skill: %w", err)
		}
		skills = append(skills, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability skills: %w", err)
	}
	return skills, nil
}

// SeedCapabilityToolSlugs is the canonical closed set of capability tools
// adapted from Claude Code for AgentHub web (migration 000090).
// Ordered by Claude Code source tool: WebSearch, WebFetch, Grep, Read,
// TodoWrite, TodoRead, Agent.
var SeedCapabilityToolSlugs = []string{
	"core-web-search",
	"core-web-fetch",
	"core-doc-search",
	"core-doc-read",
	"core-todo-create",
	"core-todo-list",
	"core-subagent-run",
}

// SeedCapabilitySkillSlugs is the canonical closed set of capability skills
// that group the 7 tools into coherent workflows (migration 000090).
var SeedCapabilitySkillSlugs = []string{
	"core-web-research",
	"core-doc-analysis",
	"core-task-workflow",
}

// SeedCapabilityToolCount is the expected row count after migration 000090.
const SeedCapabilityToolCount = 7

// SeedCapabilitySkillCount is the expected row count after migration 000090.
const SeedCapabilitySkillCount = 3

// SeedCapabilitySkillCategory is the category tag for capability skills.
const SeedCapabilitySkillCategory = "capability"

// SeedCapabilityToolTypes is the closed set of tool types used by capability tools.
// DOCUMENT_SEARCH covers core-doc-search; HTTP covers the remaining six.
var SeedCapabilityToolTypes = []string{"HTTP", "DOCUMENT_SEARCH"}

// SeedCapabilitySkillBindingCount is the total skill→tool binding count
// installed by migration 000090 (2 bindings per skill × 3 skills = 6).
const SeedCapabilitySkillBindingCount = 6

// SeedWebResearchToolSlugs lists the tools backing core-web-research.
var SeedWebResearchToolSlugs = []string{"core-web-search", "core-web-fetch"}

// SeedDocAnalysisToolSlugs lists the tools backing core-doc-analysis.
var SeedDocAnalysisToolSlugs = []string{"core-doc-search", "core-doc-read"}

// SeedTaskWorkflowToolSlugs lists the tools backing core-task-workflow.
var SeedTaskWorkflowToolSlugs = []string{"core-todo-create", "core-todo-list"}

// SeedDocumentSearchToolSlug is the only DOCUMENT_SEARCH type tool slug.
const SeedDocumentSearchToolSlug = "core-doc-search"
