-- Seed capability tool permission rows in ah_core
-- (migration 000118). These 12 rows encode per-agent default tool permission
-- declarations — which tools each agent is allowed to invoke, with one of three
-- permission modes (allow, deny, require_approval). The design is adapted from
-- the Claude Code permission mode system and gives operators fine-grained control
-- over what each capability agent can do autonomously vs. with human approval.
--
-- Researcher (4 rows):
--   core-web-search    → allow          (primary capability: search the web)
--   core-web-fetch     → allow          (secondary: fetch full web page content)
--   core-doc-search    → allow          (fallback: search tenant knowledge base)
--   core-subagent-run  → require_approval (delegation needs explicit user approval)
--
-- Analyst (4 rows):
--   core-doc-search    → allow          (primary: search documents for data/patterns)
--   core-web-search    → allow          (secondary: supplement with web data)
--   core-web-fetch     → allow          (fetch external data sources when referenced)
--   core-subagent-run  → deny           (single-thread analysis; no delegation)
--
-- Planner (4 rows):
--   core-subagent-run  → allow          (primary: delegates subtasks to specialised agents)
--   core-doc-search    → allow          (consult project documentation when planning)
--   core-web-search    → require_approval (web search requires approval to stay focused)
--   core-web-fetch     → deny           (planner does not fetch external content directly)
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; tool permission
--     identity is (agent_slug, tool_slug) enforced by the UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this permission
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - tool_slug TEXT NOT NULL: the slug of the tool being governed
--     (e.g. "core-web-search", "core-web-fetch", "core-doc-search",
--     "core-subagent-run").
--   - permission_mode TEXT NOT NULL CHECK: one of 'allow', 'deny',
--     'require_approval'. Determines whether the agent may invoke the tool
--     autonomously (allow), never (deny), or only after the user confirms
--     (require_approval).
--   - rationale TEXT NOT NULL DEFAULT '': human-readable explanation of why this
--     permission mode was chosen for this agent/tool pair.
--   - ON CONFLICT (agent_slug, tool_slug) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_tool_permission (
    id              BIGSERIAL    NOT NULL,
    agent_slug      TEXT         NOT NULL,
    tool_slug       TEXT         NOT NULL,
    permission_mode TEXT         NOT NULL CHECK (permission_mode IN ('allow', 'deny', 'require_approval')),
    rationale       TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_tool_permission PRIMARY KEY (id),
    CONSTRAINT uq_capability_tool_permission_agent_tool UNIQUE (agent_slug, tool_slug)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_tool_permission_agent_slug
    ON ah_core.capability_tool_permission (agent_slug);

-- ==============================
-- 12 capability tool permission rows
-- ==============================
INSERT INTO ah_core.capability_tool_permission
    (agent_slug, tool_slug, permission_mode, rationale)
VALUES
    -- core-researcher permissions (web-focused, read-only tools)
    ('core-researcher',
     'core-web-search',
     'allow',
     'Researcher primary capability: search the web for information'),
    ('core-researcher',
     'core-web-fetch',
     'allow',
     'Researcher secondary capability: fetch full web page content'),
    ('core-researcher',
     'core-doc-search',
     'allow',
     'Researcher fallback: search tenant knowledge base documents'),
    ('core-researcher',
     'core-subagent-run',
     'require_approval',
     'Subagent delegation requires explicit user approval for researcher'),

    -- core-analyst permissions (analysis-focused, data tools)
    ('core-analyst',
     'core-doc-search',
     'allow',
     'Analyst primary: search documents for data and patterns'),
    ('core-analyst',
     'core-web-search',
     'allow',
     'Analyst secondary: supplement with web data when needed'),
    ('core-analyst',
     'core-web-fetch',
     'allow',
     'Analyst: fetch external data sources when referenced'),
    ('core-analyst',
     'core-subagent-run',
     'deny',
     'Analyst does not delegate; maintains single-thread analysis focus'),

    -- core-planner permissions (planning-focused, coordination tools)
    ('core-planner',
     'core-subagent-run',
     'allow',
     'Planner primary: delegates subtasks to specialized agents'),
    ('core-planner',
     'core-doc-search',
     'allow',
     'Planner: consults project documentation when planning'),
    ('core-planner',
     'core-web-search',
     'require_approval',
     'Planner web search requires approval to stay focused on provided context'),
    ('core-planner',
     'core-web-fetch',
     'deny',
     'Planner does not fetch external content directly')

ON CONFLICT (agent_slug, tool_slug) DO NOTHING;
