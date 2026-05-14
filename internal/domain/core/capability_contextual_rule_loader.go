package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityContextualRule is a single per-agent contextual behavior rule
// entry seeded by migration 000114. Contextual rules encode natural-language
// instructions that capability agents follow depending on the situation, adapted
// from Claude Code's CLAUDE.md rule system for web-context agents.
type CoreCapabilityContextualRule struct {
	// ID is the auto-generated BIGSERIAL primary key.
	ID int64
	// AgentSlug identifies the capability agent that owns this rule
	// (e.g. "core-researcher", "core-analyst", "core-planner").
	AgentSlug string
	// RuleText is the natural-language instruction in full
	// (e.g. "always cite sources when making factual claims").
	RuleText string
	// TriggerContext specifies when the rule is active.
	// Known values: "always", "on_factual_claim", "on_uncertainty",
	// "on_task_start", "on_ambiguity", "on_plan_request",
	// "on_code_generation", "on_multiple_approaches".
	TriggerContext string
	// Priority controls evaluation order within an agent's rule set.
	// Higher values are evaluated first. Seed values: 0 (low), 5 (medium), 10 (high).
	Priority int
	// IsActive indicates whether the rule is currently enforced.
	// All seeded rules default to TRUE.
	IsActive bool
	// CreatedAt is the UTC timestamp when the row was created.
	CreatedAt time.Time
}

// CoreCapabilityContextualRuleLoader loads per-agent contextual behavior rules
// from ah_core.capability_contextual_rule. The table is seeded by migration
// 000114 with 9 rows (3 per capability agent). Non-fatal when the table or
// schema is missing (supports fresh deployments before the migration runs).
type CoreCapabilityContextualRuleLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityContextualRuleLoader creates a
// CoreCapabilityContextualRuleLoader backed by pool.
func NewCoreCapabilityContextualRuleLoader(pool *pgxpool.Pool) *CoreCapabilityContextualRuleLoader {
	return &CoreCapabilityContextualRuleLoader{pool: pool}
}

// LoadCapabilityContextualRules returns all active contextual rule rows from
// ah_core.capability_contextual_rule WHERE is_active = TRUE, ordered by
// agent_slug then priority DESC. Returns nil, nil when the table is not
// accessible (non-fatal).
func (l *CoreCapabilityContextualRuleLoader) LoadCapabilityContextualRules(ctx context.Context) ([]CoreCapabilityContextualRule, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, rule_text, trigger_context, priority, is_active, created_at
		  FROM ah_core.capability_contextual_rule
		 WHERE is_active = TRUE
		 ORDER BY agent_slug, priority DESC`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_contextual_rule not accessible, contextual rules unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability contextual rules: %w", err)
	}
	defer rows.Close()

	var rules []CoreCapabilityContextualRule
	for rows.Next() {
		var r CoreCapabilityContextualRule
		if err := rows.Scan(
			&r.ID, &r.AgentSlug, &r.RuleText, &r.TriggerContext,
			&r.Priority, &r.IsActive, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability contextual rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_contextual_rule not accessible (post-iter), contextual rules unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability contextual rules: %w", err)
	}
	return rules, nil
}

// LoadContextualRulesForAgent returns the active contextual rule rows from
// ah_core.capability_contextual_rule WHERE agent_slug = $1 AND
// is_active = TRUE, ordered by priority DESC. Returns nil, nil when the table
// is not accessible (non-fatal).
func (l *CoreCapabilityContextualRuleLoader) LoadContextualRulesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityContextualRule, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, rule_text, trigger_context, priority, is_active, created_at
		  FROM ah_core.capability_contextual_rule
		 WHERE agent_slug = $1
		   AND is_active = TRUE
		 ORDER BY priority DESC`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_contextual_rule not accessible, contextual rules unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query contextual rules for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var rules []CoreCapabilityContextualRule
	for rows.Next() {
		var r CoreCapabilityContextualRule
		if err := rows.Scan(
			&r.ID, &r.AgentSlug, &r.RuleText, &r.TriggerContext,
			&r.Priority, &r.IsActive, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan contextual rule for agent %q: %w", agentSlug, err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_contextual_rule not accessible (post-iter), contextual rules unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate contextual rules for agent %q: %w", agentSlug, err)
	}
	return rules, nil
}

// ============================================================
// Seed catalog constants — migration 000114 (2026-05-11).
// ============================================================

// SeedContextualRuleCount is the expected total row count after migration
// 000114. Nine contextual rule rows — three per capability agent (researcher,
// analyst, planner), each encoding a natural-language behavioral instruction
// adapted from Claude Code's CLAUDE.md rule system.
const SeedContextualRuleCount = 9

// SeedContextualRuleAgentCount is the number of capability agents that have
// seeded contextual rules in migration 000114 (researcher, analyst, planner).
const SeedContextualRuleAgentCount = 3

// SeedResearcherContextualRuleCount is the number of contextual rules seeded
// for core-researcher (3: cite_sources + prefer_search + summarize_findings).
const SeedResearcherContextualRuleCount = 3

// SeedAnalystContextualRuleCount is the number of contextual rules seeded for
// core-analyst (3: break_steps + show_uncertainty + validate_assumptions).
const SeedAnalystContextualRuleCount = 3

// SeedPlannerContextualRuleCount is the number of contextual rules seeded for
// core-planner (3: ask_clarification + break_tasks + present_tradeoffs).
const SeedPlannerContextualRuleCount = 3

// First-rule text slug constants (first rule per agent, highest priority).

// SeedResearcherCiteSourcesRule is the rule_text of the highest-priority
// contextual rule seeded for core-researcher. Priority 10 — the most critical
// rule for research integrity, as false citations are the main risk for the
// researcher agent.
const SeedResearcherCiteSourcesRule = "always cite sources when making factual claims"

// SeedAnalyzerBreakStepsRule is the rule_text for the core-analyst rule that
// requires labeled step decomposition before presenting conclusions. Priority 5
// — medium priority behind validate_assumptions (priority 10).
const SeedAnalyzerBreakStepsRule = "break complex analysis into labeled steps before presenting conclusions"

// SeedPlannerAskClarificationRule is the rule_text of the highest-priority
// contextual rule seeded for core-planner. Priority 10 — plans built on
// ambiguous goals must be redone entirely, making it the highest-cost
// planning mistake.
const SeedPlannerAskClarificationRule = "ask for clarification before generating implementation plans"

// Trigger context constants — migration 000114.

// SeedTriggerContextAlways is the trigger_context value meaning the rule
// applies unconditionally in every agent turn.
const SeedTriggerContextAlways = "always"

// SeedTriggerContextOnUncertainty is the trigger_context value meaning the
// rule fires when the agent detects data uncertainty or low-confidence inputs.
const SeedTriggerContextOnUncertainty = "on_uncertainty"

// SeedTriggerContextOnTaskStart is the trigger_context value meaning the rule
// fires at the start of a new task or analysis session.
const SeedTriggerContextOnTaskStart = "on_task_start"

// SeedTriggerContextOnAmbiguity is the trigger_context value meaning the rule
// fires when the agent encounters ambiguous data or requirements.
const SeedTriggerContextOnAmbiguity = "on_ambiguity"

// SeedTriggerContextOnPlanRequest is the trigger_context value meaning the
// rule fires when the agent is asked to generate a plan or roadmap.
const SeedTriggerContextOnPlanRequest = "on_plan_request"

// SeedTriggerContextOnMultipleApproaches is the trigger_context value meaning
// the rule fires when the agent identifies multiple viable solution approaches.
const SeedTriggerContextOnMultipleApproaches = "on_multiple_approaches"

// Priority constants — migration 000114.

// SeedContextualRulePriorityHigh is the priority value (10) assigned to the
// most critical contextual rules. High-priority rules are evaluated first
// within an agent's active rule set.
const SeedContextualRulePriorityHigh = 10

// SeedContextualRulePriorityMedium is the priority value (5) assigned to
// important but non-critical contextual rules.
const SeedContextualRulePriorityMedium = 5

// SeedContextualRulePriorityLow is the default priority value (0) for
// contextual rules that apply broadly but yield to higher-priority rules.
const SeedContextualRulePriorityLow = 0

// SeedAllContextualRulesActive signals that all 9 seeded contextual rules have
// is_active = TRUE by default. Rules may be disabled by operators after
// deployment to relax behavior for specific agents.
const SeedAllContextualRulesActive = true
