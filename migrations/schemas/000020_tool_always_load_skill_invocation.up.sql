-- Add tool.always_load and skill.disable_model_invocation columns.
-- Inspired by Claude Code's alwaysLoad (Tool.ts) and disableModelInvocation (bundledSkills.ts).

-- =====================================================================
-- 1. tool.always_load — prevents deferred loading for critical tools
-- =====================================================================
-- When ToolSearch is active, tools with should_defer=true have their schemas
-- excluded from the initial prompt. always_load overrides this: the tool's
-- full schema is always included regardless of the deferred threshold.
-- Use for MCP tools that the LLM must see on turn 1 without a tool_search
-- round-trip.

ALTER TABLE tool
  ADD COLUMN IF NOT EXISTS always_load BOOLEAN NOT NULL DEFAULT FALSE;

-- =====================================================================
-- 2. skill.disable_model_invocation — user-only skills
-- =====================================================================
-- Skills with this flag are excluded from the LLM's tool list.
-- They can only be invoked by the user (via slash commands or UI).
-- This saves context tokens for skills that are diagnostic, administrative,
-- or only make sense when explicitly requested by the user.

ALTER TABLE skill
  ADD COLUMN IF NOT EXISTS disable_model_invocation BOOLEAN NOT NULL DEFAULT FALSE;

-- Mark diagnostic/admin skills as user-only.
UPDATE skill SET disable_model_invocation = TRUE
WHERE slug IN ('troubleshoot', 'prompt-template-management');

-- =====================================================================
-- 3. Update existing tools: always_load for critical platform tools
-- =====================================================================
-- Core listing tools should never be deferred — the LLM needs them
-- on turn 1 to understand what the user has available.

UPDATE tool SET always_load = TRUE
WHERE name IN (
    'agenthub_list_agents',
    'agenthub_list_skills',
    'agenthub_list_tools',
    'agenthub_list_knowledge_bases',
    'agenthub_list_mcp_servers',
    'agenthub_list_sessions'
);
