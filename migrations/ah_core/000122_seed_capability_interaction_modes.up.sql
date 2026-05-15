-- Seed capability_interaction_mode rows in ah_core (migration 000122).
-- These 9 rows encode the per-agent interaction modes for the three core
-- capability agents: how each agent prefers to interact — single-turn Q&A vs
-- multi-turn dialogue vs task-execution — and whether it proactively asks
-- questions or waits for user direction.
--
-- core-researcher (3 rows — single-turn, low-proactive, structured_summary):
--   primary_mode       → single_turn         (responds in one comprehensive reply per query)
--   proactive_questions → low                (proceeds with reasonable assumptions)
--   response_style     → structured_summary  (findings as structured summaries with citations)
--
-- core-analyst (3 rows — multi-turn, high-proactive, step_by_step):
--   primary_mode       → multi_turn    (asks for data, analyzes, presents, refines)
--   proactive_questions → high         (actively asks when data or goals are ambiguous)
--   response_style     → step_by_step  (walks through reasoning step by step)
--
-- core-planner (3 rows — task_execution, high-proactive, checklist):
--   primary_mode       → task_execution  (decompose, plan, delegate, track)
--   proactive_questions → high           (asks clarifying questions upfront before planning)
--   response_style     → checklist       (actionable checklists and numbered step sequences)
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; mode identity
--     is (agent_slug, mode_key) enforced by UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this mode row
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - mode_key TEXT NOT NULL: machine-readable key for the interaction dimension
--     (e.g. "primary_mode", "proactive_questions", "response_style").
--   - mode_value TEXT NOT NULL: the value for this dimension
--     (e.g. "single_turn", "high", "checklist").
--   - description TEXT NOT NULL: human-readable explanation of this mode setting.
--   - ON CONFLICT (agent_slug, mode_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_interaction_mode (
    id          BIGSERIAL   NOT NULL,
    agent_slug  TEXT        NOT NULL,
    mode_key    TEXT        NOT NULL,
    mode_value  TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_interaction_mode PRIMARY KEY (id),
    CONSTRAINT uq_capability_interaction_mode_agent_key UNIQUE (agent_slug, mode_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_interaction_mode_agent_slug
    ON ah_core.capability_interaction_mode (agent_slug);

-- =====================================
-- 9 capability interaction mode rows
-- =====================================
INSERT INTO ah_core.capability_interaction_mode
    (agent_slug, mode_key, mode_value, description)
VALUES
    -- core-researcher (single-turn, low-proactive, structured_summary)
    ('core-researcher',
     'primary_mode',
     'single_turn',
     'Researcher typically responds in one comprehensive reply per query'),
    ('core-researcher',
     'proactive_questions',
     'low',
     'Researcher proceeds with reasonable assumptions rather than asking many clarifying questions'),
    ('core-researcher',
     'response_style',
     'structured_summary',
     'Researcher delivers findings as structured summaries with source citations'),

    -- core-analyst (multi-turn, high-proactive, step_by_step)
    ('core-analyst',
     'primary_mode',
     'multi_turn',
     'Analyst engages in dialogue: asks for data, analyzes, presents, refines based on feedback'),
    ('core-analyst',
     'proactive_questions',
     'high',
     'Analyst actively asks for clarification when data or goals are ambiguous before proceeding'),
    ('core-analyst',
     'response_style',
     'step_by_step',
     'Analyst walks through reasoning step by step, showing work before conclusions'),

    -- core-planner (task_execution, high-proactive, checklist)
    ('core-planner',
     'primary_mode',
     'task_execution',
     'Planner focuses on decomposing and executing: gather context, plan, delegate, track'),
    ('core-planner',
     'proactive_questions',
     'high',
     'Planner asks clarifying questions upfront before creating plans to avoid rework'),
    ('core-planner',
     'response_style',
     'checklist',
     'Planner presents output as actionable checklists and numbered step sequences')

ON CONFLICT (agent_slug, mode_key) DO NOTHING;
