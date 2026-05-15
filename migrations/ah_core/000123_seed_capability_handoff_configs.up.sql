-- Seed capability_handoff_config rows in ah_core (migration 000123).
-- These 9 rows encode the per-agent handoff configuration for the three core
-- capability agents: when and how each agent hands off to another agent or
-- escalates to a human — including the trigger condition, target agent slug,
-- handoff message, and whether the handoff fires automatically.
--
-- core-researcher (3 rows — hands off beyond research scope):
--   needs_analysis     → target: core-analyst    (is_automatic=false)
--   needs_planning     → target: core-planner    (is_automatic=false)
--   human_escalation   → target: ''              (is_automatic=true)
--
-- core-analyst (3 rows — hands off beyond analysis scope):
--   needs_research     → target: core-researcher (is_automatic=false)
--   needs_planning     → target: core-planner    (is_automatic=false)
--   human_escalation   → target: ''              (is_automatic=true)
--
-- core-planner (3 rows — hands off when execution or research is needed):
--   needs_research     → target: core-researcher (is_automatic=false)
--   needs_analysis     → target: core-analyst    (is_automatic=false)
--   human_escalation   → target: ''              (is_automatic=true)
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; handoff identity
--     is (agent_slug, handoff_key) enforced by UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this handoff row
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - handoff_key TEXT NOT NULL: machine-readable key for the handoff condition
--     (e.g. "needs_analysis", "needs_planning", "human_escalation").
--   - trigger_condition TEXT NOT NULL: machine-readable event that triggers this
--     handoff (e.g. "user_asks_for_data_analysis").
--   - target_agent_slug TEXT NOT NULL DEFAULT '': slug of the target agent to
--     hand off to. Empty string ('') means human escalation — no target agent.
--   - handoff_message TEXT NOT NULL: user-facing message sent when handoff fires.
--   - is_automatic BOOLEAN NOT NULL DEFAULT FALSE: when true the handoff fires
--     without explicit user confirmation (used for human_escalation rows).
--   - ON CONFLICT (agent_slug, handoff_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_handoff_config (
    id               BIGSERIAL   NOT NULL,
    agent_slug       TEXT        NOT NULL,
    handoff_key      TEXT        NOT NULL,
    trigger_condition TEXT       NOT NULL,
    target_agent_slug TEXT       NOT NULL DEFAULT '',
    handoff_message  TEXT        NOT NULL,
    is_automatic     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_handoff_config PRIMARY KEY (id),
    CONSTRAINT uq_capability_handoff_config_agent_key UNIQUE (agent_slug, handoff_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_handoff_config_agent_slug
    ON ah_core.capability_handoff_config (agent_slug);

-- =====================================
-- 9 capability handoff config rows
-- =====================================
INSERT INTO ah_core.capability_handoff_config
    (agent_slug, handoff_key, trigger_condition, target_agent_slug, handoff_message, is_automatic)
VALUES
    -- core-researcher (hands off when task goes beyond research)
    ('core-researcher',
     'needs_analysis',
     'user_asks_for_data_analysis',
     'core-analyst',
     'This looks like an analysis task. Let me connect you with the Analyst agent who specializes in breaking down data.',
     FALSE),
    ('core-researcher',
     'needs_planning',
     'user_asks_for_project_plan',
     'core-planner',
     'Planning is better handled by our Planner agent. Transferring you now.',
     FALSE),
    ('core-researcher',
     'human_escalation',
     'information_cannot_be_found',
     '',
     'I was unable to find reliable information on this topic. Please consult a human expert or provide additional context.',
     TRUE),

    -- core-analyst (hands off when task goes beyond analysis)
    ('core-analyst',
     'needs_research',
     'insufficient_data_for_analysis',
     'core-researcher',
     'I need more data to complete this analysis. The Researcher agent can gather what''s needed.',
     FALSE),
    ('core-analyst',
     'needs_planning',
     'analysis_complete_plan_needed',
     'core-planner',
     'Analysis complete. The Planner agent can help turn these insights into an actionable plan.',
     FALSE),
    ('core-analyst',
     'human_escalation',
     'analysis_requires_domain_expertise',
     '',
     'This analysis requires domain expertise beyond my capabilities. Please involve a human specialist.',
     TRUE),

    -- core-planner (hands off when execution or research is needed)
    ('core-planner',
     'needs_research',
     'plan_requires_more_information',
     'core-researcher',
     'I need more information before finalizing this plan. Routing to the Researcher.',
     FALSE),
    ('core-planner',
     'needs_analysis',
     'plan_requires_data_analysis',
     'core-analyst',
     'This plan requires data analysis. Connecting you with the Analyst.',
     FALSE),
    ('core-planner',
     'human_escalation',
     'plan_exceeds_agent_authority',
     '',
     'This plan requires decisions beyond my authority. Please review with a human decision-maker.',
     TRUE)

ON CONFLICT (agent_slug, handoff_key) DO NOTHING;
