package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityEscalationRuleLoader loads per-agent escalation rules from
// ah_core.capability_escalation_rule. These 9 rows (seeded by migration
// 000112) encode when capability agents should stop acting autonomously and
// escalate to the user for guidance — adapted from Claude Code's
// abort/interrupt patterns (§4.5):
//
//   - esc-researcher-low-confidence    (core-researcher, low_source_confidence,     0.4, pause_and_ask)
//   - esc-researcher-tool-failure      (core-researcher, consecutive_tool_failures, 3,   pause_and_ask)
//   - esc-researcher-out-of-scope      (core-researcher, out_of_scope_request,      1,   clarify_scope)
//   - esc-analyst-low-confidence       (core-analyst,    low_analysis_confidence,   0.5, pause_and_ask)
//   - esc-analyst-ambiguous-doc        (core-analyst,    ambiguous_document_content,1,   clarify_intent)
//   - esc-analyst-tool-failure         (core-analyst,    consecutive_tool_failures,  2,   pause_and_ask)
//   - esc-planner-ambiguous-goal       (core-planner,    ambiguous_goal,             1,   clarify_goal)
//   - esc-planner-dependency-conflict  (core-planner,    dependency_conflict_detected,1,  pause_and_ask)
//   - esc-planner-tool-failure         (core-planner,    consecutive_tool_failures,   3,  pause_and_ask)
//
// Non-fatal when the ah_core schema or the capability_escalation_rule table is
// missing — supports fresh deployments where migration 000112 has not yet run.
type CoreCapabilityEscalationRuleLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityEscalationRuleLoader creates a
// CoreCapabilityEscalationRuleLoader backed by pool.
func NewCoreCapabilityEscalationRuleLoader(pool *pgxpool.Pool) *CoreCapabilityEscalationRuleLoader {
	return &CoreCapabilityEscalationRuleLoader{pool: pool}
}

// CoreCapabilityEscalationRule is a single per-agent escalation rule entry.
// Captures the slug, the target agent slug, the trigger condition, the
// threshold value, the action to take, the active flag, and the display
// sort order.
type CoreCapabilityEscalationRule struct {
	Slug             string
	AgentSlug        string
	TriggerCondition string
	ThresholdValue   string
	Action           string
	IsActive         bool
	SortOrder        int
}

// LoadCapabilityEscalationRules returns all active escalation rule rows from
// ah_core.capability_escalation_rule WHERE agent_slug = ANY($1) AND
// is_active = TRUE, ordered by agent_slug, sort_order. Returns nil, nil when
// the table is not accessible (non-fatal).
func (l *CoreCapabilityEscalationRuleLoader) LoadCapabilityEscalationRules(ctx context.Context) ([]CoreCapabilityEscalationRule, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, trigger_condition, threshold_value, action, is_active, sort_order
		  FROM ah_core.capability_escalation_rule
		 WHERE agent_slug = ANY($1)
		   AND is_active = TRUE
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityEscalationRuleAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_escalation_rule not accessible, escalation rules unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability escalation rules: %w", err)
	}
	defer rows.Close()

	var rules []CoreCapabilityEscalationRule
	for rows.Next() {
		var r CoreCapabilityEscalationRule
		if err := rows.Scan(
			&r.Slug, &r.AgentSlug, &r.TriggerCondition, &r.ThresholdValue,
			&r.Action, &r.IsActive, &r.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability escalation rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_escalation_rule not accessible (post-iter), escalation rules unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability escalation rules: %w", err)
	}
	return rules, nil
}

// LoadEscalationRulesForAgent returns the active escalation rule rows from
// ah_core.capability_escalation_rule WHERE agent_slug = $1 AND
// is_active = TRUE, ordered by sort_order. Returns nil, nil when the table
// is not accessible (non-fatal).
func (l *CoreCapabilityEscalationRuleLoader) LoadEscalationRulesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityEscalationRule, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, trigger_condition, threshold_value, action, is_active, sort_order
		  FROM ah_core.capability_escalation_rule
		 WHERE agent_slug = $1
		   AND is_active = TRUE
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_escalation_rule not accessible, escalation rules unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query escalation rules for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var rules []CoreCapabilityEscalationRule
	for rows.Next() {
		var r CoreCapabilityEscalationRule
		if err := rows.Scan(
			&r.Slug, &r.AgentSlug, &r.TriggerCondition, &r.ThresholdValue,
			&r.Action, &r.IsActive, &r.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan escalation rule for agent %q: %w", agentSlug, err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_escalation_rule not accessible (post-iter), escalation rules unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate escalation rules for agent %q: %w", agentSlug, err)
	}
	return rules, nil
}

// ============================================================
// Seed catalog constants — migration 000112 (2026-05-11).
// ============================================================

// SeedCapabilityEscalationRuleCount is the expected total row count after
// migration 000112. Nine escalation rule rows — three per capability agent
// (researcher, analyst, planner), each agent having rules for confidence
// threshold, tool failure, and clarification scenarios.
const SeedCapabilityEscalationRuleCount = 9

// SeedCapabilityEscalationRuleAgentSlugs is the canonical list of capability
// agent slugs that have seeded escalation rules in migration 000112.
var SeedCapabilityEscalationRuleAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherEscalationRuleCount is the number of escalation rules seeded
// for core-researcher (3: low_source_confidence + consecutive_tool_failures +
// out_of_scope_request).
const SeedResearcherEscalationRuleCount = 3

// SeedAnalystEscalationRuleCount is the number of escalation rules seeded for
// core-analyst (3: low_analysis_confidence + ambiguous_document_content +
// consecutive_tool_failures).
const SeedAnalystEscalationRuleCount = 3

// SeedPlannerEscalationRuleCount is the number of escalation rules seeded for
// core-planner (3: ambiguous_goal + dependency_conflict_detected +
// consecutive_tool_failures).
const SeedPlannerEscalationRuleCount = 3

// Trigger condition constants — migration 000112.

// SeedEscalationTriggerLowSourceConfidence is the trigger_condition value for
// when retrieved sources fall below the researcher's minimum confidence
// threshold before proceeding with a response.
const SeedEscalationTriggerLowSourceConfidence = "low_source_confidence"

// SeedEscalationTriggerLowAnalysisConfidence is the trigger_condition value
// for when the analyst's confidence in its conclusions falls below the
// minimum threshold. Threshold is higher (0.5) than researcher (0.4) because
// analytical errors are harder to detect post-hoc.
const SeedEscalationTriggerLowAnalysisConfidence = "low_analysis_confidence"

// SeedEscalationTriggerConsecutiveToolFailures is the trigger_condition value
// for when tool calls fail in succession beyond the per-agent retry budget.
const SeedEscalationTriggerConsecutiveToolFailures = "consecutive_tool_failures"

// SeedEscalationTriggerOutOfScopeRequest is the trigger_condition value for
// when the user's request falls outside the researcher's configured topic scope.
const SeedEscalationTriggerOutOfScopeRequest = "out_of_scope_request"

// SeedEscalationTriggerAmbiguousDocContent is the trigger_condition value for
// when document content is too ambiguous for the analyst to interpret safely
// without user clarification.
const SeedEscalationTriggerAmbiguousDocContent = "ambiguous_document_content"

// SeedEscalationTriggerAmbiguousGoal is the trigger_condition value for when
// the planner cannot decompose a goal into concrete tasks because the goal is
// under-specified.
const SeedEscalationTriggerAmbiguousGoal = "ambiguous_goal"

// SeedEscalationTriggerDependencyConflict is the trigger_condition value for
// when the planner detects a conflict in task dependencies that requires
// human resolution.
const SeedEscalationTriggerDependencyConflict = "dependency_conflict_detected"

// Action constants — migration 000112.

// SeedEscalationActionPauseAndAsk is the action taken when an escalation rule
// fires and the agent needs the user to provide direction before continuing.
// This is the most common action across all three agents.
const SeedEscalationActionPauseAndAsk = "pause_and_ask"

// SeedEscalationActionClarifyScope is the action taken when the researcher
// receives an out-of-scope request and needs the user to confirm or adjust
// the research boundaries.
const SeedEscalationActionClarifyScope = "clarify_scope"

// SeedEscalationActionClarifyIntent is the action taken when the analyst
// encounters ambiguous document content and needs the user to confirm the
// interpretation intent.
const SeedEscalationActionClarifyIntent = "clarify_intent"

// SeedEscalationActionClarifyGoal is the action taken when the planner
// detects an ambiguous or under-specified goal and needs the user to provide
// a more concrete objective before task decomposition can proceed.
const SeedEscalationActionClarifyGoal = "clarify_goal"

// SeedAllEscalationRulesActive signals that all 9 seeded escalation rules
// have is_active = TRUE by default. Rules may be disabled by operators after
// deployment to relax escalation behaviour for specific agents.
const SeedAllEscalationRulesActive = true

// Threshold constants for notable seed values — migration 000112.

// SeedResearcherLowConfidenceThreshold is the threshold_value for the
// researcher's low_source_confidence escalation rule ("0.4"). Sources with
// confidence below 40% trigger escalation.
const SeedResearcherLowConfidenceThreshold = "0.4"

// SeedAnalystLowConfidenceThreshold is the threshold_value for the analyst's
// low_analysis_confidence escalation rule ("0.5"). Analyst applies a higher
// bar than the researcher — conclusions below 50% confidence trigger
// escalation.
const SeedAnalystLowConfidenceThreshold = "0.5"
