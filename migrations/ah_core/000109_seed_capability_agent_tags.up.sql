-- Seed capability agent tag rows in ah_core
-- (migration 000109). These 12 rows define searchable tags per capability agent,
-- enabling tag-based agent discovery in the marketplace and catalog UI. Four tags
-- per agent are seeded here:
--
-- Researcher (4 tags):
--   tag-researcher-research   — "research"
--   tag-researcher-web        — "web"
--   tag-researcher-sources    — "sources"
--   tag-researcher-knowledge  — "knowledge"
--
-- Analyst (4 tags):
--   tag-analyst-analysis      — "analysis"
--   tag-analyst-documents     — "documents"
--   tag-analyst-insights      — "insights"
--   tag-analyst-patterns      — "patterns"
--
-- Planner (4 tags):
--   tag-planner-planning      — "planning"
--   tag-planner-tasks         — "tasks"
--   tag-planner-projects      — "projects"
--   tag-planner-workflows     — "workflows"
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - agent_slug VARCHAR(120): the capability agent this tag belongs to.
--   - tag VARCHAR(80): lowercase searchable tag value.
--   - sort_order INTEGER: display order within each agent's tag list.
--   - UNIQUE (agent_slug, tag): enforces one entry per agent+tag combination.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_tag (
    slug        VARCHAR(120) PRIMARY KEY,
    agent_slug  VARCHAR(120) NOT NULL,
    tag         VARCHAR(80)  NOT NULL,
    sort_order  INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, tag)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_agent_tag_agent_slug
    ON ah_core.capability_agent_tag (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_agent_tag_tag
    ON ah_core.capability_agent_tag (tag);

-- ==============================
-- 12 capability agent tag rows
-- ==============================
INSERT INTO ah_core.capability_agent_tag
    (slug, agent_slug, tag, sort_order)
VALUES
    -- Researcher tags
    ('tag-researcher-research',  'core-researcher', 'research',  1),
    ('tag-researcher-web',       'core-researcher', 'web',       2),
    ('tag-researcher-sources',   'core-researcher', 'sources',   3),
    ('tag-researcher-knowledge', 'core-researcher', 'knowledge', 4),

    -- Analyst tags
    ('tag-analyst-analysis',     'core-analyst', 'analysis',  1),
    ('tag-analyst-documents',    'core-analyst', 'documents', 2),
    ('tag-analyst-insights',     'core-analyst', 'insights',  3),
    ('tag-analyst-patterns',     'core-analyst', 'patterns',  4),

    -- Planner tags
    ('tag-planner-planning',     'core-planner', 'planning',   1),
    ('tag-planner-tasks',        'core-planner', 'tasks',      2),
    ('tag-planner-projects',     'core-planner', 'projects',   3),
    ('tag-planner-workflows',    'core-planner', 'workflows',  4)

ON CONFLICT (slug) DO NOTHING;
