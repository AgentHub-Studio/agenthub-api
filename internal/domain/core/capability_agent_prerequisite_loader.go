package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentPrerequisiteLoader loads prerequisite definitions from
// ah_core.capability_agent_prerequisite. These 6 rows (seeded by migration 000110)
// encode the minimum viable configuration each capability agent requires to function
// — referencing feature flags and skills that must exist for the agent to work
// properly:
//
//   - prereq-researcher-citations-flag      (core-researcher, feature_flag: capability-citations)
//   - prereq-researcher-web-research-skill  (core-researcher, skill:        core-web-research)
//   - prereq-analyst-doc-citations-flag     (core-analyst,    feature_flag: capability-doc-citations)
//   - prereq-analyst-doc-analysis-skill     (core-analyst,    skill:        core-doc-analysis)
//   - prereq-planner-task-tracking-flag     (core-planner,    feature_flag: capability-task-tracking)
//   - prereq-planner-task-workflow-skill    (core-planner,    skill:        core-task-workflow)
//
// Non-fatal when the ah_core schema or the capability_agent_prerequisite table is
// missing — supports fresh deployments where migration 000110 has not yet run.
type CoreCapabilityAgentPrerequisiteLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentPrerequisiteLoader creates a
// CoreCapabilityAgentPrerequisiteLoader backed by pool.
func NewCoreCapabilityAgentPrerequisiteLoader(pool *pgxpool.Pool) *CoreCapabilityAgentPrerequisiteLoader {
	return &CoreCapabilityAgentPrerequisiteLoader{pool: pool}
}

// CoreCapabilityAgentPrerequisite is a single agent prerequisite definition.
// Captures the slug, the target agent slug, the prerequisite type (feature_flag
// or skill), the referenced slug, whether the prerequisite is required, and the
// human-readable reason.
type CoreCapabilityAgentPrerequisite struct {
	Slug             string
	AgentSlug        string
	PrerequisiteType string
	PrerequisiteSlug string
	IsRequired       bool
	Reason           string
	SortOrder        int
}

// LoadCapabilityAgentPrerequisites returns all prerequisite rows from
// ah_core.capability_agent_prerequisite WHERE agent_slug = ANY($1), ordered by
// agent_slug, sort_order. Returns nil, nil when the table is not accessible
// (non-fatal).
func (l *CoreCapabilityAgentPrerequisiteLoader) LoadCapabilityAgentPrerequisites(ctx context.Context) ([]CoreCapabilityAgentPrerequisite, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, prerequisite_type, prerequisite_slug,
		       is_required, reason, sort_order
		  FROM ah_core.capability_agent_prerequisite
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityAgentPrerequisiteAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_prerequisite not accessible, agent prerequisites unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent prerequisites: %w", err)
	}
	defer rows.Close()

	var prereqs []CoreCapabilityAgentPrerequisite
	for rows.Next() {
		var p CoreCapabilityAgentPrerequisite
		if err := rows.Scan(
			&p.Slug, &p.AgentSlug, &p.PrerequisiteType, &p.PrerequisiteSlug,
			&p.IsRequired, &p.Reason, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent prerequisite: %w", err)
		}
		prereqs = append(prereqs, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_prerequisite not accessible (post-iter), agent prerequisites unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent prerequisites: %w", err)
	}
	return prereqs, nil
}

// LoadPrerequisitesForAgent returns the prerequisite rows from
// ah_core.capability_agent_prerequisite WHERE agent_slug = $1, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentPrerequisiteLoader) LoadPrerequisitesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityAgentPrerequisite, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, prerequisite_type, prerequisite_slug,
		       is_required, reason, sort_order
		  FROM ah_core.capability_agent_prerequisite
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_prerequisite not accessible, agent prerequisites unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query prerequisites for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var prereqs []CoreCapabilityAgentPrerequisite
	for rows.Next() {
		var p CoreCapabilityAgentPrerequisite
		if err := rows.Scan(
			&p.Slug, &p.AgentSlug, &p.PrerequisiteType, &p.PrerequisiteSlug,
			&p.IsRequired, &p.Reason, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan prerequisite for agent %q: %w", agentSlug, err)
		}
		prereqs = append(prereqs, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_prerequisite not accessible (post-iter), agent prerequisites unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate prerequisites for agent %q: %w", agentSlug, err)
	}
	return prereqs, nil
}

// ============================================================
// Seed catalog constants — migration 000110 (2026-05-11).
// ============================================================

// SeedCapabilityAgentPrerequisiteCount is the expected total row count after
// migration 000110. Six prerequisite rows — two per capability agent
// (researcher, analyst, planner), each agent requiring one feature_flag and
// one skill.
const SeedCapabilityAgentPrerequisiteCount = 6

// SeedCapabilityAgentPrerequisiteAgentSlugs is the canonical list of capability
// agent slugs that have seeded prerequisites in migration 000110.
var SeedCapabilityAgentPrerequisiteAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherPrerequisiteCount is the number of prerequisites seeded for
// core-researcher (2: one feature_flag + one skill).
const SeedResearcherPrerequisiteCount = 2

// SeedAnalystPrerequisiteCount is the number of prerequisites seeded for
// core-analyst (2: one feature_flag + one skill).
const SeedAnalystPrerequisiteCount = 2

// SeedPlannerPrerequisiteCount is the number of prerequisites seeded for
// core-planner (2: one feature_flag + one skill).
const SeedPlannerPrerequisiteCount = 2

// SeedPrerequisiteTypeFeatureFlag is the prerequisite_type value for feature
// flag dependencies.
const SeedPrerequisiteTypeFeatureFlag = "feature_flag"

// SeedPrerequisiteTypeSkill is the prerequisite_type value for skill
// dependencies.
const SeedPrerequisiteTypeSkill = "skill"

// SeedAllPrerequisitesRequired indicates that all 6 seeded prerequisites are
// required (is_required = true).
const SeedAllPrerequisitesRequired = true

// Individual slug constants — migration 000110.

// SeedResearcherCitationsFlagPrereqSlug is the slug for the researcher's
// feature_flag prerequisite (capability-citations).
const SeedResearcherCitationsFlagPrereqSlug = "prereq-researcher-citations-flag"

// SeedResearcherWebResearchSkillPrereqSlug is the slug for the researcher's
// skill prerequisite (core-web-research).
const SeedResearcherWebResearchSkillPrereqSlug = "prereq-researcher-web-research-skill"

// SeedAnalystDocCitationsFlagPrereqSlug is the slug for the analyst's
// feature_flag prerequisite (capability-doc-citations).
const SeedAnalystDocCitationsFlagPrereqSlug = "prereq-analyst-doc-citations-flag"

// SeedAnalystDocAnalysisSkillPrereqSlug is the slug for the analyst's
// skill prerequisite (core-doc-analysis).
const SeedAnalystDocAnalysisSkillPrereqSlug = "prereq-analyst-doc-analysis-skill"

// SeedPlannerTaskTrackingFlagPrereqSlug is the slug for the planner's
// feature_flag prerequisite (capability-task-tracking).
const SeedPlannerTaskTrackingFlagPrereqSlug = "prereq-planner-task-tracking-flag"

// SeedPlannerTaskWorkflowSkillPrereqSlug is the slug for the planner's
// skill prerequisite (core-task-workflow).
const SeedPlannerTaskWorkflowSkillPrereqSlug = "prereq-planner-task-workflow-skill"
