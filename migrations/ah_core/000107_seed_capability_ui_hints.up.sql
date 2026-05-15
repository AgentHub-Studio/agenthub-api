-- Seed capability UI hint rows in ah_core
-- (migration 000107). These 6 rows define contextual UI hints shown to new
-- users when working with capability agents in the AgentHub web UI. Hints are
-- adapted from Claude Code's in-app guidance patterns, covering three core
-- capability agents:
--
--   hint-researcher-start-tip      — agent_chat_start  tip  (core-researcher)
--   hint-researcher-citation-tip   — post_tool_result  info (core-researcher)
--   hint-analyst-doc-upload-tip    — agent_chat_start  tip  (core-analyst)
--   hint-analyst-confidence-tip    — post_tool_result  tip  (core-analyst)
--   hint-planner-task-tip          — agent_chat_start  info (core-planner)
--   hint-planner-breakdown-tip     — agent_chat_start  tip  (core-planner)
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - hint_type VARCHAR(40): 'tip' (actionable guidance) or 'info' (informational).
--   - trigger_context VARCHAR(120): when to surface the hint in the UI.
--   - agent_slug VARCHAR(120): NULL means global (agent-independent) hint.
--   - is_dismissable=TRUE for all 6: users can suppress hints they no longer need.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_ui_hint (
    slug             VARCHAR(120) PRIMARY KEY,
    title            VARCHAR(255) NOT NULL,
    body             TEXT         NOT NULL,
    trigger_context  VARCHAR(120) NOT NULL,
    agent_slug       VARCHAR(120),
    hint_type        VARCHAR(40)  NOT NULL DEFAULT 'tip',
    is_dismissable   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order       INTEGER      NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_ui_hint_agent_slug
    ON ah_core.capability_ui_hint (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_ui_hint_trigger_context
    ON ah_core.capability_ui_hint (trigger_context);

-- ==============================
-- 6 capability UI hint rows
-- ==============================
INSERT INTO ah_core.capability_ui_hint
    (slug, title, body, trigger_context, agent_slug, hint_type, is_dismissable, sort_order)
VALUES
    (
        'hint-researcher-start-tip',
        'Start with a clear research question',
        'Researcher works best when you provide a specific question rather than a broad topic. Example: "What are the latest methods for vector search in PostgreSQL?"',
        'agent_chat_start',
        'core-researcher',
        'tip',
        TRUE,
        1
    ),
    (
        'hint-researcher-citation-tip',
        'Sources are cited automatically',
        'Researcher automatically cites web sources in the response. You can ask follow-up questions about any cited source.',
        'post_tool_result',
        'core-researcher',
        'info',
        TRUE,
        2
    ),
    (
        'hint-analyst-doc-upload-tip',
        'Upload documents before analysis',
        'For best results, add documents to a Knowledge Base and assign it to this agent before starting analysis.',
        'agent_chat_start',
        'core-analyst',
        'tip',
        TRUE,
        3
    ),
    (
        'hint-analyst-confidence-tip',
        'Ask for confidence levels',
        'Analyst can rate its confidence in conclusions. Try asking: "How confident are you in this finding?"',
        'post_tool_result',
        'core-analyst',
        'tip',
        TRUE,
        4
    ),
    (
        'hint-planner-task-tip',
        'Tasks are tracked automatically',
        'Planner maintains a task list throughout the conversation. Ask "show me the current task list" at any time.',
        'agent_chat_start',
        'core-planner',
        'info',
        TRUE,
        5
    ),
    (
        'hint-planner-breakdown-tip',
        'Break down complex goals',
        'Planner works best with high-level goals. Example: "Plan the migration of our API from REST to GraphQL"',
        'agent_chat_start',
        'core-planner',
        'tip',
        TRUE,
        6
    )

ON CONFLICT (slug) DO NOTHING;
