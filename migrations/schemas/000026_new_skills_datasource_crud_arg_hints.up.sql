-- Migration 000025: datasource CRUD tools, review-agent/batch-execute/review-memory skills,
-- argument_hint for user-only skills, when_to_use for new skills.
--
-- Rationale:
--   1. datasource-management skill was missing CRUD tools (only had list).
--      Add get/create/update/delete/test tools for full datasource lifecycle.
--
--   2. Three new platform skills inspired by CC bundled skill patterns:
--      - review-agent: like CC's verify skill — review agent execution quality
--      - batch-execute: like CC's batch.ts — parallel sub-agent orchestration
--      - review-memory: like CC's remember.ts — audit/promote memory entries
--
--   3. argument_hint UPDATE for user-invocable skills so the ## User Commands
--      section in the system prompt shows proper usage hints (e.g. "/skill <agent_id>").
--      Inspired by CC's BundledSkillDefinition.argumentHint.
--
--   4. Two missing tools for agent-management: rollback_agent_version and
--      get_agent_version (detail view for a specific version).

-- =====================================================================
-- 1. DATASOURCE CRUD TOOLS (c071–c075)
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, read_only, should_defer, is_destructive,
                  search_hint, always_load, concurrency_safe, max_result_chars,
                  interrupt_behavior, is_search_or_read, created_at, updated_at)
VALUES
-- c071: get_datasource
('c1000000-0000-0000-0000-000000000071',
 'agenthub_get_datasource', 'HTTP',
 'Get full details of a specific datasource by ID. Returns type, host, port, database name, VPN binding, and status.',
 '{"method":"GET","urlTemplate":"/api/datasources/{datasource_id}"}',
 true, false, false,
 'get datasource details connection', false, true, 8000,
 NULL, true, NOW(), NOW()),

-- c072: create_datasource
('c1000000-0000-0000-0000-000000000072',
 'agenthub_create_datasource', 'HTTP',
 'Create a new database datasource connection (PostgreSQL, MySQL, or SQL Server). '
 'Requires name, type, host, port, database, db_user, and db_password. '
 'Optional vpn_resource_id for datasources behind a VPN.',
 '{"method":"POST","url":"/api/datasources"}',
 false, false, false,
 'create datasource connection database', false, false, 4000,
 NULL, false, NOW(), NOW()),

-- c073: update_datasource
('c1000000-0000-0000-0000-000000000073',
 'agenthub_update_datasource', 'HTTP',
 'Update an existing datasource connection. Accepts any combination of name, host, port, database, '
 'db_user, db_password, vpn_resource_id. ALWAYS confirm credential changes with the user first.',
 '{"method":"PATCH","urlTemplate":"/api/datasources/{datasource_id}"}',
 false, false, false,
 'update datasource connection credentials', false, false, 4000,
 NULL, false, NOW(), NOW()),

-- c074: delete_datasource
('c1000000-0000-0000-0000-000000000074',
 'agenthub_delete_datasource', 'HTTP',
 'Permanently delete a datasource connection. ALWAYS confirm with the user before executing. '
 'This is irreversible and will break any agents that depend on this datasource.',
 '{"method":"DELETE","urlTemplate":"/api/datasources/{datasource_id}"}',
 false, false, true,
 'delete datasource remove connection', false, false, 2000,
 'block', false, NOW(), NOW()),

-- c075: test_datasource
('c1000000-0000-0000-0000-000000000075',
 'agenthub_test_datasource', 'HTTP',
 'Test the connectivity of a datasource by ID. Attempts a connection and returns status, latency, '
 'and error details if the connection fails. Use to verify credentials after creating or updating.',
 '{"method":"POST","urlTemplate":"/api/datasources/{datasource_id}/test"}',
 true, false, false,
 'test datasource connection check', false, true, 4000,
 NULL, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 2. AGENT VERSION TOOLS (c076–c077)
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, read_only, should_defer, is_destructive,
                  search_hint, always_load, concurrency_safe, max_result_chars,
                  interrupt_behavior, is_search_or_read, created_at, updated_at)
VALUES
-- c076: get_agent_version
('c1000000-0000-0000-0000-000000000076',
 'agenthub_get_agent_version', 'HTTP',
 'Get details of a specific agent version including the snapshot of system_prompt, model_config, '
 'and bound skills at that point in time. Useful for comparing versions or rollback planning.',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/versions/{version_id}"}',
 true, false, false,
 'get agent version history snapshot', false, true, 10000,
 NULL, true, NOW(), NOW()),

-- c077: rollback_agent_version
('c1000000-0000-0000-0000-000000000077',
 'agenthub_rollback_agent_version', 'HTTP',
 'Rollback an agent to a specific previous version. Replaces current system_prompt and model_config '
 'with the snapshot from the target version. Creates a new version entry for the rollback. '
 'ALWAYS confirm with the user which version to roll back to before executing.',
 '{"method":"POST","urlTemplate":"/api/agents/{agent_id}/versions/{version_id}/rollback"}',
 false, false, false,
 'rollback agent version restore', false, false, 4000,
 'block', false, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 3. BIND NEW DATASOURCE TOOLS TO datasource-management SKILL
--    skill ID: b1000000-0000-0000-0001-000000000008 (datasource-management)
-- =====================================================================

INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000008', 'c1000000-0000-0000-0000-000000000071', 101, true, NOW()),  -- get_datasource
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000008', 'c1000000-0000-0000-0000-000000000072', 102, true, NOW()),  -- create_datasource
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000008', 'c1000000-0000-0000-0000-000000000073', 103, true, NOW()),  -- update_datasource
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000008', 'c1000000-0000-0000-0000-000000000074', 104, true, NOW()),  -- delete_datasource
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000008', 'c1000000-0000-0000-0000-000000000075', 105, true, NOW())   -- test_datasource
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 4. BIND VERSION TOOLS TO agent-management SKILL
--    skill ID: b1000000-0000-0000-0001-000000000001
-- =====================================================================

INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000076', 125, true, NOW()),  -- get_agent_version
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000077', 126, true, NOW())   -- rollback_agent_version
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 5. THREE NEW PLATFORM SKILLS
-- =====================================================================

INSERT INTO skill (id, name, slug, description, category, input_schema,
                   allowed_tools, disable_model_invocation, context_mode,
                   when_to_use, argument_hint, created_at, updated_at)
VALUES

-- review-agent: like CC's verify skill — deep quality review of an agent
('b1000000-0000-0000-0002-000000000007',
 'Review Agent', 'review-agent',
 'Performs a deep quality review of an agent: analyzes recent execution success/failure rates, '
 'checks system prompt clarity and completeness, verifies skill and KB bindings are appropriate, '
 'and produces a structured improvement report. '
 'Use when the user asks to audit, evaluate, or improve an existing agent. '
 'Key parameter: ''agent_id'' (UUID) of the agent to review; optional ''focus'' (prompt|skills|executions|all). '
 'Returns a structured report with findings and concrete improvement recommendations.',
 'diagnostic',
 '{"type":"object","properties":{"agent_id":{"type":"string","description":"UUID of the agent to review"},"focus":{"type":"string","enum":["prompt","skills","executions","all"],"description":"Aspect to focus on (default: all)"}},"required":["agent_id"]}',
 ARRAY[
   'agenthub_get_agent', 'agenthub_list_executions', 'agenthub_get_execution_detail',
   'agenthub_get_agent_stats', 'agenthub_get_agent_skills', 'agenthub_list_messages',
   'agenthub_list_agent_hooks', 'agenthub_list_agent_versions'
 ],
 false, 'fork',
 'Use when the user wants a thorough audit of an agent — quality, reliability, prompt effectiveness, '
 'or skill configuration. Also use proactively after seeing repeated execution failures. '
 'Run in fork mode to avoid polluting the main conversation with raw diagnostic data.',
 '<agent_id> [focus: prompt|skills|executions|all]',
 NOW(), NOW()),

-- batch-execute: like CC's batch.ts — parallel sub-agent work orchestration
('b1000000-0000-0000-0002-000000000008',
 'Batch Execute', 'batch-execute',
 'Orchestrates a large, parallelizable task by: researching scope, decomposing into independent work units, '
 'spawning parallel sub-agents (one per unit), tracking progress, and synthesizing results. '
 'Use when a task is too large for a single agent turn and can be split into independent parallel workstreams. '
 'Key parameter: ''instruction'' — a clear description of the overall goal. '
 'Returns a status table with progress per unit and a final synthesis.',
 'orchestration',
 '{"type":"object","properties":{"instruction":{"type":"string","description":"The overall task to parallelize and execute"},"max_agents":{"type":"integer","description":"Maximum parallel sub-agents to spawn (default 10, max 30)"}},"required":["instruction"]}',
 NULL,
 false, 'inline',
 'Use when the user requests a large-scale change or analysis that can be split into independent units '
 'that run in parallel (e.g. bulk agent updates, large dataset processing, multi-agent testing). '
 'Do NOT use for sequential tasks or tasks with strong interdependencies.',
 '<instruction> [max_agents]',
 NOW(), NOW()),

-- review-memory: like CC's remember.ts — audit and promote memory entries
('b1000000-0000-0000-0002-000000000009',
 'Review Memory', 'review-memory',
 'Reviews the agent''s current memory landscape: lists all stored memories grouped by type, '
 'identifies duplicates and outdated entries, checks for conflicts across memory types, '
 'and proposes promotions (e.g. feedback that should become agent instructions). '
 'Use when the agent is behaving inconsistently or memories seem stale. '
 'Key parameter: ''agent_id'' (UUID); optional ''action'' (audit|deduplicate|clean|promote). '
 'Returns a structured report of proposed changes — does NOT modify memories without confirmation.',
 'memory',
 '{"type":"object","properties":{"agent_id":{"type":"string","description":"UUID of the agent whose memories to review"},"action":{"type":"string","enum":["audit","deduplicate","clean","promote"],"description":"Review action (default: audit)"}},"required":["agent_id"]}',
 ARRAY[
   'agenthub_list_memories', 'agenthub_get_agent', 'agenthub_search_memories',
   'agenthub_delete_memory'
 ],
 true, 'inline',
 'Use when the user asks to review, clean up, or organize an agent''s memories. '
 'Also use when the agent is behaving inconsistently across sessions — memory conflicts may be the cause. '
 'Suggest proactively after curate-memory has run several times without full cleanup.',
 '<agent_id> [action: audit|deduplicate|clean|promote]',
 NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 6. BIND TOOLS TO NEW SKILLS
-- =====================================================================

-- review-agent tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'a1000000-0000-0000-0001-000000000002', 100, true, NOW()),  -- get_agent
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000054', 101, true, NOW()),  -- list_executions
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000055', 102, true, NOW()),  -- get_execution_detail
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000056', 103, true, NOW()),  -- get_agent_stats
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000063', 104, true, NOW()),  -- get_agent_skills
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000053', 105, true, NOW()),  -- list_messages
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000076', 106, true, NOW())   -- get_agent_version
ON CONFLICT DO NOTHING;

-- review-memory tools (bound tools: list_memories, search_memories, delete_memory)
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000009', 'c1000000-0000-0000-0000-000000000065', 100, true, NOW()),  -- list_memories
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000009', 'a1000000-0000-0000-0001-000000000002', 101, true, NOW()),  -- get_agent
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000009', 'c1000000-0000-0000-0000-000000000059', 102, true, NOW()),  -- search_memories
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000009', 'c1000000-0000-0000-0000-000000000061', 103, true, NOW())   -- delete_memory
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 7. REGISTER NEW SKILLS ON DEFAULT AGENT (d1000000-0000-0000-0001-000000000001)
-- =====================================================================

INSERT INTO agent_skill (agent_id, skill_id, created_at)
VALUES
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000007', NOW()),  -- review-agent
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000008', NOW()),  -- batch-execute
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000009', NOW())   -- review-memory
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 8. SET argument_hint FOR USER-INVOCABLE SKILLS
--    These are shown in the ## User Commands section so the LLM can
--    suggest proper usage with correct argument format.
--    Inspired by CC's BundledSkillDefinition.argumentHint.
-- =====================================================================

-- Skills already seeded in previous migrations — update argument_hint where NULL
UPDATE skill SET argument_hint = '<agent_id>'              WHERE slug = 'debug-agent'    AND argument_hint IS NULL;
UPDATE skill SET argument_hint = '<agent_id> [focus: prompt|tools|performance|cost]'
                                                            WHERE slug = 'optimize-agent' AND argument_hint IS NULL;
UPDATE skill SET argument_hint = '[description]'           WHERE slug = 'onboard-agent'  AND argument_hint IS NULL;
UPDATE skill SET argument_hint = '<agent_id> [action: audit|deduplicate|clean|list]'
                                                            WHERE slug = 'curate-memory'  AND argument_hint IS NULL;
UPDATE skill SET argument_hint = '[scope: agents|kbs|mcp|executions|all]'
                                                            WHERE slug = 'health-check'   AND argument_hint IS NULL;
UPDATE skill SET argument_hint = '<question> [datasource_id]'
                                                            WHERE slug = 'data-explorer'  AND argument_hint IS NULL;

-- =====================================================================
-- 9. SET is_search_or_read FOR NEW READ-ONLY TOOLS
-- =====================================================================

UPDATE tool
SET is_search_or_read = TRUE
WHERE name IN (
    'agenthub_get_datasource',
    'agenthub_test_datasource',
    'agenthub_get_agent_version'
);

-- =====================================================================
-- 10. CREATE INDEXES FOR NEW TOOLS
-- =====================================================================

CREATE INDEX IF NOT EXISTS idx_tool_is_destructive ON tool (id)
    WHERE is_destructive = TRUE;
