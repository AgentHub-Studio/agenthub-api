-- Seed capability agent capability rows in ah_core
-- (migration 000119). These 12 rows encode per-agent declared capabilities —
-- what each capability agent can and cannot do — used by the frontend to show
-- capability badges and by the orchestrator to route requests to the right agent.
--
-- core-researcher (4 rows):
--   web_search        → is_supported=true   (primary: search the web for info)
--   document_analysis → is_supported=true   (secondary: read and analyze KB docs)
--   code_generation   → is_supported=false  (not a coder; research only)
--   task_planning     → is_supported=false  (not a planner; use the Planner agent)
--
-- core-analyst (4 rows):
--   data_analysis     → is_supported=true   (primary: analyze structured/unstructured data)
--   document_analysis → is_supported=true   (secondary: extract structured insights)
--   code_generation   → is_supported=false  (not a coder; analysis and interpretation only)
--   web_search        → is_supported=true   (supplement analysis with web data)
--
-- core-planner (4 rows):
--   task_planning       → is_supported=true  (primary: step-by-step plans for complex projects)
--   subagent_delegation → is_supported=true  (delegate subtasks to researcher/analyst)
--   code_generation     → is_supported=false (not a coder; use a coding agent instead)
--   web_search          → is_supported=true  (search the web for planning context)
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; agent capability
--     identity is (agent_slug, capability_key) enforced by the UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that declares this capability
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - capability_key TEXT NOT NULL: machine-readable capability identifier
--     (e.g. "web_search", "document_analysis", "code_generation").
--   - is_supported BOOLEAN NOT NULL DEFAULT TRUE: whether the agent supports this
--     capability. FALSE rows are displayed as "not supported" badges in the UI.
--   - display_label TEXT NOT NULL: human-readable label shown in the frontend badge.
--   - description TEXT NOT NULL DEFAULT '': full sentence explaining why the agent
--     supports or does not support this capability.
--   - display_order INTEGER NOT NULL DEFAULT 0: ascending sort order within an agent.
--   - ON CONFLICT (agent_slug, capability_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_capability (
    id            BIGSERIAL    NOT NULL,
    agent_slug    TEXT         NOT NULL,
    capability_key TEXT        NOT NULL,
    is_supported  BOOLEAN      NOT NULL DEFAULT TRUE,
    display_label TEXT         NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    display_order INTEGER      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_agent_capability PRIMARY KEY (id),
    CONSTRAINT uq_capability_agent_capability_agent_key UNIQUE (agent_slug, capability_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_agent_capability_agent_slug
    ON ah_core.capability_agent_capability (agent_slug);

-- ==============================
-- 12 capability agent capability rows
-- ==============================
INSERT INTO ah_core.capability_agent_capability
    (agent_slug, capability_key, is_supported, display_label, description, display_order)
VALUES
    -- core-researcher capabilities
    ('core-researcher',
     'web_search',
     TRUE,
     'Web Search',
     'Can search the web for current information and news',
     1),
    ('core-researcher',
     'document_analysis',
     TRUE,
     'Document Analysis',
     'Can read and analyze documents from the knowledge base',
     2),
    ('core-researcher',
     'code_generation',
     FALSE,
     'Code Generation',
     'Does not generate code; focused on research and information retrieval',
     3),
    ('core-researcher',
     'task_planning',
     FALSE,
     'Task Planning',
     'Does not create project plans; use the Planner agent instead',
     4),

    -- core-analyst capabilities
    ('core-analyst',
     'data_analysis',
     TRUE,
     'Data Analysis',
     'Can analyze structured and unstructured data systematically',
     1),
    ('core-analyst',
     'document_analysis',
     TRUE,
     'Document Analysis',
     'Can analyze documents and extract structured insights',
     2),
    ('core-analyst',
     'code_generation',
     FALSE,
     'Code Generation',
     'Does not generate code; focused on analysis and interpretation',
     3),
    ('core-analyst',
     'web_search',
     TRUE,
     'Web Search',
     'Can supplement analysis with web data when needed',
     4),

    -- core-planner capabilities
    ('core-planner',
     'task_planning',
     TRUE,
     'Task Planning',
     'Can create detailed step-by-step plans for complex projects',
     1),
    ('core-planner',
     'subagent_delegation',
     TRUE,
     'Agent Delegation',
     'Can delegate subtasks to specialized agents (researcher/analyst)',
     2),
    ('core-planner',
     'code_generation',
     FALSE,
     'Code Generation',
     'Does not generate code; use a coding agent for implementation',
     3),
    ('core-planner',
     'web_search',
     TRUE,
     'Web Search',
     'Can search the web when additional context is needed for planning',
     4)

ON CONFLICT (agent_slug, capability_key) DO NOTHING;
