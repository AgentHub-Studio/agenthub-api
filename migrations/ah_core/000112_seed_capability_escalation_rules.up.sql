-- Seed capability escalation rule rows in ah_core
-- (migration 000112). These 9 rows encode per-agent escalation rules that
-- define when capability agents should stop acting autonomously and escalate
-- to the user for guidance. Three rules per agent (researcher, analyst,
-- planner), adapted from Claude Code's abort/interrupt patterns (§4.5):
--
-- Researcher (3 rules):
--   esc-researcher-low-confidence   — low_source_confidence    threshold 0.4 → pause_and_ask
--   esc-researcher-tool-failure     — consecutive_tool_failures threshold 3   → pause_and_ask
--   esc-researcher-out-of-scope     — out_of_scope_request      threshold 1   → clarify_scope
--
-- Analyst (3 rules):
--   esc-analyst-low-confidence      — low_analysis_confidence   threshold 0.5 → pause_and_ask
--   esc-analyst-ambiguous-doc       — ambiguous_document_content threshold 1  → clarify_intent
--   esc-analyst-tool-failure        — consecutive_tool_failures  threshold 2  → pause_and_ask
--
-- Planner (3 rules):
--   esc-planner-ambiguous-goal      — ambiguous_goal             threshold 1   → clarify_goal
--   esc-planner-dependency-conflict — dependency_conflict_detected threshold 1 → pause_and_ask
--   esc-planner-tool-failure        — consecutive_tool_failures   threshold 3  → pause_and_ask
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - agent_slug VARCHAR(120): the capability agent this rule applies to.
--   - trigger_condition VARCHAR(120): machine-readable event that fires escalation.
--   - threshold_value VARCHAR(50): numeric or count threshold (stored as string for flexibility).
--   - action VARCHAR(80): what to do when the rule fires.
--   - is_active BOOLEAN: allows disabling a rule without deleting it.
--   - sort_order INTEGER: evaluation order within each agent's rule set.
--   - UNIQUE (agent_slug, trigger_condition): prevents duplicate conditions per agent.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).
--   - Analyst confidence threshold (0.5) is higher than researcher (0.4) — analysts
--     require stronger source confidence before proceeding without user confirmation.

CREATE TABLE IF NOT EXISTS ah_core.capability_escalation_rule (
    slug                VARCHAR(120) PRIMARY KEY,
    agent_slug          VARCHAR(120) NOT NULL,
    trigger_condition   VARCHAR(120) NOT NULL,
    threshold_value     VARCHAR(50)  NOT NULL,
    action              VARCHAR(80)  NOT NULL,
    is_active           BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order          INTEGER      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, trigger_condition)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_escalation_rule_agent_slug
    ON ah_core.capability_escalation_rule (agent_slug);

-- ==============================
-- 9 capability escalation rule rows
-- ==============================
INSERT INTO ah_core.capability_escalation_rule
    (slug, agent_slug, trigger_condition, threshold_value, action, is_active, sort_order)
VALUES
    -- Researcher escalation rules
    ('esc-researcher-low-confidence',
     'core-researcher', 'low_source_confidence', '0.4',
     'pause_and_ask', TRUE, 1),
    ('esc-researcher-tool-failure',
     'core-researcher', 'consecutive_tool_failures', '3',
     'pause_and_ask', TRUE, 2),
    ('esc-researcher-out-of-scope',
     'core-researcher', 'out_of_scope_request', '1',
     'clarify_scope', TRUE, 3),

    -- Analyst escalation rules
    ('esc-analyst-low-confidence',
     'core-analyst', 'low_analysis_confidence', '0.5',
     'pause_and_ask', TRUE, 1),
    ('esc-analyst-ambiguous-doc',
     'core-analyst', 'ambiguous_document_content', '1',
     'clarify_intent', TRUE, 2),
    ('esc-analyst-tool-failure',
     'core-analyst', 'consecutive_tool_failures', '2',
     'pause_and_ask', TRUE, 3),

    -- Planner escalation rules
    ('esc-planner-ambiguous-goal',
     'core-planner', 'ambiguous_goal', '1',
     'clarify_goal', TRUE, 1),
    ('esc-planner-dependency-conflict',
     'core-planner', 'dependency_conflict_detected', '1',
     'pause_and_ask', TRUE, 2),
    ('esc-planner-tool-failure',
     'core-planner', 'consecutive_tool_failures', '3',
     'pause_and_ask', TRUE, 3)

ON CONFLICT (slug) DO NOTHING;
