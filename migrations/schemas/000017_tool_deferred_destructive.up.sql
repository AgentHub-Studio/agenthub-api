-- Add deferred loading and destructive flags to tools (inspired by Claude Code).
-- Also marks remaining read-only tools missed by migration 000014.

-- =====================================================================
-- 1. New columns: should_defer, is_destructive, search_hint
-- =====================================================================

-- should_defer: When true, the tool's full schema is not sent to the LLM
-- initially. Instead, only the tool name appears in a deferred-tools list.
-- The LLM must use the builtin tool_search to load the full schema before
-- calling. Reduces prompt size for agents with 30+ tools.
-- Inspired by Claude Code's shouldDefer flag (Tool.ts).
ALTER TABLE tool ADD COLUMN IF NOT EXISTS should_defer BOOLEAN NOT NULL DEFAULT FALSE;

-- is_destructive: Flags tools that perform irreversible operations (delete,
-- drop, overwrite). Used by permission rules to auto-require confirmation
-- even when mode is allow_edits.
-- Inspired by Claude Code's isDestructive per-tool flag (Tool.ts).
ALTER TABLE tool ADD COLUMN IF NOT EXISTS is_destructive BOOLEAN NOT NULL DEFAULT FALSE;

-- search_hint: Short keyword phrase (3-10 words) used by tool_search for
-- keyword matching when a tool is deferred. Helps the LLM find tools by
-- capability rather than name.
-- Inspired by Claude Code's searchHint per-tool string (Tool.ts).
ALTER TABLE tool ADD COLUMN IF NOT EXISTS search_hint VARCHAR(100);

-- =====================================================================
-- 2. Mark remaining read-only tools missed by migration 000014
-- =====================================================================

-- These GET tools were created in 000013 but not marked in 000014.
UPDATE tool SET read_only = TRUE WHERE name IN (
    'agenthub_get_agent',
    'agenthub_get_skill',
    'agenthub_get_tool',
    'agenthub_get_knowledge_base',
    'agenthub_get_settings'
) AND read_only = FALSE;

-- =====================================================================
-- 3. Mark destructive tools (delete, drop operations)
-- =====================================================================

UPDATE tool SET is_destructive = TRUE WHERE name IN (
    'agenthub_delete_agent',
    'agenthub_delete_skill',
    'agenthub_delete_tool',
    'agenthub_delete_knowledge_base'
);

-- =====================================================================
-- 4. Mark platform management tools as deferred
--    (reduces prompt size when AgentHub Assistant has 40+ tools)
-- =====================================================================

-- Only defer non-essential tools. Core tools (list, get) stay loaded.
-- Write tools (create, update, delete) can be loaded on demand via tool_search.
UPDATE tool SET should_defer = TRUE WHERE name IN (
    'agenthub_create_agent',
    'agenthub_update_agent',
    'agenthub_delete_agent',
    'agenthub_publish_agent',
    'agenthub_clone_agent',
    'agenthub_create_skill',
    'agenthub_delete_skill',
    'agenthub_create_tool',
    'agenthub_delete_tool',
    'agenthub_test_tool',
    'agenthub_create_knowledge_base',
    'agenthub_delete_knowledge_base',
    'agenthub_update_settings',
    'agenthub_create_mcp_server',
    'agenthub_create_session',
    'agenthub_create_datasource',
    'agenthub_create_prompt_template',
    'agenthub_sync_agent_skills',
    'agenthub_sync_agent_kbs',
    'agenthub_create_agent_hook'
);

-- =====================================================================
-- 5. Add search hints for deferred tools
-- =====================================================================

UPDATE tool SET search_hint = 'create new agent assistant bot' WHERE name = 'agenthub_create_agent';
UPDATE tool SET search_hint = 'modify edit change agent' WHERE name = 'agenthub_update_agent';
UPDATE tool SET search_hint = 'remove delete agent permanently' WHERE name = 'agenthub_delete_agent';
UPDATE tool SET search_hint = 'publish activate agent draft' WHERE name = 'agenthub_publish_agent';
UPDATE tool SET search_hint = 'duplicate copy clone agent' WHERE name = 'agenthub_clone_agent';
UPDATE tool SET search_hint = 'create new skill capability' WHERE name = 'agenthub_create_skill';
UPDATE tool SET search_hint = 'remove delete skill' WHERE name = 'agenthub_delete_skill';
UPDATE tool SET search_hint = 'create new tool HTTP SQL' WHERE name = 'agenthub_create_tool';
UPDATE tool SET search_hint = 'remove delete tool' WHERE name = 'agenthub_delete_tool';
UPDATE tool SET search_hint = 'test execute tool dry-run' WHERE name = 'agenthub_test_tool';
UPDATE tool SET search_hint = 'create new knowledge base RAG' WHERE name = 'agenthub_create_knowledge_base';
UPDATE tool SET search_hint = 'remove delete knowledge base documents' WHERE name = 'agenthub_delete_knowledge_base';
UPDATE tool SET search_hint = 'change update tenant settings config' WHERE name = 'agenthub_update_settings';
UPDATE tool SET search_hint = 'create configure MCP server protocol' WHERE name = 'agenthub_create_mcp_server';
UPDATE tool SET search_hint = 'create new chat conversation session' WHERE name = 'agenthub_create_session';
UPDATE tool SET search_hint = 'create database datasource PostgreSQL MySQL' WHERE name = 'agenthub_create_datasource';
UPDATE tool SET search_hint = 'create prompt template system prompt' WHERE name = 'agenthub_create_prompt_template';
UPDATE tool SET search_hint = 'bind assign skills to agent' WHERE name = 'agenthub_sync_agent_skills';
UPDATE tool SET search_hint = 'bind assign knowledge bases to agent' WHERE name = 'agenthub_sync_agent_kbs';
UPDATE tool SET search_hint = 'create webhook hook event trigger' WHERE name = 'agenthub_create_agent_hook';
