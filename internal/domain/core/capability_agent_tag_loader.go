package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentTagLoader loads searchable tags from
// ah_core.capability_agent_tag. These 12 rows (seeded by migration 000109)
// define discoverable tags per capability agent, enabling tag-based agent
// lookup in the AgentHub marketplace and catalog UI:
//
//   - tag-researcher-research   (core-researcher, tag: research)
//   - tag-researcher-web        (core-researcher, tag: web)
//   - tag-researcher-sources    (core-researcher, tag: sources)
//   - tag-researcher-knowledge  (core-researcher, tag: knowledge)
//   - tag-analyst-analysis      (core-analyst,    tag: analysis)
//   - tag-analyst-documents     (core-analyst,    tag: documents)
//   - tag-analyst-insights      (core-analyst,    tag: insights)
//   - tag-analyst-patterns      (core-analyst,    tag: patterns)
//   - tag-planner-planning      (core-planner,    tag: planning)
//   - tag-planner-tasks         (core-planner,    tag: tasks)
//   - tag-planner-projects      (core-planner,    tag: projects)
//   - tag-planner-workflows     (core-planner,    tag: workflows)
//
// Non-fatal when the ah_core schema or the capability_agent_tag table is
// missing — supports fresh deployments where migration 000109 has not yet run.
type CoreCapabilityAgentTagLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentTagLoader creates a CoreCapabilityAgentTagLoader
// backed by pool.
func NewCoreCapabilityAgentTagLoader(pool *pgxpool.Pool) *CoreCapabilityAgentTagLoader {
	return &CoreCapabilityAgentTagLoader{pool: pool}
}

// CoreCapabilityAgentTag is a single agent tag definition. Captures the slug,
// the target agent slug, the searchable tag value, and the display order.
type CoreCapabilityAgentTag struct {
	Slug      string
	AgentSlug string
	Tag       string
	SortOrder int
}

// LoadCapabilityAgentTags returns all tag rows from
// ah_core.capability_agent_tag WHERE agent_slug = ANY($1), ordered by
// agent_slug, sort_order. Returns nil, nil when the table is not accessible
// (non-fatal).
func (l *CoreCapabilityAgentTagLoader) LoadCapabilityAgentTags(ctx context.Context) ([]CoreCapabilityAgentTag, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, tag, sort_order
		  FROM ah_core.capability_agent_tag
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityAgentTagAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tag not accessible, agent tags unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent tags: %w", err)
	}
	defer rows.Close()

	var tags []CoreCapabilityAgentTag
	for rows.Next() {
		var t CoreCapabilityAgentTag
		if err := rows.Scan(
			&t.Slug, &t.AgentSlug, &t.Tag, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tag not accessible (post-iter), agent tags unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent tags: %w", err)
	}
	return tags, nil
}

// LoadTagsForAgent returns the tag strings from
// ah_core.capability_agent_tag WHERE agent_slug = $1, ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentTagLoader) LoadTagsForAgent(ctx context.Context, agentSlug string) ([]string, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT tag
		  FROM ah_core.capability_agent_tag
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tag not accessible, agent tags unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query tags for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("core: scan tag for agent %q: %w", agentSlug, err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tag not accessible (post-iter), agent tags unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate tags for agent %q: %w", agentSlug, err)
	}
	return tags, nil
}

// ============================================================
// Seed catalog constants — migration 000109 (2026-05-11).
// ============================================================

// SeedCapabilityAgentTagCount is the expected total row count after
// migration 000109. Twelve tag rows — four per capability agent
// (researcher, analyst, planner), covering the most common discovery tags.
const SeedCapabilityAgentTagCount = 12

// SeedCapabilityAgentTagAgentSlugs is the canonical list of capability
// agent slugs that have seeded tags in migration 000109.
var SeedCapabilityAgentTagAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherTagCount is the number of tags seeded for core-researcher
// (4 tags: research, web, sources, knowledge).
const SeedResearcherTagCount = 4

// SeedAnalystTagCount is the number of tags seeded for core-analyst
// (4 tags: analysis, documents, insights, patterns).
const SeedAnalystTagCount = 4

// SeedPlannerTagCount is the number of tags seeded for core-planner
// (4 tags: planning, tasks, projects, workflows).
const SeedPlannerTagCount = 4

// SeedResearcherTags is the canonical ordered list of tags seeded for
// core-researcher by migration 000109 (sort_order ascending).
var SeedResearcherTags = []string{"research", "web", "sources", "knowledge"}

// SeedAnalystTags is the canonical ordered list of tags seeded for
// core-analyst by migration 000109 (sort_order ascending).
var SeedAnalystTags = []string{"analysis", "documents", "insights", "patterns"}

// SeedPlannerTags is the canonical ordered list of tags seeded for
// core-planner by migration 000109 (sort_order ascending).
var SeedPlannerTags = []string{"planning", "tasks", "projects", "workflows"}

// SeedCapabilityAgentAllTags is the canonical closed set of all 12 tags
// seeded by migration 000109 (researcher → analyst → planner).
var SeedCapabilityAgentAllTags = func() []string {
	all := make([]string, 0, SeedCapabilityAgentTagCount)
	all = append(all, SeedResearcherTags...)
	all = append(all, SeedAnalystTags...)
	all = append(all, SeedPlannerTags...)
	return all
}()
