-- Add tool.concurrency_safe, tool.max_result_chars, and skill.context_mode columns.
-- Inspired by Claude Code's per-tool isConcurrencySafe, maxResultSizeChars (Tool.ts),
-- and BundledSkillDefinition.context ('inline' | 'fork').

-- =====================================================================
-- 1. tool.concurrency_safe — allows parallel execution for non-read-only tools
-- =====================================================================
-- In Claude Code, isConcurrencySafe is independent of isReadOnly. A tool can
-- be non-read-only but still safe to run in parallel (e.g., tools that write
-- to different locations). When concurrency_safe is NULL, the system falls back
-- to the read_only flag for batch partitioning.

ALTER TABLE tool
  ADD COLUMN IF NOT EXISTS concurrency_safe BOOLEAN;

-- =====================================================================
-- 2. tool.max_result_chars — per-tool result size override
-- =====================================================================
-- Claude Code's maxResultSizeChars defaults to 50,000 chars per tool.
-- Tools like Read default to a higher limit (different from global).
-- This allows customization per tool without changing the global default.

ALTER TABLE tool
  ADD COLUMN IF NOT EXISTS max_result_chars INTEGER;

-- Set higher limits for tools that return large payloads.
UPDATE tool SET max_result_chars = 100000
WHERE name IN ('agenthub_list_messages', 'agenthub_get_execution_detail');

-- Set lower limits for listing tools (they return paginated summaries).
UPDATE tool SET max_result_chars = 20000
WHERE name IN (
    'agenthub_list_agents', 'agenthub_list_skills', 'agenthub_list_tools',
    'agenthub_list_knowledge_bases', 'agenthub_list_mcp_servers',
    'agenthub_list_sessions', 'agenthub_list_datasources',
    'agenthub_list_executions'
);

-- =====================================================================
-- 3. skill.context_mode — inline vs fork execution mode
-- =====================================================================
-- Inspired by Claude Code's BundledSkillDefinition.context:
--   'inline' — skill runs in the current conversation context
--   'fork' — skill runs in a sub-agent with its own context
-- Default is 'inline'. Fork mode is useful for diagnostic skills that
-- should not pollute the main conversation with tool call history.

ALTER TABLE skill
  ADD COLUMN IF NOT EXISTS context_mode VARCHAR(10) NOT NULL DEFAULT 'inline';

-- Diagnostic and optimization skills should fork to keep main context clean.
UPDATE skill SET context_mode = 'fork'
WHERE slug IN ('debug-agent', 'optimize-agent', 'health-check');

-- =====================================================================
-- 4. Mark concurrency-safe tools
-- =====================================================================
-- All read_only=true tools are implicitly concurrency-safe.
-- Additionally, memory search and document search are safe to run in parallel
-- even though they POST (write) data (the request is idempotent search).

UPDATE tool SET concurrency_safe = TRUE
WHERE name IN (
    'agenthub_search_memories',
    'agenthub_get_agent_skills',
    'agenthub_get_agent_kbs',
    'agenthub_get_kb_status',
    'agenthub_get_agent_stats'
);

-- =====================================================================
-- 5. Enrich search_hint for new tools from migration 000020
-- =====================================================================
-- Tools without search_hint are harder to find via tool_search.

UPDATE tool SET search_hint = 'list agent executions runs history status errors'
WHERE name = 'agenthub_list_executions' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'execution detail log errors tool calls tokens'
WHERE name = 'agenthub_get_execution_detail' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'agent statistics usage metrics success rate tokens'
WHERE name = 'agenthub_get_agent_stats' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'clone duplicate copy agent'
WHERE name = 'agenthub_clone_agent' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'bulk update agent status publish archive'
WHERE name = 'agenthub_bulk_agent_status' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'search find recall memories semantic'
WHERE name = 'agenthub_search_memories' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'store save remember persist memory'
WHERE name = 'agenthub_store_memory' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'delete remove forget memory'
WHERE name = 'agenthub_delete_memory' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'knowledge base indexing status documents pending'
WHERE name = 'agenthub_get_kb_status' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'agent skills bindings capabilities'
WHERE name = 'agenthub_get_agent_skills' AND search_hint IS NULL;

UPDATE tool SET search_hint = 'agent knowledge bases bindings documents'
WHERE name = 'agenthub_get_agent_kbs' AND search_hint IS NULL;
