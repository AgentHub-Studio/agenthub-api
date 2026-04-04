-- Migration 000026: Claude Code-inspired patterns + document management + session/agent gaps
--
-- Rationale:
--   Analysis of Claude Code's bundledSkills, Tool.ts, hooks/coreTypes, and Task.ts revealed
--   several patterns not yet captured in AgentHub. This migration implements the highest-value
--   additions that make the AgentHub Assistant more capable as a platform operator.
--
--   1. Schema: skill.user_invocable, skill.availability, skill.fork_agent_type
--      Inspired by CC's BundledSkillDefinition: disableModelInvocation (→ user_invocable=false),
--      availability ('all'|'claude_ai'|'console'), and fork context agent type.
--
--   2. Schema: agent_hook.async_rewake
--      Inspired by CC's HookCommand.asyncRewake: exit code 2 signals the model to wake up.
--      Enables background hooks that conditionally interrupt the session.
--
--   3. Schema: tool.activity_description, tool.tool_summary
--      Inspired by CC's Tool.getActivityDescription() and getToolUseSummary():
--      - activity_description: text shown in spinner while tool runs ("Searching documents…")
--      - tool_summary: compact one-liner for collapsed views ("Found 5 results")
--
--   4. Schema: new hook event types from CC's HOOK_EVENTS (27-event catalog)
--      CC has: TaskCreated, TaskCompleted, PreCompact, PostCompact, Stop, StopFailure,
--      SubagentStart, SubagentStop, PermissionRequest, PermissionDenied, FileChanged,
--      WorktreeCreate, WorktreeRemove, CwdChanged, ConfigChange, Elicitation.
--      We add the most relevant ones to the existing event comment.
--
--   5. New tools: document lifecycle (list/get/delete), session create/update,
--      agent system-prompt patch, prompt-template apply, execution logs/retry.
--      IDs use c1000000-0000-0000-0000-00000000007{8..9} and 008{0..9} range.
--
--   6. Three new skills inspired by CC bundled skills:
--      - skillify: create skills from conversation context (CC's /skillify)
--      - compact: trigger context compaction (CC's /compact)
--      - api-explorer: explore and test tools/API endpoints (CC's /verify pattern)
--
--   7. Populate activity_description for high-frequency existing tools.
--
--   8. Set user_invocable=false for skills that should NOT be model-invocable
--      (platform management skills the LLM should suggest but not autonomously trigger).

-- =====================================================================
-- 1. SCHEMA: skill enhancements
-- =====================================================================

ALTER TABLE skill
    ADD COLUMN IF NOT EXISTS user_invocable   BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS availability     VARCHAR(20) NOT NULL DEFAULT 'all',
    ADD COLUMN IF NOT EXISTS fork_agent_type  VARCHAR(50);

COMMENT ON COLUMN skill.user_invocable IS
    'When FALSE: skill is user-invocable only (not callable by the LLM directly). '
    'LLM can SUGGEST it but cannot invoke it autonomously. '
    'Mirrors CC''s BundledSkillDefinition.disableModelInvocation (inverted).';

COMMENT ON COLUMN skill.availability IS
    '"all" (default), "claude_ai" (Anthropic cloud only), or "console" (admin console only). '
    'Gates which surfaces can invoke this skill. '
    'Inspired by CC''s BundledSkillDefinition.availability.';

COMMENT ON COLUMN skill.fork_agent_type IS
    'When context_mode="fork", specifies the agent type for the sub-agent spawned. '
    'Example values: "general-purpose", "bash", "code-review". '
    'NULL = default agent type. '
    'Inspired by CC''s BundledSkillDefinition.agent.';

-- =====================================================================
-- 2. SCHEMA: agent_hook async_rewake + extended event catalog
-- =====================================================================

ALTER TABLE agent_hook
    ADD COLUMN IF NOT EXISTS async_rewake BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN agent_hook.async_rewake IS
    'When TRUE (requires is_async=TRUE): if the hook exits with code 2, the model is woken. '
    'Enables background tasks that conditionally interrupt the session. '
    'Inspired by CC''s HookCommand.asyncRewake.';

COMMENT ON COLUMN agent_hook.event IS
    'Hook event name. Supported events:
     Lifecycle: session_start, session_end, stop, stop_failure
     Tool:      pre_tool_use, post_tool_use, post_tool_failure
     Context:   pre_compact, post_compact
     Subagent:  subagent_start, subagent_stop
     Task:      task_created, task_completed
     Security:  permission_request, permission_denied
     System:    notification, file_changed, config_change
     Inspired by CC''s HOOK_EVENTS catalog (27 events).';

-- =====================================================================
-- 3. SCHEMA: tool UI hints
-- =====================================================================

ALTER TABLE tool
    ADD COLUMN IF NOT EXISTS activity_description VARCHAR(200),
    ADD COLUMN IF NOT EXISTS tool_summary         VARCHAR(200);

COMMENT ON COLUMN tool.activity_description IS
    'Short present-participle phrase shown in spinner while tool runs. '
    'Example: "Searching knowledge base…", "Fetching agent config…". '
    'Inspired by CC''s Tool.getActivityDescription().';

COMMENT ON COLUMN tool.tool_summary IS
    'One-line summary template for compact/collapsed views. '
    'May use {count}, {name}, {result} placeholders. '
    'Example: "Found {count} results", "Updated {name}". '
    'Inspired by CC''s Tool.getToolUseSummary().';

-- =====================================================================
-- 4. NEW TOOLS: document lifecycle (c078–c080)
--    Knowledge base documents are currently write-only from the LLM side
--    (upload=c048, sync=c044). Add list/get/delete to close the gap.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer,
                  is_destructive, search_hint, always_load, concurrency_safe,
                  max_result_chars, interrupt_behavior, is_search_or_read,
                  activity_description, tool_summary, created_at, updated_at)
VALUES
-- c078: list_documents
('c1000000-0000-0000-0000-000000000078',
 'agenthub_list_documents', 'HTTP',
 'Lists documents in a knowledge base with their processing status, file size, and indexing state. '
 'Use to check upload progress, find document IDs for deletion, or audit KB contents. '
 'Parameters: ''kb_id'' (required), ''status'' (optional: PENDING|EXTRACTING|CHUNKING|EMBEDDING|INDEXED|FAILED), '
 '''page'' (default 0), ''size'' (default 20, max 100).',
 '{"method":"GET","urlTemplate":"/api/knowledge-bases/{kb_id}/documents?page={page}&size={size}&status={status}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"kb_id":{"type":"string","description":"Knowledge base UUID"},"status":{"type":"string","description":"Filter by status: PENDING, EXTRACTING, CHUNKING, EMBEDDING, INDEXED, FAILED"},"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}},"required":["kb_id"]}}',
 ARRAY['knowledge-bases','documents','list','read'],
 TRUE, FALSE, FALSE,
 'list documents knowledge base files', FALSE, TRUE, 10000,
 NULL, TRUE,
 'Listing documents…', 'Found {count} documents',
 NOW(), NOW()),

-- c079: get_document
('c1000000-0000-0000-0000-000000000079',
 'agenthub_get_document', 'HTTP',
 'Gets details of a specific document: filename, file size, MIME type, processing status, '
 'chunk count, embedding count, and any processing error messages. '
 'Use when a document shows FAILED status to understand what went wrong. '
 'Parameters: ''kb_id'' (required), ''document_id'' (required).',
 '{"method":"GET","urlTemplate":"/api/knowledge-bases/{kb_id}/documents/{document_id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"kb_id":{"type":"string","description":"Knowledge base UUID"},"document_id":{"type":"string","description":"Document UUID"}},"required":["kb_id","document_id"]}}',
 ARRAY['knowledge-bases','documents','get','read'],
 TRUE, FALSE, FALSE,
 'get document details status chunks', FALSE, TRUE, 6000,
 NULL, TRUE,
 'Fetching document details…', 'Document: {name} ({status})',
 NOW(), NOW()),

-- c080: delete_document
('c1000000-0000-0000-0000-000000000080',
 'agenthub_delete_document', 'HTTP',
 'Permanently deletes a document from a knowledge base, removing its file, chunks, and embeddings. '
 'ALWAYS confirm with the user before deleting — this removes the document from all RAG searches. '
 'After deletion, sync the KB with agenthub_sync_knowledge_base to update the index. '
 'Parameters: ''kb_id'' (required), ''document_id'' (required).',
 '{"method":"DELETE","urlTemplate":"/api/knowledge-bases/{kb_id}/documents/{document_id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"kb_id":{"type":"string","description":"Knowledge base UUID"},"document_id":{"type":"string","description":"Document UUID to delete"}},"required":["kb_id","document_id"]}}',
 ARRAY['knowledge-bases','documents','delete','write'],
 FALSE, TRUE, TRUE,
 'delete remove document knowledge base', FALSE, FALSE, 2000,
 'block', FALSE,
 'Deleting document…', NULL,
 NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 5. NEW TOOLS: session lifecycle (c081–c082)
--    The LLM can list/archive/delete sessions but cannot CREATE a new one
--    or rename an existing one.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer,
                  is_destructive, search_hint, always_load, concurrency_safe,
                  max_result_chars, interrupt_behavior, is_search_or_read,
                  activity_description, tool_summary, created_at, updated_at)
VALUES
-- c081: create_session
('c1000000-0000-0000-0000-000000000081',
 'agenthub_create_session', 'HTTP',
 'Creates a new chat session, optionally bound to a specific agent. '
 'Use when the user asks to start a fresh conversation with a given agent, '
 'or to create a test session to verify agent behavior. '
 'Parameters: ''name'' (optional display name), ''agent_id'' (optional — binds session to agent). '
 'Returns the new session ID and a link to open it.',
 '{"method":"POST","url":"/api/chat/sessions","useCallerToken":true,"inputSchema":{"type":"object","properties":{"name":{"type":"string","description":"Optional session display name"},"agent_id":{"type":"string","description":"Optional agent UUID to bind to this session"}}}}',
 ARRAY['chat','sessions','create','write'],
 FALSE, FALSE, FALSE,
 'create new chat session', FALSE, FALSE, 4000,
 NULL, FALSE,
 'Creating chat session…', 'Session created',
 NOW(), NOW()),

-- c082: update_session
('c1000000-0000-0000-0000-000000000082',
 'agenthub_update_session', 'HTTP',
 'Updates a chat session — currently supports renaming and changing the bound agent. '
 'Use when the user wants to give a session a descriptive name for easier identification. '
 'Parameters: ''session_id'' (required), ''name'' (optional new display name), '
 '''agent_id'' (optional new agent UUID).',
 '{"method":"PATCH","urlTemplate":"/api/chat/sessions/{session_id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"session_id":{"type":"string","description":"Session UUID"},"name":{"type":"string","description":"New session display name"},"agent_id":{"type":"string","description":"New agent UUID to bind"}},"required":["session_id"]}}',
 ARRAY['chat','sessions','update','write'],
 FALSE, FALSE, FALSE,
 'rename update chat session', FALSE, FALSE, 4000,
 NULL, FALSE,
 'Updating session…', 'Session updated',
 NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 6. NEW TOOLS: agent system-prompt and template management (c083–c085)
--    The LLM can update full agent config but there''s no shortcut for
--    the most common operation: updating just the system prompt, or
--    applying a curated prompt template.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer,
                  is_destructive, search_hint, always_load, concurrency_safe,
                  max_result_chars, interrupt_behavior, is_search_or_read,
                  activity_description, tool_summary, created_at, updated_at)
VALUES
-- c083: update_agent_system_prompt
('c1000000-0000-0000-0000-000000000083',
 'agenthub_update_agent_system_prompt', 'HTTP',
 'Updates ONLY the system prompt of an agent, leaving all other config unchanged. '
 'Use this instead of agenthub_update_agent when only the system prompt needs to change. '
 'ALWAYS show the user the new system prompt text before updating. '
 'Parameters: ''agent_id'' (required), ''system_prompt'' (required — full new system prompt text). '
 'Creates a new agent version automatically.',
 '{"method":"PATCH","urlTemplate":"/api/agents/{agent_id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"},"system_prompt":{"type":"string","description":"New system prompt text"}},"required":["agent_id","system_prompt"]}}',
 ARRAY['agents','system-prompt','update','write'],
 FALSE, FALSE, FALSE,
 'update patch system prompt agent', FALSE, FALSE, 4000,
 NULL, FALSE,
 'Updating system prompt…', 'System prompt updated',
 NOW(), NOW()),

-- c084: list_prompt_templates_for_agent
-- (Complements c029 list_prompt_templates — returns templates scoped to an agent + global)
('c1000000-0000-0000-0000-000000000084',
 'agenthub_list_agent_prompt_templates', 'HTTP',
 'Lists prompt templates available to a specific agent: both global templates (agent_id IS NULL) '
 'and templates created specifically for this agent. '
 'Returns template name, slug, category, and first 200 chars of content as preview. '
 'Use before applying a template to see what''s available. '
 'Parameters: ''agent_id'' (required).',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/prompt-templates","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"}},"required":["agent_id"]}}',
 ARRAY['agents','prompt-templates','list','read'],
 TRUE, FALSE, FALSE,
 'list agent prompt templates available', FALSE, TRUE, 10000,
 NULL, TRUE,
 'Loading prompt templates…', 'Found {count} templates',
 NOW(), NOW()),

-- c085: apply_prompt_template
('c1000000-0000-0000-0000-000000000085',
 'agenthub_apply_prompt_template', 'HTTP',
 'Applies a prompt template to an agent: copies the template''s content into the agent''s system_prompt '
 'and optionally sets model_override if the template specifies one. '
 'ALWAYS show the user what will change before applying. '
 'For builtin templates, creates a copy first (cannot modify originals). '
 'Parameters: ''agent_id'' (required), ''template_id'' (required), '
 '''merge'' (optional bool — if true, appends template content to existing system_prompt; default false=replace).',
 '{"method":"POST","urlTemplate":"/api/agents/{agent_id}/apply-template","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"},"template_id":{"type":"string","description":"Prompt template UUID"},"merge":{"type":"boolean","description":"If true, append template to existing prompt (default: false=replace)"}},"required":["agent_id","template_id"]}}',
 ARRAY['agents','prompt-templates','apply','write'],
 FALSE, TRUE, FALSE,
 'apply template to agent system prompt', FALSE, FALSE, 4000,
 NULL, FALSE,
 'Applying template…', 'Template applied to agent',
 NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 7. NEW TOOLS: execution lifecycle (c086–c088)
--    Execution visibility is critical for debugging. The LLM can list
--    and get executions (c054-c056) but cannot see logs or retry.
-- =====================================================================

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer,
                  is_destructive, search_hint, always_load, concurrency_safe,
                  max_result_chars, interrupt_behavior, is_search_or_read,
                  activity_description, tool_summary, created_at, updated_at)
VALUES
-- c086: get_execution_logs
('c1000000-0000-0000-0000-000000000086',
 'agenthub_get_execution_logs', 'HTTP',
 'Gets the detailed execution logs for a specific agent execution: LLM calls, tool invocations, '
 'errors, token usage per step, and timing. Essential for debugging failed executions. '
 'Use after agenthub_get_execution_detail to dig into errors. '
 'Parameters: ''execution_id'' (required), ''level'' (optional: "debug"|"info"|"error" — default "info"). '
 'Returns structured log entries sorted by timestamp.',
 '{"method":"GET","urlTemplate":"/api/executions/{execution_id}/logs?level={level}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"execution_id":{"type":"string","description":"Execution UUID"},"level":{"type":"string","enum":["debug","info","error"],"description":"Minimum log level (default: info)"}},"required":["execution_id"]}}',
 ARRAY['executions','logs','debug','read'],
 TRUE, TRUE, FALSE,
 'get execution logs errors debug steps', FALSE, TRUE, 20000,
 NULL, TRUE,
 'Fetching execution logs…', '{count} log entries',
 NOW(), NOW()),

-- c087: retry_execution
('c1000000-0000-0000-0000-000000000087',
 'agenthub_retry_execution', 'HTTP',
 'Retries a failed or timed-out agent execution with the same input. '
 'ALWAYS confirm with the user before retrying — it will consume tokens and may have side effects. '
 'Check agenthub_get_execution_detail first to confirm the failure reason. '
 'Parameters: ''execution_id'' (required). '
 'Returns the new execution ID (different from the original).',
 '{"method":"POST","urlTemplate":"/api/executions/{execution_id}/retry","useCallerToken":true,"inputSchema":{"type":"object","properties":{"execution_id":{"type":"string","description":"Failed execution UUID to retry"}},"required":["execution_id"]}}',
 ARRAY['executions','retry','write'],
 FALSE, TRUE, FALSE,
 'retry execution failed agent run', FALSE, FALSE, 4000,
 NULL, FALSE,
 'Retrying execution…', 'Retry started',
 NOW(), NOW()),

-- c088: get_agent_config
('c1000000-0000-0000-0000-000000000088',
 'agenthub_get_agent_config', 'HTTP',
 'Gets the full configuration snapshot of an agent: system_prompt (full text), model_config '
 '(provider, model, temperature, max_tokens), permission_rules, current_version, and status. '
 'Use when you need to read the system prompt before editing it, or to compare versions. '
 'Unlike agenthub_get_agent (which returns summary fields), this always includes full system_prompt text. '
 'Parameters: ''agent_id'' (required).',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/config","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"}},"required":["agent_id"]}}',
 ARRAY['agents','config','system-prompt','read'],
 TRUE, FALSE, FALSE,
 'get agent full config system prompt model', FALSE, TRUE, 30000,
 NULL, TRUE,
 'Loading agent config…', 'Agent config loaded',
 NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 8. BIND NEW TOOLS TO SKILLS
-- =====================================================================

-- knowledge-base-management: add document list/get/delete
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000078', 114, true, NOW()),  -- list_documents
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000079', 115, true, NOW()),  -- get_document
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000080', 116, true, NOW())   -- delete_document
ON CONFLICT DO NOTHING;

-- chat-management: add create/update session
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000081', 113, true, NOW()),  -- create_session
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000082', 114, true, NOW())   -- update_session
ON CONFLICT DO NOTHING;

-- agent-management: add system-prompt patch, template ops, get_config
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000083', 119, true, NOW()),  -- update_system_prompt
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000084', 120, true, NOW()),  -- list_agent_prompt_templates
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000085', 121, true, NOW()),  -- apply_prompt_template
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000088', 122, true, NOW())   -- get_agent_config
ON CONFLICT DO NOTHING;

-- prompt-template-management: apply template
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000009', 'c1000000-0000-0000-0000-000000000085', 103, true, NOW())   -- apply_prompt_template
ON CONFLICT DO NOTHING;

-- agent-management/review: add execution logs + retry
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000086', 123, true, NOW()),  -- get_execution_logs
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000087', 124, true, NOW())   -- retry_execution
ON CONFLICT DO NOTHING;

-- review-agent skill: add execution logs + agent config
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000086', 107, true, NOW()),  -- get_execution_logs
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000007', 'c1000000-0000-0000-0000-000000000088', 108, true, NOW())   -- get_agent_config
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 9. THREE NEW PLATFORM SKILLS inspired by CC bundled skills
--    IDs: b1000000-0000-0000-0002-000000000010 to 000000000012
-- =====================================================================

INSERT INTO skill (id, name, slug, description, category, input_schema,
                   allowed_tools, disable_model_invocation, context_mode,
                   user_invocable, availability, when_to_use, argument_hint,
                   created_at, updated_at)
VALUES

-- skillify: help user create a new skill from a conversation or description
-- Inspired by CC's /skillify — turns a prompt into a reusable skill definition
('b1000000-0000-0000-0002-000000000010',
 'Skillify', 'skillify',
 'Guides the user through creating a new skill from scratch or from a conversation example. '
 'Analyzes the desired behavior, generates an appropriate description, input_schema, '
 'and when_to_use guidance, then calls agenthub_create_skill to register it. '
 'Can also bind appropriate tools and suggest system prompt additions to support the new skill. '
 'Use when the user wants to package a repeated workflow or external API call as a reusable skill. '
 'Key parameter: ''description'' — what the skill should do. '
 'Returns the new skill ID, slug, and suggested agent binding instructions.',
 'platform',
 '{"type":"object","properties":{"description":{"type":"string","description":"What the new skill should do"},"name":{"type":"string","description":"Optional skill name"},"category":{"type":"string","description":"Skill category (optional)"},"from_conversation":{"type":"boolean","description":"If true, infer skill spec from recent conversation context"}},"required":["description"]}',
 ARRAY[
   'agenthub_create_skill', 'agenthub_list_skills',
   'agenthub_create_tool', 'agenthub_list_tools',
   'agenthub_list_agent_skills', 'agenthub_sync_agent_skills'
 ],
 FALSE, 'inline',
 TRUE, 'all',
 'Use when the user wants to create a new skill, package a repeated operation, or add a capability '
 'that doesn''t yet exist in the platform. Also trigger when they describe a workflow that could be '
 'reused across multiple agents.',
 '<description> [--name <name>] [--category <category>]',
 NOW(), NOW()),

-- compact: manually trigger context compaction
-- Inspired by CC's /compact command — preserves key context, summarizes the rest
('b1000000-0000-0000-0002-000000000011',
 'Compact', 'compact',
 'Triggers context compaction for the current session: summarizes older conversation turns '
 'into a compact_summary message, preserving key decisions, errors, and unfinished work. '
 'Use when the conversation is growing long and you want to reduce token usage '
 'while keeping important context. '
 'The compaction preserves: active tool calls, last 4 messages, error state, and key decisions. '
 'Optional parameter: ''focus'' — specific topic to emphasize in the summary. '
 'Returns a confirmation with estimated token savings.',
 'system',
 '{"type":"object","properties":{"focus":{"type":"string","description":"Optional: topic to emphasize in the compact summary"},"session_id":{"type":"string","description":"Session to compact (default: current session)"}}}',
 ARRAY['agenthub_list_messages', 'agenthub_get_session'],
 FALSE, 'inline',
 TRUE, 'all',
 'Use when the conversation has grown long (30+ messages) and the user wants to reduce token cost. '
 'Also use proactively when context compaction is needed before a complex multi-step task. '
 'Suggest it when you notice repeated context re-injection or the session is approaching limits.',
 '[focus: topic to preserve]',
 NOW(), NOW()),

-- api-explorer: explore and interactively test platform API endpoints
-- Inspired by CC's /verify skill — validates behavior with real calls
('b1000000-0000-0000-0002-000000000012',
 'API Explorer', 'api-explorer',
 'Explores and tests AgentHub platform APIs interactively: lists available tools for a skill, '
 'shows their input/output schemas, makes test calls with sample data, and reports results. '
 'Use when the user wants to understand what a tool does, test if credentials are correct, '
 'verify an API endpoint is working, or explore new capabilities. '
 'Key parameter: ''target'' — tool name, skill slug, or API path to explore. '
 'Returns tool schema, sample call, and live test result.',
 'platform',
 '{"type":"object","properties":{"target":{"type":"string","description":"Tool name, skill slug, or API path to explore"},"test_input":{"type":"object","description":"Optional test input to use instead of generated sample data"},"dry_run":{"type":"boolean","description":"If true, show what would be called but do not execute (default: false)"}},"required":["target"]}',
 ARRAY[
   'agenthub_list_tools', 'agenthub_get_tool',
   'agenthub_list_skills', 'agenthub_get_skill',
   'agenthub_get_settings', 'agenthub_list_datasources'
 ],
 FALSE, 'inline',
 TRUE, 'all',
 'Use when the user wants to understand what a tool or skill does, test connectivity to external services, '
 'debug tool configuration, or explore what APIs are available. '
 'Also use when a tool is returning unexpected errors and you need to examine its schema.',
 '<tool-name|skill-slug|api-path> [--dry-run]',
 NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 10. BIND NEW SKILLS TO AGENTHUB ASSISTANT
-- =====================================================================

INSERT INTO agent_skill (agent_id, skill_id, created_at)
VALUES
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000010', NOW()),  -- skillify
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000011', NOW()),  -- compact
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000012', NOW())   -- api-explorer
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 11. SET user_invocable=FALSE FOR PLATFORM MANAGEMENT SKILLS
--     These are complex multi-step operations that should only be triggered
--     explicitly by the user (/agent-management, /skill-management, etc.),
--     not autonomously invoked by the LLM mid-conversation.
--     The LLM can SUGGEST them but cannot invoke them directly.
--     Inspired by CC's disableModelInvocation=true pattern.
-- =====================================================================

-- Platform management skills: LLM should suggest, user confirms
UPDATE skill SET user_invocable = FALSE
WHERE slug IN (
    'agent-management',
    'skill-management',
    'tool-management',
    'knowledge-base-management',
    'settings-management',
    'datasource-management',
    'mcp-management',
    'batch-execute'   -- high-blast-radius, always needs explicit user intent
);

-- =====================================================================
-- 12. POPULATE activity_description FOR HIGH-FREQUENCY EXISTING TOOLS
--     Gives the Flutter/Angular UI meaningful spinner text.
-- =====================================================================

UPDATE tool SET activity_description = 'Searching documents…'
WHERE name IN ('agenthub_search_documents', 'document_search');

UPDATE tool SET activity_description = 'Searching memories…'
WHERE name IN ('agenthub_search_memories', 'agenthub_list_memories');

UPDATE tool SET activity_description = 'Loading agents…'
WHERE name = 'agenthub_list_agents';

UPDATE tool SET activity_description = 'Loading agent details…'
WHERE name = 'agenthub_get_agent';

UPDATE tool SET activity_description = 'Running agent…'
WHERE name = 'agenthub_run_chat';

UPDATE tool SET activity_description = 'Loading sessions…'
WHERE name = 'agenthub_list_sessions';

UPDATE tool SET activity_description = 'Loading messages…'
WHERE name = 'agenthub_list_messages';

UPDATE tool SET activity_description = 'Loading skills…'
WHERE name = 'agenthub_list_skills';

UPDATE tool SET activity_description = 'Loading tools…'
WHERE name = 'agenthub_list_tools';

UPDATE tool SET activity_description = 'Loading knowledge bases…'
WHERE name = 'agenthub_list_knowledge_bases';

UPDATE tool SET activity_description = 'Syncing knowledge base…'
WHERE name = 'agenthub_sync_knowledge_base';

UPDATE tool SET activity_description = 'Uploading document…'
WHERE name = 'agenthub_upload_document';

UPDATE tool SET activity_description = 'Loading executions…'
WHERE name IN ('agenthub_list_executions', 'agenthub_list_agent_versions');

UPDATE tool SET activity_description = 'Fetching execution details…'
WHERE name = 'agenthub_get_execution_detail';

-- =====================================================================
-- 13. SET tool_summary FOR KEY TOOLS
-- =====================================================================

UPDATE tool SET tool_summary = 'Found {count} agents'        WHERE name = 'agenthub_list_agents';
UPDATE tool SET tool_summary = 'Found {count} skills'        WHERE name = 'agenthub_list_skills';
UPDATE tool SET tool_summary = 'Found {count} tools'         WHERE name = 'agenthub_list_tools';
UPDATE tool SET tool_summary = 'Found {count} sessions'      WHERE name = 'agenthub_list_sessions';
UPDATE tool SET tool_summary = 'Found {count} messages'      WHERE name = 'agenthub_list_messages';
UPDATE tool SET tool_summary = 'Found {count} documents'     WHERE name = 'agenthub_list_documents';
UPDATE tool SET tool_summary = '{count} memories found'      WHERE name = 'agenthub_list_memories';
UPDATE tool SET tool_summary = '{count} search results'      WHERE name IN ('agenthub_search_documents', 'document_search');
UPDATE tool SET tool_summary = 'Found {count} executions'    WHERE name = 'agenthub_list_executions';
UPDATE tool SET tool_summary = 'Found {count} knowledge bases' WHERE name = 'agenthub_list_knowledge_bases';
UPDATE tool SET tool_summary = 'Found {count} MCP servers'   WHERE name = 'agenthub_list_mcp_servers';

-- =====================================================================
-- 14. MARK NEW READ-ONLY TOOLS as is_search_or_read
-- =====================================================================

UPDATE tool SET is_search_or_read = TRUE
WHERE name IN (
    'agenthub_list_documents',
    'agenthub_get_document',
    'agenthub_get_execution_logs',
    'agenthub_get_agent_config',
    'agenthub_list_agent_prompt_templates'
);

-- =====================================================================
-- 15. CREATE INDEXES
-- =====================================================================

CREATE INDEX IF NOT EXISTS idx_skill_user_invocable ON skill (id)
    WHERE user_invocable = FALSE;

CREATE INDEX IF NOT EXISTS idx_skill_availability ON skill (availability)
    WHERE availability != 'all';

CREATE INDEX IF NOT EXISTS idx_tool_has_activity_desc ON tool (id)
    WHERE activity_description IS NOT NULL;
