-- Seed capability knowledge source rows in ah_core
-- (migration 000117). These 9 rows encode per-agent default knowledge source
-- declarations — declarative hints about where each capability agent looks
-- for information when fulfilling a request. Each agent has three priority
-- levels (primary, secondary, fallback) that guide source selection at runtime.
--
-- Researcher (3 rows):
--   primary   → web_search / web_search_index (live search engine index)
--   secondary → document_fetch / web_page_content (full page content)
--   fallback  → knowledge_base / tenant_knowledge_base (uploaded documents)
--
-- Analyst (3 rows):
--   primary   → knowledge_base / tenant_knowledge_base (structured tenant data)
--   secondary → conversation_context / session_context (data in conversation)
--   fallback  → web_search / web_search_index (web search backup)
--
-- Planner (3 rows):
--   primary   → conversation_context / session_context (requirements from user)
--   secondary → knowledge_base / tenant_knowledge_base (project docs)
--   fallback  → web_search / web_search_index (technical reference)
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; source identity
--     is (agent_slug, source_key) enforced by the UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this source
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - source_key TEXT NOT NULL: the logical priority slot ("primary",
--     "secondary", "fallback").
--   - source_type TEXT NOT NULL: the category of source access mechanism
--     (e.g. "web_search", "document_fetch", "knowledge_base",
--     "conversation_context").
--   - source_ref TEXT NOT NULL: the specific source reference within the type
--     (e.g. "web_search_index", "web_page_content", "tenant_knowledge_base",
--     "session_context").
--   - priority INTEGER NOT NULL DEFAULT 1: numeric priority; 1 = highest.
--     Maps to primary=1, secondary=2, fallback=3.
--   - description TEXT NOT NULL DEFAULT '': human-readable explanation of when
--     this source is used and what kind of information it provides.
--   - is_active BOOLEAN NOT NULL DEFAULT TRUE: whether this source declaration
--     is active. Inactive sources are skipped during source selection.
--   - ON CONFLICT (agent_slug, source_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_knowledge_source (
    id          BIGSERIAL    NOT NULL,
    agent_slug  TEXT         NOT NULL,
    source_key  TEXT         NOT NULL,
    source_type TEXT         NOT NULL,
    source_ref  TEXT         NOT NULL,
    priority    INTEGER      NOT NULL DEFAULT 1,
    description TEXT         NOT NULL DEFAULT '',
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_knowledge_source PRIMARY KEY (id),
    CONSTRAINT uq_capability_knowledge_source_agent_key UNIQUE (agent_slug, source_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_knowledge_source_agent_priority
    ON ah_core.capability_knowledge_source (agent_slug, priority);

-- ==============================
-- 9 capability knowledge source rows
-- ==============================
INSERT INTO ah_core.capability_knowledge_source
    (agent_slug, source_key, source_type, source_ref, priority, description)
VALUES
    -- core-researcher knowledge sources
    ('core-researcher',
     'primary',
     'web_search',
     'web_search_index',
     1,
     'Primary source: live web search via search engine index'),
    ('core-researcher',
     'secondary',
     'document_fetch',
     'web_page_content',
     2,
     'Secondary: fetch and read full web page content when search results are insufficient'),
    ('core-researcher',
     'fallback',
     'knowledge_base',
     'tenant_knowledge_base',
     3,
     'Fallback: tenant-uploaded knowledge base documents'),

    -- core-analyst knowledge sources
    ('core-analyst',
     'primary',
     'knowledge_base',
     'tenant_knowledge_base',
     1,
     'Primary: structured documents and data uploaded by tenant'),
    ('core-analyst',
     'secondary',
     'conversation_context',
     'session_context',
     2,
     'Secondary: data and files shared directly in the conversation'),
    ('core-analyst',
     'fallback',
     'web_search',
     'web_search_index',
     3,
     'Fallback: web search when tenant data is insufficient'),

    -- core-planner knowledge sources
    ('core-planner',
     'primary',
     'conversation_context',
     'session_context',
     1,
     'Primary: requirements and context provided directly in conversation'),
    ('core-planner',
     'secondary',
     'knowledge_base',
     'tenant_knowledge_base',
     2,
     'Secondary: project documentation in tenant knowledge base'),
    ('core-planner',
     'fallback',
     'web_search',
     'web_search_index',
     3,
     'Fallback: web search for technical reference when needed')

ON CONFLICT (agent_slug, source_key) DO NOTHING;
