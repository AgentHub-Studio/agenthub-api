package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilitySystemPromptTemplateLoader loads capability-specific system
// prompt templates from ah_core.system_prompt_template. These 3 templates
// (sort_order 1–3) provide role-specific LLM instructions for the three
// capability agents introduced in migration 000091:
//
//   - capability-researcher-system-prompt  →  core-researcher  (web research)
//   - capability-analyst-system-prompt     →  core-analyst     (document analysis)
//   - capability-planner-system-prompt     →  core-planner     (task decomposition)
//
// Seeded by migration 000100. The table ah_core.system_prompt_template is also
// created by that migration.
//
// Non-fatal when the ah_core schema or the system_prompt_template table is
// missing — supports fresh deployments where migration 000100 has not yet run.
type CoreCapabilitySystemPromptTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilitySystemPromptTemplateLoader creates a
// CoreCapabilitySystemPromptTemplateLoader backed by pool.
func NewCoreCapabilitySystemPromptTemplateLoader(pool *pgxpool.Pool) *CoreCapabilitySystemPromptTemplateLoader {
	return &CoreCapabilitySystemPromptTemplateLoader{pool: pool}
}

// CoreSystemPromptTemplate is a platform-managed system prompt template for a
// capability agent. Captures the full LLM instruction content, optional agent
// linkage, recommendation status, and display ordering.
type CoreSystemPromptTemplate struct {
	Slug          string
	Name          string
	Description   string
	Content       string
	AgentSlug     string
	IsRecommended bool
	SortOrder     int
}

// LoadCapabilitySystemPromptTemplates returns all capability system prompt
// templates from ah_core.system_prompt_template WHERE slug = ANY($1), ordered
// by sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilitySystemPromptTemplateLoader) LoadCapabilitySystemPromptTemplates(ctx context.Context) ([]CoreSystemPromptTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, name, description, content, agent_slug,
		       is_recommended, sort_order
		  FROM ah_core.system_prompt_template
		 WHERE slug = ANY($1)
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilitySystemPromptTemplateSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.system_prompt_template not accessible, capability system prompt templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability system prompt templates: %w", err)
	}
	defer rows.Close()

	var templates []CoreSystemPromptTemplate
	for rows.Next() {
		var t CoreSystemPromptTemplate
		if err := rows.Scan(
			&t.Slug, &t.Name, &t.Description, &t.Content, &t.AgentSlug,
			&t.IsRecommended, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability system prompt template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.system_prompt_template not accessible (post-iter), capability system prompt templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability system prompt templates: %w", err)
	}
	return templates, nil
}

// FindSystemPromptTemplateByAgentSlug returns the system prompt template
// linked to the given agentSlug from ah_core.system_prompt_template.
// Returns (nil, nil) when no row matches or the table is not accessible
// (non-fatal).
func (l *CoreCapabilitySystemPromptTemplateLoader) FindSystemPromptTemplateByAgentSlug(ctx context.Context, agentSlug string) (*CoreSystemPromptTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, name, description, content, agent_slug,
		       is_recommended, sort_order
		  FROM ah_core.system_prompt_template
		 WHERE agent_slug = $1
		 LIMIT 1`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.system_prompt_template not accessible, system prompt by agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query system prompt template for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	if rows.Next() {
		var t CoreSystemPromptTemplate
		if err := rows.Scan(
			&t.Slug, &t.Name, &t.Description, &t.Content, &t.AgentSlug,
			&t.IsRecommended, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan system prompt template for agent %q: %w", agentSlug, err)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("core: iterate system prompt template for agent %q: %w", agentSlug, err)
		}
		return &t, nil
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.system_prompt_template not accessible (post-iter), system prompt by agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate system prompt template for agent %q: %w", agentSlug, err)
	}
	return nil, nil
}

// ============================================================
// Seed catalog constants — migration 000100 (2026-05-11).
// ============================================================

// SeedCapabilitySystemPromptTemplateCount is the expected row count after
// migration 000100. One system prompt template per capability agent.
const SeedCapabilitySystemPromptTemplateCount = 3

// SeedCapabilitySystemPromptTemplateSlugs is the canonical closed set of
// capability system prompt template slugs seeded in migration 000100.
// One template per capability agent: researcher, analyst, planner.
var SeedCapabilitySystemPromptTemplateSlugs = []string{
	"capability-researcher-system-prompt",
	"capability-analyst-system-prompt",
	"capability-planner-system-prompt",
}

// SeedResearcherSystemPromptSlug is the system prompt template slug for the
// core-researcher capability agent.
const SeedResearcherSystemPromptSlug = "capability-researcher-system-prompt"

// SeedAnalystSystemPromptSlug is the system prompt template slug for the
// core-analyst capability agent.
const SeedAnalystSystemPromptSlug = "capability-analyst-system-prompt"

// SeedPlannerSystemPromptSlug is the system prompt template slug for the
// core-planner capability agent.
const SeedPlannerSystemPromptSlug = "capability-planner-system-prompt"

// SeedCapabilitySystemPromptAgentMap maps system prompt template slug to the
// capability agent slug it is designed for. Used for tests and validation.
var SeedCapabilitySystemPromptAgentMap = map[string]string{
	"capability-researcher-system-prompt": "core-researcher",
	"capability-analyst-system-prompt":    "core-analyst",
	"capability-planner-system-prompt":    "core-planner",
}
