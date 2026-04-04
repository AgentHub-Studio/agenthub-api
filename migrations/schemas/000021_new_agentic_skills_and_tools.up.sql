-- New agentic skills and tools inspired by Claude Code bundled skills patterns.
-- Adds diagnostic, optimization, and workflow skills that make the agentic chat
-- more effective at managing the AgentHub platform itself.

-- =====================================================================
-- 1. New skills: diagnostic, optimization, and workflow
-- =====================================================================
-- Inspired by Claude Code's bundled skills: /debug (restricted read-only),
-- /simplify (parallel review agents), /remember (memory curation),
-- /batch (parallel execution), /skillify (interactive wizard).

INSERT INTO skill (id, name, slug, description, category, input_schema, allowed_tools, disable_model_invocation, created_at, updated_at)
VALUES

-- debug-agent: diagnostic skill with restricted read-only tools (like CC's /debug)
('b1000000-0000-0000-0002-000000000001', 'Debug Agent', 'debug-agent',
 'Diagnoses agent execution failures by analyzing recent executions, error patterns, and configuration issues. '
 'Use when an agent is failing, producing incorrect results, or behaving unexpectedly. '
 'Analyzes execution logs, tool call history, system prompt effectiveness, and skill bindings. '
 'Parameters: ''agent_id'' (target agent UUID), ''issue'' (optional description of the problem). '
 'Returns a structured diagnosis with root cause analysis, affected components, and suggested fixes.',
 'diagnostic',
 '{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID to diagnose"},"issue":{"type":"string","description":"Optional description of the issue"}},"required":["agent_id"]}',
 ARRAY['agenthub_list_agents', 'agenthub_list_skills', 'agenthub_list_tools',
       'agenthub_list_knowledge_bases', 'agenthub_list_executions',
       'agenthub_get_execution_detail', 'agenthub_list_messages',
       'agenthub_list_agent_hooks', 'agenthub_list_agent_versions',
       'document_search', 'agenthub_search_memories'],
 TRUE,
 NOW(), NOW()),

-- optimize-agent: reviews agent config for improvements (like CC's /simplify)
('b1000000-0000-0000-0002-000000000002', 'Optimize Agent', 'optimize-agent',
 'Reviews an agent''s configuration and suggests optimizations for better performance, accuracy, and efficiency. '
 'Use when the user wants to improve an agent''s behavior, reduce token usage, or fix quality issues. '
 'Analyzes: system prompt quality, skill selection, tool binding efficiency, knowledge base coverage, and model config. '
 'Parameters: ''agent_id'' (target agent UUID), ''focus'' (optional: ''prompt'', ''tools'', ''performance'', ''cost''). '
 'Returns a structured report with specific, actionable recommendations ranked by impact.',
 'optimization',
 '{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID to optimize"},"focus":{"type":"string","description":"Focus area: prompt, tools, performance, cost"}},"required":["agent_id"]}',
 ARRAY['agenthub_list_agents', 'agenthub_list_skills', 'agenthub_list_tools',
       'agenthub_list_knowledge_bases', 'agenthub_list_executions',
       'agenthub_list_messages', 'agenthub_list_agent_hooks',
       'document_search'],
 TRUE,
 NOW(), NOW()),

-- onboard-agent: interactive wizard for creating a fully configured agent (like CC's /skillify)
('b1000000-0000-0000-0002-000000000003', 'Onboard Agent', 'onboard-agent',
 'Interactive wizard that guides the user through creating a fully configured agent step by step. '
 'Use when the user wants to create a new agent from scratch or needs help setting one up properly. '
 'Steps: (1) Define purpose and identity, (2) Generate system prompt, (3) Select and bind skills, '
 '(4) Configure knowledge bases, (5) Set model and parameters, (6) Create hooks for automation. '
 'ALWAYS ask for user confirmation at each step before proceeding. '
 'Returns a fully configured agent ready for publishing.',
 'wizard',
 '{"type":"object","properties":{"purpose":{"type":"string","description":"What the agent should do"},"template":{"type":"string","description":"Optional: start from a prompt template slug"}},"required":["purpose"]}',
 '{}',
 TRUE,
 NOW(), NOW()),

-- curate-memory: review and manage agent memories (like CC's /remember)
('b1000000-0000-0000-0002-000000000004', 'Curate Memory', 'curate-memory',
 'Reviews and curates an agent''s long-term memories: identifies duplicates, outdated entries, conflicts, and gaps. '
 'Use when the user wants to clean up an agent''s memory, resolve conflicting memories, or audit what the agent remembers. '
 'Steps: (1) List all memories, (2) Classify by type and relevance, (3) Identify issues, (4) Present structured report. '
 'Parameters: ''agent_id'' (target agent UUID), ''action'' (''review'', ''cleanup'', ''export''). '
 'Returns a memory audit report with recommended actions (keep, update, delete, merge).',
 'memory',
 '{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"},"action":{"type":"string","description":"Action: review, cleanup, export","default":"review"}},"required":["agent_id"]}',
 ARRAY['agenthub_search_memories', 'agenthub_list_agents'],
 TRUE,
 NOW(), NOW()),

-- health-check: platform health and status overview (like CC's /stuck)
('b1000000-0000-0000-0002-000000000005', 'Health Check', 'health-check',
 'Performs a comprehensive health check of the AgentHub platform: agents, knowledge bases, MCP servers, and recent executions. '
 'Use when the user asks about platform status, wants to check if everything is working, or suspects issues. '
 'Checks: (1) Agent status distribution, (2) KB indexing status, (3) MCP server connectivity, '
 '(4) Recent execution success/failure rates, (5) Pending document processing. '
 'Parameters: ''scope'' (optional: ''agents'', ''kbs'', ''mcp'', ''executions'', or ''all''). '
 'Returns a structured health report with status indicators and any detected issues.',
 'diagnostic',
 '{"type":"object","properties":{"scope":{"type":"string","description":"Scope: agents, kbs, mcp, executions, all","default":"all"}}}',
 ARRAY['agenthub_list_agents', 'agenthub_list_skills', 'agenthub_list_tools',
       'agenthub_list_knowledge_bases', 'agenthub_list_mcp_servers',
       'agenthub_list_sessions', 'agenthub_list_executions'],
 TRUE,
 NOW(), NOW()),

-- data-explorer: combined SQL + document search for analysis (like CC's /claude-api multi-source)
('b1000000-0000-0000-0002-000000000006', 'Data Explorer', 'data-explorer',
 'Combines SQL queries and document search to answer complex data questions that span structured and unstructured sources. '
 'Use when the user asks analytical questions that may require both database queries and document lookups. '
 'Automatically determines which data sources to query based on the question. '
 'Parameters: ''question'' (the analytical question), ''datasource_id'' (optional: specific datasource). '
 'Returns a synthesized analysis combining results from all relevant sources with citations.',
 'analysis',
 '{"type":"object","properties":{"question":{"type":"string","description":"Analytical question to answer"},"datasource_id":{"type":"string","description":"Optional: specific datasource UUID"}},"required":["question"]}',
 '{}',
 FALSE,
 NOW(), NOW())

ON CONFLICT (slug) DO NOTHING;


-- =====================================================================
-- 2. New tools: execution monitoring, agent stats, bulk operations
-- =====================================================================
-- These tools support the new diagnostic and optimization skills.

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, search_hint, always_load, created_at, updated_at)
VALUES

-- List recent executions for an agent
('c1000000-0000-0000-0000-000000000054', 'agenthub_list_executions', 'HTTP',
 'Lists recent agent executions with status, duration, and error summary. Use for debugging and monitoring.',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/executions?page={page}&size={size}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"},"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}},"required":["agent_id"]}}',
 ARRAY['platform','executions','diagnostic','read'], TRUE, TRUE, 'list executions runs history errors status', FALSE, NOW(), NOW()),

-- Get execution detail with tool call log
('c1000000-0000-0000-0000-000000000055', 'agenthub_get_execution_detail', 'HTTP',
 'Gets detailed execution log including tool calls, token usage, and errors. Essential for diagnosing failures.',
 '{"method":"GET","urlTemplate":"/api/executions/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"Execution UUID"}},"required":["id"]}}',
 ARRAY['platform','executions','diagnostic','read'], TRUE, TRUE, 'get execution detail log errors tool calls', FALSE, NOW(), NOW()),

-- Get agent statistics (usage, success rate, avg tokens)
('c1000000-0000-0000-0000-000000000056', 'agenthub_get_agent_stats', 'HTTP',
 'Gets usage statistics for an agent: execution count, success rate, average tokens, most used tools.',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/stats","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"}},"required":["agent_id"]}}',
 ARRAY['platform','agents','analytics','read'], TRUE, TRUE, 'agent statistics usage metrics success rate tokens', FALSE, NOW(), NOW()),

-- Duplicate/clone agent with customizations
('c1000000-0000-0000-0000-000000000057', 'agenthub_clone_agent', 'HTTP',
 'Clones an existing agent with all its skills, KBs, and hooks. Optionally customize name and prompt. ALWAYS confirm with user.',
 '{"method":"POST","urlTemplate":"/api/agents/{agent_id}/clone","useCallerToken":true,"bodyTemplate":{"name":"{name}","systemPrompt":"{system_prompt}"},"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Source agent UUID"},"name":{"type":"string","description":"Name for the cloned agent"},"system_prompt":{"type":"string","description":"Optional: override system prompt"}},"required":["agent_id","name"]}}',
 ARRAY['platform','agents','write'], FALSE, TRUE, 'clone duplicate copy agent', FALSE, NOW(), NOW()),

-- Bulk update agent status (publish/archive multiple)
('c1000000-0000-0000-0000-000000000058', 'agenthub_bulk_agent_status', 'HTTP',
 'Updates the status of multiple agents at once (publish, archive, or activate). ALWAYS confirm with user first.',
 '{"method":"POST","urlTemplate":"/api/agents/bulk-status","useCallerToken":true,"bodyTemplate":{"ids":"{agent_ids}","status":"{status}"},"inputSchema":{"type":"object","properties":{"agent_ids":{"type":"array","items":{"type":"string"},"description":"Array of agent UUIDs"},"status":{"type":"string","enum":["PUBLISHED","ARCHIVED","DRAFT"],"description":"Target status"}},"required":["agent_ids","status"]}}',
 ARRAY['platform','agents','write','bulk'], FALSE, TRUE, 'bulk update agent status publish archive', FALSE, NOW(), NOW()),

-- Search memories (semantic search across agent memories)
('c1000000-0000-0000-0000-000000000059', 'agenthub_search_memories', 'HTTP',
 'Searches the agent''s long-term memory by semantic similarity. Returns relevant memories with timestamps and scores.',
 '{"method":"POST","urlTemplate":"/api/memory/search","useCallerToken":true,"bodyTemplate":{"query":"{query}","agentId":"{agent_id}","limit":"{limit}"},"inputSchema":{"type":"object","properties":{"query":{"type":"string","description":"Semantic search query"},"agent_id":{"type":"string","description":"Agent UUID"},"limit":{"type":"integer","default":10,"description":"Max results"}},"required":["query","agent_id"]}}',
 ARRAY['platform','memory','read'], TRUE, FALSE, NULL, FALSE, NOW(), NOW()),

-- Store memory (persist a new memory entry)
('c1000000-0000-0000-0000-000000000060', 'agenthub_store_memory', 'HTTP',
 'Stores a new entry in the agent''s long-term memory. Use to persist important facts, preferences, or decisions.',
 '{"method":"POST","urlTemplate":"/api/memory/store","useCallerToken":true,"bodyTemplate":{"content":"{content}","agentId":"{agent_id}","type":"{type}"},"inputSchema":{"type":"object","properties":{"content":{"type":"string","description":"Memory content to store"},"agent_id":{"type":"string","description":"Agent UUID"},"type":{"type":"string","description":"Memory type: fact, preference, decision, context","default":"fact"}},"required":["content","agent_id"]}}',
 ARRAY['platform','memory','write'], FALSE, TRUE, 'store save remember memory persist', FALSE, NOW(), NOW()),

-- Delete memory
('c1000000-0000-0000-0000-000000000061', 'agenthub_delete_memory', 'HTTP',
 'Deletes a specific memory entry. ALWAYS confirm with user before deleting memories.',
 '{"method":"DELETE","urlTemplate":"/api/memory/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"Memory UUID"}},"required":["id"]}}',
 ARRAY['platform','memory','write'], FALSE, TRUE, 'delete remove forget memory', FALSE, NOW(), NOW()),

-- Get KB indexing status (document processing pipeline status)
('c1000000-0000-0000-0000-000000000062', 'agenthub_get_kb_status', 'HTTP',
 'Gets detailed indexing status for a knowledge base: pending, processing, indexed, and failed document counts.',
 '{"method":"GET","urlTemplate":"/api/knowledge-bases/{id}/status","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"Knowledge base UUID"}},"required":["id"]}}',
 ARRAY['platform','knowledge-bases','diagnostic','read'], TRUE, TRUE, 'knowledge base indexing status documents pending', FALSE, NOW(), NOW()),

-- List agent skill bindings (what skills an agent has)
('c1000000-0000-0000-0000-000000000063', 'agenthub_get_agent_skills', 'HTTP',
 'Lists all skills bound to a specific agent with their descriptions and tool counts.',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/skills","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"}},"required":["agent_id"]}}',
 ARRAY['platform','agents','skills','read'], TRUE, FALSE, NULL, FALSE, NOW(), NOW()),

-- List agent knowledge base bindings
('c1000000-0000-0000-0000-000000000064', 'agenthub_get_agent_kbs', 'HTTP',
 'Lists all knowledge bases bound to a specific agent with their indexing status and document counts.',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/knowledge-bases","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"}},"required":["agent_id"]}}',
 ARRAY['platform','agents','knowledge-bases','read'], TRUE, FALSE, NULL, FALSE, NOW(), NOW())

ON CONFLICT (id) DO NOTHING;


-- Mark destructive tools
UPDATE tool SET is_destructive = TRUE
WHERE name IN ('agenthub_bulk_agent_status', 'agenthub_clone_agent', 'agenthub_delete_memory');


-- =====================================================================
-- 3. Bind new tools to new skills
-- =====================================================================

-- debug-agent skill: diagnostic tools (all read-only)
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000001', 'c1000000-0000-0000-0000-000000000054', 100, true, NOW()),  -- list_executions
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000001', 'c1000000-0000-0000-0000-000000000055', 101, true, NOW()),  -- get_execution_detail
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000001', 'c1000000-0000-0000-0000-000000000056', 102, true, NOW()),  -- get_agent_stats
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000001', 'c1000000-0000-0000-0000-000000000053', 103, true, NOW()),  -- list_messages
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000001', 'c1000000-0000-0000-0000-000000000063', 104, true, NOW()),  -- get_agent_skills
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000001', 'c1000000-0000-0000-0000-000000000064', 105, true, NOW()),  -- get_agent_kbs
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000001', 'c1000000-0000-0000-0000-000000000059', 106, true, NOW())   -- search_memories
ON CONFLICT DO NOTHING;

-- optimize-agent skill: same read-only tools plus stats
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000002', 'c1000000-0000-0000-0000-000000000054', 100, true, NOW()),  -- list_executions
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000002', 'c1000000-0000-0000-0000-000000000056', 101, true, NOW()),  -- get_agent_stats
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000002', 'c1000000-0000-0000-0000-000000000053', 102, true, NOW()),  -- list_messages
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000002', 'c1000000-0000-0000-0000-000000000063', 103, true, NOW()),  -- get_agent_skills
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000002', 'c1000000-0000-0000-0000-000000000064', 104, true, NOW()),  -- get_agent_kbs
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000002', 'c1000000-0000-0000-0000-000000000062', 105, true, NOW())   -- get_kb_status
ON CONFLICT DO NOTHING;

-- onboard-agent skill: needs create/update tools for building the agent
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'a1000000-0000-0000-0001-000000000001', 100, true, NOW()),  -- list_agents
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'a1000000-0000-0000-0001-000000000003', 101, true, NOW()),  -- create_agent
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'a1000000-0000-0000-0001-000000000004', 102, true, NOW()),  -- update_agent
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'a1000000-0000-0000-0002-000000000001', 103, true, NOW()),  -- list_skills
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'a1000000-0000-0000-0004-000000000001', 104, true, NOW()),  -- list_kbs
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'c1000000-0000-0000-0000-000000000032', 105, true, NOW()),  -- sync_agent_skills
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'c1000000-0000-0000-0000-000000000034', 106, true, NOW()),  -- sync_agent_kbs
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'a1000000-0000-0000-0009-000000000001', 107, true, NOW()),  -- list_prompt_templates
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000003', 'a1000000-0000-0000-0001-000000000006', 108, true, NOW())   -- publish_agent
ON CONFLICT DO NOTHING;

-- curate-memory skill: memory read/write tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000004', 'c1000000-0000-0000-0000-000000000059', 100, true, NOW()),  -- search_memories
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000004', 'c1000000-0000-0000-0000-000000000060', 101, true, NOW()),  -- store_memory
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000004', 'c1000000-0000-0000-0000-000000000061', 102, true, NOW())   -- delete_memory
ON CONFLICT DO NOTHING;

-- health-check skill: platform-wide read-only tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000005', 'a1000000-0000-0000-0001-000000000001', 100, true, NOW()),  -- list_agents
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000005', 'a1000000-0000-0000-0002-000000000001', 101, true, NOW()),  -- list_skills
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000005', 'a1000000-0000-0000-0003-000000000001', 102, true, NOW()),  -- list_tools
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000005', 'a1000000-0000-0000-0004-000000000001', 103, true, NOW()),  -- list_kbs
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000005', 'a1000000-0000-0000-0006-000000000001', 104, true, NOW()),  -- list_mcp_servers
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000005', 'c1000000-0000-0000-0000-000000000054', 105, true, NOW()),  -- list_executions
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000005', 'c1000000-0000-0000-0000-000000000062', 106, true, NOW())   -- get_kb_status
ON CONFLICT DO NOTHING;

-- data-explorer skill: combines SQL + document search + memory
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000006', 'a1000000-0000-0000-0008-000000000001', 100, true, NOW()),  -- list_datasources
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000006', 'c1000000-0000-0000-0000-000000000038', 101, true, NOW()),  -- document_search
(gen_random_uuid(), 'b1000000-0000-0000-0002-000000000006', 'c1000000-0000-0000-0000-000000000059', 102, true, NOW())   -- search_memories
ON CONFLICT DO NOTHING;


-- =====================================================================
-- 4. Bind new skills to AgentHub Assistant agent
-- =====================================================================
-- The AgentHub Assistant (d1000000-0000-0000-0001-000000000001) should have
-- access to all new diagnostic and management skills.

INSERT INTO agent_skill (agent_id, skill_id, created_at) VALUES
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000001', NOW()),  -- debug-agent
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000002', NOW()),  -- optimize-agent
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000003', NOW()),  -- onboard-agent
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000004', NOW()),  -- curate-memory
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000005', NOW()),  -- health-check
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0002-000000000006', NOW())   -- data-explorer
ON CONFLICT DO NOTHING;


-- =====================================================================
-- 5. Bind new tools to existing platform management skills
-- =====================================================================
-- Existing skills need access to the new tools where relevant.

-- agent-management: add clone, bulk-status, agent-stats, get-agent-skills, get-agent-kbs
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000057', 120, true, NOW()),  -- clone_agent
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000058', 121, true, NOW()),  -- bulk_agent_status
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000056', 122, true, NOW()),  -- get_agent_stats
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000063', 123, true, NOW()),  -- get_agent_skills
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000064', 124, true, NOW())   -- get_agent_kbs
ON CONFLICT DO NOTHING;

-- knowledge-base-management: add KB status tool
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000062', 114, true, NOW())   -- get_kb_status
ON CONFLICT DO NOTHING;

-- chat-management: add execution listing for context
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at) VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000054', 113, true, NOW())   -- list_executions
ON CONFLICT DO NOTHING;


-- =====================================================================
-- 6. Enrich existing skill descriptions with LLM-optimized text
-- =====================================================================
-- Update descriptions that are shorter than the enriched versions.
-- This ensures the LLM gets better guidance on when/how to use each skill.

UPDATE skill SET description = 'Searches documents in the knowledge base by semantic similarity. Use when the user asks about information that may exist in uploaded documents or company knowledge bases. The ''query'' parameter should be a descriptive sentence or question, not isolated keywords — for example, use ''how to reset a user password'' instead of ''password reset''. Returns relevant text excerpts with similarity scores, sorted by relevance.', updated_at = NOW()
  WHERE slug = 'document-search' AND length(description) < 305;

UPDATE skill SET description = 'Executes SQL queries against configured PostgreSQL datasources. Use when the user needs to query, analyze, or explore structured data. The ''query'' parameter must be valid SQL. Always use SELECT for reads; NEVER execute UPDATE, DELETE, or DROP without explicit user confirmation. Use LIMIT to avoid returning excessive rows. Returns rows as an array of objects with column names as keys.', updated_at = NOW()
  WHERE slug = 'execute-sql' AND length(description) < 315;

UPDATE skill SET description = 'Auto-diagnoses errors and failures during tool execution. Use when a previous tool call failed and the user needs help understanding what went wrong. Analyzes the error, suggests fixes, and optionally retries with corrected parameters. Parameters: ''error'' (the error message), ''tool_name'' (which tool failed), ''original_input'' (what was sent). Returns diagnosis and suggested next steps.', updated_at = NOW()
  WHERE slug = 'troubleshoot' AND length(description) < 320;

UPDATE skill SET description = 'Searches the agent''s long-term memory for relevant facts, preferences, and decisions from past conversations. Use when the user references something discussed before, or when context from previous sessions would improve the response. Parameters: ''query'' (semantic search query), ''limit'' (max results). Returns matching memories with timestamps and relevance scores.', updated_at = NOW()
  WHERE slug = 'memory-recall' AND length(description) < 310;


-- =====================================================================
-- 7. Update tool deferred loading for new tools
-- =====================================================================
-- Tools that are diagnostic/monitoring should be deferred by default
-- (only loaded when user invokes a diagnostic skill).
-- Memory tools should NOT be deferred — the LLM uses them frequently.

UPDATE tool SET should_defer = TRUE
WHERE name IN (
    'agenthub_list_executions',
    'agenthub_get_execution_detail',
    'agenthub_get_agent_stats',
    'agenthub_clone_agent',
    'agenthub_bulk_agent_status',
    'agenthub_delete_memory',
    'agenthub_get_kb_status'
);

-- Memory search/store and agent skill/KB listing should NOT be deferred.
UPDATE tool SET should_defer = FALSE
WHERE name IN (
    'agenthub_search_memories',
    'agenthub_store_memory',
    'agenthub_get_agent_skills',
    'agenthub_get_agent_kbs'
);
