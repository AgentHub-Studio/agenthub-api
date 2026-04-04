-- Migration 000024: whenToUse/argumentHint for skills, interruptBehavior/isSearchOrRead for tools
--
-- Rationale:
--   1. skill.when_to_use: Separate "when to use" guidance from "what it does" description.
--      The description tells WHAT the skill does; when_to_use tells WHEN the LLM should
--      invoke it. This separation follows CC's BundledSkillDefinition.whenToUse pattern
--      and allows the system prompt to inject richer selection guidance without bloating
--      the tool listing.
--      Inspired by Claude Code's bundledSkills.ts: whenToUse field.
--
--   2. skill.argument_hint: Short hint for slash command argument format.
--      Used by the Flutter/Angular UI to show input hints when the user types a slash command.
--      Inspired by Claude Code's BundledSkillDefinition.argumentHint.
--
--   3. tool.interrupt_behavior: Whether to 'cancel' or 'block' when the user interrupts
--      the run while this tool is executing. Destructive/long-running tools should use
--      'block' (wait for completion before stopping), safe read-only tools use 'cancel'.
--      Inspired by Claude Code's Tool.ts interruptBehavior(): 'cancel' | 'block'.
--
--   4. tool.is_search_or_read: Marks tools whose results should auto-collapse in the
--      Flutter/Angular chat UI. Read, search, and list operations are collapsed by default
--      since users usually want the synthesized answer, not the raw result.
--      Inspired by Claude Code's Tool.ts isSearchOrReadCommand().

-- =====================================================================
-- 1. ADD skill.when_to_use
--    Separate field for "when should the LLM invoke this skill" guidance.
--    Injected into the system prompt as a sub-bullet under the skill listing.
--    Inspired by CC's BundledSkillDefinition.whenToUse.
-- =====================================================================

ALTER TABLE skill
    ADD COLUMN when_to_use TEXT,
    ADD COLUMN argument_hint VARCHAR(255);

COMMENT ON COLUMN skill.when_to_use IS 'Guidance on when the LLM should invoke this skill. '
    'Separate from description (what it does). '
    'Inspired by CC''s BundledSkillDefinition.whenToUse.';

COMMENT ON COLUMN skill.argument_hint IS 'Short hint for slash command argument format shown in the UI. '
    'Example: "<agent_id> [focus]". '
    'Inspired by CC''s BundledSkillDefinition.argumentHint.';

-- =====================================================================
-- 2. ADD tool.interrupt_behavior + tool.is_search_or_read
--    Inspired by CC's Tool.ts interruptBehavior() and isSearchOrReadCommand().
-- =====================================================================

ALTER TABLE tool
    ADD COLUMN interrupt_behavior VARCHAR(10),
    ADD COLUMN is_search_or_read  BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN tool.interrupt_behavior IS '"cancel" or "block". When NULL defaults to "cancel". '
    'Block = wait for completion before stopping (use for destructive/long-running tools). '
    'Inspired by CC''s Tool.ts interruptBehavior().';

COMMENT ON COLUMN tool.is_search_or_read IS 'When TRUE, Flutter/Angular UI collapses this tool''s result by default. '
    'Set for list, search, and read-only query tools. '
    'Inspired by CC''s Tool.ts isSearchOrReadCommand().';

-- =====================================================================
-- 3. POPULATE when_to_use FOR PLATFORM SKILLS
--    Following CC's whenToUse pattern: specific scenarios, not just descriptions.
-- =====================================================================

-- agent-management
UPDATE skill SET when_to_use =
    'Use when the user asks to create, configure, list, update, delete, publish, or clone an agent. '
    'Also use when asked to review an agent''s configuration or suggest improvements to its setup.',
    argument_hint = '[agent_id] [action]'
WHERE slug = 'agent-management';

-- skill-management
UPDATE skill SET when_to_use =
    'Use when the user asks to create, configure, update, or delete skills. '
    'Also use when asked to list available skills or check which skills are bound to an agent.',
    argument_hint = '[skill_id] [action]'
WHERE slug = 'skill-management';

-- tool-management
UPDATE skill SET when_to_use =
    'Use when the user asks to create, configure, test, or delete tools. '
    'Also use to inspect tool schemas, check bindings, or troubleshoot tool execution.',
    argument_hint = '[tool_id] [action]'
WHERE slug = 'tool-management';

-- knowledge-base-management
UPDATE skill SET when_to_use =
    'Use when the user asks to create, configure, or manage knowledge bases. '
    'Also use to check indexing status, list documents, or troubleshoot document processing.',
    argument_hint = '[kb_id] [action]'
WHERE slug = 'knowledge-base-management';

-- chat-management
UPDATE skill SET when_to_use =
    'Use when the user asks to list, archive, or manage chat sessions. '
    'Also use to fetch session history or delete old sessions.',
    argument_hint = '[session_id] [action]'
WHERE slug = 'chat-management';

-- mcp-management
UPDATE skill SET when_to_use =
    'Use when the user asks to connect, configure, update, or remove MCP servers. '
    'Also use to check MCP server connectivity or list available servers.',
    argument_hint = '[server_id] [action]'
WHERE slug = 'mcp-management';

-- settings-management
UPDATE skill SET when_to_use =
    'Use when the user asks to view or update platform settings, API keys, LLM provider configuration, '
    'or other global configuration.',
    argument_hint = '[setting_name] [value]'
WHERE slug = 'settings-management';

-- prompt-template-management
UPDATE skill SET when_to_use =
    'Use when the user asks to create, edit, view, or delete prompt templates. '
    'Also use when they ask to apply a template to an agent or browse available templates.',
    argument_hint = '[template_id] [action]'
WHERE slug = 'prompt-template-management';

-- datasource-management
UPDATE skill SET when_to_use =
    'Use when the user asks to connect, configure, or manage database datasources (PostgreSQL, MySQL, SQL Server). '
    'Also use to set up VPN tunnels for datasource access.',
    argument_hint = '[datasource_id] [action]'
WHERE slug = 'datasource-management';

-- document-search
UPDATE skill SET when_to_use =
    'Use when the user asks a question that may be answered by documents in the knowledge base. '
    'Use BEFORE answering any factual question about company data, policies, products, or documentation. '
    'Always search before saying you don''t know.',
    argument_hint = '<query>'
WHERE slug = 'document-search';

-- execute-sql
UPDATE skill SET when_to_use =
    'Use when the user asks a data question that requires querying a database. '
    'Also use when asked to count, aggregate, filter, or analyze structured data. '
    'Always confirm destructive queries (INSERT, UPDATE, DELETE) with the user before executing.',
    argument_hint = '<sql_query> [datasource_id]'
WHERE slug = 'execute-sql';

-- http-request
UPDATE skill SET when_to_use =
    'Use when the user asks to call an external API, fetch data from a URL, or submit data to a service. '
    'Confirm POST/PUT/DELETE requests that may have side effects.',
    argument_hint = '<method> <url> [body]'
WHERE slug = 'http-request';

-- debug-agent
UPDATE skill SET when_to_use =
    'Use when the user reports that an agent is failing, producing errors, giving wrong answers, or behaving unexpectedly. '
    'Also use proactively after seeing repeated execution failures in the dashboard.',
    argument_hint = '<agent_id>'
WHERE slug = 'debug-agent';

-- optimize-agent
UPDATE skill SET when_to_use =
    'Use when the user wants to improve an agent''s quality, reduce its token usage or cost, '
    'fix slow responses, or tune its behavior for a specific domain.',
    argument_hint = '<agent_id> [focus: prompt|tools|performance|cost]'
WHERE slug = 'optimize-agent';

-- onboard-agent
UPDATE skill SET when_to_use =
    'Use when the user wants to create a brand new agent and needs guidance on setup. '
    'Also use when an existing agent feels misconfigured and needs a fresh start.',
    argument_hint = '[description]'
WHERE slug = 'onboard-agent';

-- curate-memory
UPDATE skill SET when_to_use =
    'Use when the user asks to review, clean up, or manage an agent''s memories. '
    'Also use when memories seem inconsistent, outdated, or when the agent is behaving '
    'differently than expected across sessions.',
    argument_hint = '<agent_id> [action: audit|deduplicate|clean|list]'
WHERE slug = 'curate-memory';

-- health-check
UPDATE skill SET when_to_use =
    'Use when the user asks about platform status, suspects something is broken, or wants a health overview. '
    'Also trigger proactively when execution failure rates spike or KBs stop indexing.',
    argument_hint = '[scope: agents|kbs|mcp|executions|all]'
WHERE slug = 'health-check';

-- data-explorer
UPDATE skill SET when_to_use =
    'Use when the user asks an analytical question that may require both database queries and document lookups. '
    'Also use when a single data source is insufficient to answer the question.',
    argument_hint = '<question> [datasource_id]'
WHERE slug = 'data-explorer';

-- =====================================================================
-- 4. SET interrupt_behavior FOR DESTRUCTIVE/LONG-RUNNING TOOLS
--    'block' = wait for completion before stopping (CC pattern)
--    NULL/default = 'cancel' (safe to abort immediately)
-- =====================================================================

-- Destructive tools should block to avoid partial state
UPDATE tool
SET interrupt_behavior = 'block'
WHERE name IN (
    'agenthub_delete_agent',
    'agenthub_delete_skill',
    'agenthub_delete_tool',
    'agenthub_delete_knowledge_base',
    'agenthub_delete_mcp_server',
    'agenthub_delete_session',
    'agenthub_delete_prompt_template',
    'agenthub_delete_memory',
    'agenthub_clone_agent',
    'agenthub_bulk_agent_status'
);

-- =====================================================================
-- 5. SET is_search_or_read FOR LIST/SEARCH/GET TOOLS
--    Flutter/Angular UI will auto-collapse these results.
--    Inspired by CC's isSearchOrReadCommand() per-tool method.
-- =====================================================================

UPDATE tool
SET is_search_or_read = TRUE
WHERE name IN (
    -- List operations
    'agenthub_list_agents',
    'agenthub_list_skills',
    'agenthub_list_tools',
    'agenthub_list_knowledge_bases',
    'agenthub_list_mcp_servers',
    'agenthub_list_sessions',
    'agenthub_list_datasources',
    'agenthub_list_executions',
    'agenthub_list_prompt_templates',
    'agenthub_list_documents',
    'agenthub_list_memories',
    'agenthub_list_messages',
    'agenthub_list_agent_versions',
    'agenthub_list_agent_hooks',
    -- Get/read operations
    'agenthub_get_agent',
    'agenthub_get_skill',
    'agenthub_get_tool',
    'agenthub_get_knowledge_base',
    'agenthub_get_mcp_server',
    'agenthub_get_settings',
    'agenthub_get_session',
    'agenthub_get_prompt_template',
    'agenthub_get_agent_skills',
    'agenthub_get_agent_kbs',
    'agenthub_get_kb_status',
    'agenthub_get_agent_stats',
    'agenthub_get_execution_detail',
    -- Search operations
    'agenthub_search_memories'
);

-- =====================================================================
-- 6. CREATE INDEXES
-- =====================================================================

CREATE INDEX idx_skill_when_to_use_exists ON skill (id)
    WHERE when_to_use IS NOT NULL;

CREATE INDEX idx_tool_is_search_or_read ON tool (id)
    WHERE is_search_or_read = TRUE;
