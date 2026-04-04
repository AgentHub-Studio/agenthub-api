-- Migration 000027: Skill effort levels, hook type expansion, tool open-world/strict flags
--
-- Rationale:
--   Second batch of Claude Code patterns not yet in AgentHub after migration 026.
--   Analysis of CC source (skills/loadSkillsDir.ts, schemas/hooks.ts, Tool.ts, tools.ts)
--   revealed these high-value additions:
--
--   1. skill.effort — CC's effort field (low/medium/high/extreme or integer estimate).
--      Guides the runner on how many turns/tokens to budget for a skill invocation.
--      Example: 'compact' = low, 'skillify' = medium, batch-execute = high.
--
--   2. skill.model_override — Per-skill model override ('inherit' or model string).
--      Mirrors CC's BundledSkillDefinition.model: skills like 'compact' can force
--      a fast/cheap model (haiku) while 'api-explorer' uses the default.
--
--   3. skill.loaded_from — Source tracking for skill provenance.
--      CC tracks: bundled | mcp | plugin | managed | skills | commands_DEPRECATED.
--      AgentHub equivalent: seed | mcp | plugin | user | api.
--
--   4. agent_hook.hook_type expansion — CC has 4 hook command types:
--      command (bash), prompt (LLM), http (webhook), agent (subagent).
--      Current hook_type column only contains 'command'. Need 'prompt', 'http', 'agent'.
--      The config JSONB already handles type-specific fields (command, url, prompt text,
--      model) but the hook_type column needs a CHECK constraint and comment update.
--      Also adds: if_condition (CC's `if` field — permission-rule pattern to filter hook),
--      and shell (bash|powershell for command hooks).
--
--   5. tool.is_open_world — CC's isOpenWorld(): tools that accept arbitrary external
--      input (MCP tools, output tools). Security-relevant: open-world tools bypass
--      static input validation and may need extra scrutiny in permission checks.
--
--   6. tool.strict — CC's Tool.strict: when true, enables strict schema validation
--      at the API level. Currently false for all tools; set true for tools with
--      well-defined, closed input schemas (CRUD operations, not freeform search).
--
--   7. Populate effort for existing skills based on expected operation complexity.
--
--   8. Set is_open_world=true for MCP-backed and freeform tools.
--
--   9. New tool: agenthub_check_ollama_models (c089) — platform diagnostic.
--      Lets the assistant check which LLM models are loaded and available,
--      helping users understand why chat might not be generating responses.

-- =====================================================================
-- 1. SCHEMA: skill.effort — operation complexity estimate
-- =====================================================================

ALTER TABLE skill
    ADD COLUMN IF NOT EXISTS effort VARCHAR(20),
    ADD COLUMN IF NOT EXISTS model_override VARCHAR(100),
    ADD COLUMN IF NOT EXISTS loaded_from VARCHAR(30) NOT NULL DEFAULT 'seed';

COMMENT ON COLUMN skill.effort IS
    'Operation complexity estimate: ''low'', ''medium'', ''high'', ''extreme'', or integer hours. '
    'Guides the runner on token/turn budgets for this skill. '
    'NULL = unspecified (runner uses default budget). '
    'Inspired by CC''s BundledSkillDefinition.effort.';

COMMENT ON COLUMN skill.model_override IS
    'Override the session model for this skill. '
    '''inherit'' = use session model (same as NULL). '
    'Any model ID (e.g. ''claude-haiku-4-5-20251001'') forces that model. '
    'Useful for low-effort skills (compact, memory-recall) to use a cheaper model. '
    'Inspired by CC''s BundledSkillDefinition.model.';

COMMENT ON COLUMN skill.loaded_from IS
    'Source of this skill definition: '
    '''seed'' = built-in platform seed data (migrations), '
    '''mcp'' = loaded from MCP server, '
    '''plugin'' = installed via package registry, '
    '''user'' = created by tenant user via API, '
    '''api'' = created programmatically. '
    'Inspired by CC''s LoadedFrom type.';

-- =====================================================================
-- 2. SCHEMA: agent_hook — expanded hook types and matching
-- =====================================================================

ALTER TABLE agent_hook
    ADD COLUMN IF NOT EXISTS if_condition VARCHAR(200),
    ADD COLUMN IF NOT EXISTS shell        VARCHAR(20);

COMMENT ON COLUMN agent_hook.hook_type IS
    'Hook implementation type: '
    '''command'' — shell command (bash/powershell); config.command required. '
    '''prompt''  — LLM evaluates prompt; config.prompt required, config.model optional. '
    '''http''    — HTTP webhook; config.url required, config.headers optional. '
    '''agent''   — Sub-agent run; config.prompt required, config.model optional. '
    'Inspired by CC''s HookCommand union type.';

COMMENT ON COLUMN agent_hook.if_condition IS
    'Permission-rule pattern to filter hook activation. '
    'Uses same syntax as tool permission rules: '
    '''Bash(git *)'' — only fire when Bash tool is called with git commands. '
    '''Write(*.py)'' — only fire when Write tool targets .py files. '
    'NULL = fire on all matching events. '
    'Inspired by CC''s HookCommand.if field.';

COMMENT ON COLUMN agent_hook.shell IS
    'Shell interpreter for ''command'' type hooks: ''bash'' or ''powershell''. '
    'NULL defaults to ''bash''. '
    'Inspired by CC''s BashCommandHook.shell field.';

-- =====================================================================
-- 3. SCHEMA: tool — open-world and strict flags
-- =====================================================================

ALTER TABLE tool
    ADD COLUMN IF NOT EXISTS is_open_world BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS strict        BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN tool.is_open_world IS
    'When TRUE: tool accepts arbitrary external input (MCP tools, freeform output tools). '
    'May bypass static input validation; permission checks are more conservative. '
    'Inspired by CC''s Tool.isOpenWorld().';

COMMENT ON COLUMN tool.strict IS
    'When TRUE: API-level strict schema validation is enforced. '
    'Set true for closed-schema CRUD tools; false for freeform search/execution tools. '
    'Inspired by CC''s Tool.strict field.';

-- =====================================================================
-- 4. NEW TOOL: platform diagnostic (c089)
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer,
                  is_destructive, search_hint, always_load, concurrency_safe,
                  max_result_chars, is_search_or_read,
                  activity_description, tool_summary,
                  is_open_world, strict,
                  created_at, updated_at)
VALUES
-- c089: check available LLM models
('c1000000-0000-0000-0000-000000000089',
 'agenthub_list_llm_models', 'HTTP',
 'Lists LLM models currently available in the platform: Ollama models loaded locally, '
 'and any configured API providers (Anthropic, OpenAI, OpenRouter). '
 'Use when the user reports chat not working, to diagnose missing models. '
 'Also use when the user asks "which models do you have?" or wants to change the agent model. '
 'Returns: provider name, model ID, context window, and whether the model is currently loaded.',
 '{"method":"GET","url":"/api/settings/llm-models","useCallerToken":true,"inputSchema":{"type":"object","properties":{}}}',
 ARRAY['platform','settings','llm','models','read'],
 TRUE, FALSE, FALSE,
 'list llm models available ollama anthropic providers', FALSE, TRUE, 8000,
 TRUE,
 'Checking available models…', 'Found {count} models',
 FALSE, TRUE,
 NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 5. BIND c089 to settings-management skill
-- =====================================================================

INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000005', 'c1000000-0000-0000-0000-000000000089', 110, true, NOW())
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 6. POPULATE effort for existing skills
-- =====================================================================

-- Low effort: quick lookups, memory ops, compaction
UPDATE skill SET effort = 'low'
WHERE slug IN ('memory-recall', 'compact', 'troubleshoot');

-- Medium effort: CRUD management, API exploration, template work
UPDATE skill SET effort = 'medium'
WHERE slug IN (
    'skillify', 'api-explorer',
    'agent-management', 'skill-management', 'tool-management',
    'knowledge-base-management', 'chat-management', 'mcp-management',
    'settings-management', 'datasource-management', 'prompt-template-management'
);

-- High effort: batch operations, complex workflows
UPDATE skill SET effort = 'high'
WHERE slug IN ('batch-execute', 'review-agent');

-- =====================================================================
-- 7. SET model_override for lightweight skills
--    These skills do simple deterministic work — no need for the most
--    capable (and expensive) model in the session.
-- =====================================================================

UPDATE skill SET model_override = 'inherit'
WHERE slug IN ('compact', 'memory-recall', 'troubleshoot');

-- =====================================================================
-- 8. SET loaded_from for all seeded skills
--    All existing skills came from migration seed data.
-- =====================================================================

UPDATE skill SET loaded_from = 'seed'
WHERE loaded_from = 'seed';  -- already default, but make it explicit

-- =====================================================================
-- 9. SET strict=TRUE for well-defined CRUD tools
--    These tools have closed, machine-readable schemas (UUIDs, enums).
--    Strict mode prevents the LLM from passing extra fields.
-- =====================================================================

UPDATE tool SET strict = TRUE
WHERE name IN (
    -- Agent CRUD
    'agenthub_get_agent', 'agenthub_update_agent', 'agenthub_delete_agent',
    'agenthub_update_agent_status', 'agenthub_update_agent_system_prompt',
    -- Skill CRUD
    'agenthub_get_skill', 'agenthub_update_skill', 'agenthub_delete_skill',
    -- Tool CRUD
    'agenthub_get_tool', 'agenthub_update_tool', 'agenthub_delete_tool',
    -- KB CRUD
    'agenthub_get_knowledge_base', 'agenthub_update_knowledge_base',
    'agenthub_delete_knowledge_base', 'agenthub_sync_knowledge_base',
    -- Session CRUD
    'agenthub_archive_session', 'agenthub_update_session',
    -- Hooks CRUD
    'agenthub_delete_agent_hook', 'agenthub_update_agent_hook',
    -- MCP CRUD
    'agenthub_get_mcp_server', 'agenthub_update_mcp_server', 'agenthub_delete_mcp_server',
    -- Document CRUD
    'agenthub_get_document', 'agenthub_delete_document',
    -- Execution
    'agenthub_retry_execution',
    -- LLM Models
    'agenthub_list_llm_models'
);

-- =====================================================================
-- 10. SET is_open_world=TRUE for freeform/MCP-backed tools
-- =====================================================================

UPDATE tool SET is_open_world = TRUE
WHERE name IN (
    'agenthub_run_chat',          -- arbitrary message input
    'agenthub_search_documents',  -- freeform semantic query
    'agenthub_search_memories',   -- freeform semantic query
    'agenthub_upload_document',   -- arbitrary file content
    'agenthub_apply_prompt_template'  -- freeform merge into existing prompt
);

-- =====================================================================
-- 11. INDEX: effort (for budget-aware skill selection)
-- =====================================================================

CREATE INDEX IF NOT EXISTS idx_skill_effort ON skill (effort)
    WHERE effort IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tool_strict ON tool (id)
    WHERE strict = TRUE;
