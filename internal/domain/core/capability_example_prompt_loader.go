package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityExamplePromptLoader loads example user prompts from
// ah_core.capability_example_prompt. These 12 rows (seeded by migration 000108)
// define example prompts shown to new users in the AgentHub web UI when
// starting a conversation with capability agents. Example prompts are adapted
// from Claude Code's example queries in README/docs patterns:
//
//   - example-researcher-web-research   (core-researcher, web_research)
//   - example-researcher-competitor     (core-researcher, competitive_analysis)
//   - example-researcher-technical      (core-researcher, technical_research)
//   - example-researcher-market         (core-researcher, market_research)
//   - example-analyst-doc-summary       (core-analyst,    document_analysis)
//   - example-analyst-compare           (core-analyst,    comparison)
//   - example-analyst-extract           (core-analyst,    extraction)
//   - example-analyst-pattern           (core-analyst,    pattern_analysis)
//   - example-planner-project           (core-planner,    project_planning)
//   - example-planner-sprint            (core-planner,    task_breakdown)
//   - example-planner-release           (core-planner,    release_planning)
//   - example-planner-debug             (core-planner,    debugging)
//
// Non-fatal when the ah_core schema or the capability_example_prompt table is
// missing — supports fresh deployments where migration 000108 has not yet run.
type CoreCapabilityExamplePromptLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityExamplePromptLoader creates a CoreCapabilityExamplePromptLoader
// backed by pool.
func NewCoreCapabilityExamplePromptLoader(pool *pgxpool.Pool) *CoreCapabilityExamplePromptLoader {
	return &CoreCapabilityExamplePromptLoader{pool: pool}
}

// CoreCapabilityExamplePrompt is a single example prompt definition. Captures
// the slug, the target agent slug, the prompt text to display, the functional
// category, and the display order.
type CoreCapabilityExamplePrompt struct {
	Slug       string
	AgentSlug  string
	PromptText string
	Category   string
	SortOrder  int
}

// LoadCapabilityExamplePrompts returns all example prompt rows from
// ah_core.capability_example_prompt WHERE agent_slug = ANY($1), ordered by
// agent_slug, sort_order. Returns nil, nil when the table is not accessible
// (non-fatal).
func (l *CoreCapabilityExamplePromptLoader) LoadCapabilityExamplePrompts(ctx context.Context) ([]CoreCapabilityExamplePrompt, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, prompt_text, category, sort_order
		  FROM ah_core.capability_example_prompt
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityExamplePromptAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_example_prompt not accessible, example prompts unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability example prompts: %w", err)
	}
	defer rows.Close()

	var prompts []CoreCapabilityExamplePrompt
	for rows.Next() {
		var p CoreCapabilityExamplePrompt
		if err := rows.Scan(
			&p.Slug, &p.AgentSlug, &p.PromptText, &p.Category, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability example prompt: %w", err)
		}
		prompts = append(prompts, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_example_prompt not accessible (post-iter), example prompts unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability example prompts: %w", err)
	}
	return prompts, nil
}

// LoadExamplePromptsForAgent returns all example prompt rows from
// ah_core.capability_example_prompt WHERE agent_slug = $1, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityExamplePromptLoader) LoadExamplePromptsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityExamplePrompt, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, prompt_text, category, sort_order
		  FROM ah_core.capability_example_prompt
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_example_prompt not accessible, agent example prompts unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query example prompts for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var prompts []CoreCapabilityExamplePrompt
	for rows.Next() {
		var p CoreCapabilityExamplePrompt
		if err := rows.Scan(
			&p.Slug, &p.AgentSlug, &p.PromptText, &p.Category, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan agent example prompt: %w", err)
		}
		prompts = append(prompts, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_example_prompt not accessible (post-iter), agent example prompts unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate agent example prompts: %w", err)
	}
	return prompts, nil
}

// ============================================================
// Seed catalog constants — migration 000108 (2026-05-11).
// ============================================================

// SeedCapabilityExamplePromptCount is the expected total row count after
// migration 000108. Twelve example prompt rows — four per capability agent
// (researcher, analyst, planner), covering the most common use-case categories.
const SeedCapabilityExamplePromptCount = 12

// SeedCapabilityExamplePromptAgentSlugs is the canonical list of capability
// agent slugs that have seeded example prompts in migration 000108.
var SeedCapabilityExamplePromptAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherExamplePromptCount is the number of example prompts for
// core-researcher (4 examples covering web, competitive, technical, and market
// research).
const SeedResearcherExamplePromptCount = 4

// SeedAnalystExamplePromptCount is the number of example prompts for
// core-analyst (4 examples covering document analysis, comparison, extraction,
// and pattern analysis).
const SeedAnalystExamplePromptCount = 4

// SeedPlannerExamplePromptCount is the number of example prompts for
// core-planner (4 examples covering project planning, task breakdown, release
// planning, and debugging).
const SeedPlannerExamplePromptCount = 4

// SeedResearcherExamplePromptSlugs is the canonical closed set of slugs seeded
// for core-researcher by migration 000108, in sort_order ascending.
var SeedResearcherExamplePromptSlugs = []string{
	"example-researcher-web-research",
	"example-researcher-competitor",
	"example-researcher-technical",
	"example-researcher-market",
}

// SeedAnalystExamplePromptSlugs is the canonical closed set of slugs seeded
// for core-analyst by migration 000108, in sort_order ascending.
var SeedAnalystExamplePromptSlugs = []string{
	"example-analyst-doc-summary",
	"example-analyst-compare",
	"example-analyst-extract",
	"example-analyst-pattern",
}

// SeedPlannerExamplePromptSlugs is the canonical closed set of slugs seeded
// for core-planner by migration 000108, in sort_order ascending.
var SeedPlannerExamplePromptSlugs = []string{
	"example-planner-project",
	"example-planner-sprint",
	"example-planner-release",
	"example-planner-debug",
}

// SeedCapabilityExamplePromptSlugs is the canonical closed set of all 12 slugs
// seeded by migration 000108, in sort_order ascending (researcher → analyst →
// planner).
var SeedCapabilityExamplePromptSlugs = func() []string {
	all := make([]string, 0, SeedCapabilityExamplePromptCount)
	all = append(all, SeedResearcherExamplePromptSlugs...)
	all = append(all, SeedAnalystExamplePromptSlugs...)
	all = append(all, SeedPlannerExamplePromptSlugs...)
	return all
}()
