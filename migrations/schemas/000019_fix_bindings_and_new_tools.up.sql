-- Fix skill_tool bindings from migration 000015 that used tool IDs (a1000000-)
-- instead of skill IDs (b1000000-) in the skill_id column.
-- Also adds missing tools, new skills, and enhanced hook schema.

-- =====================================================================
-- 1. Fix wrong skill_tool bindings from 000015
-- =====================================================================
-- The bindings below were inserted with tool IDs as skill_id, which either
-- violated FK constraints or silently failed via ON CONFLICT DO NOTHING.
-- Remove any that slipped through, then re-insert with correct skill IDs.

-- Remove incorrect bindings (if FK was not enforced or IDs coincidentally matched).
DELETE FROM skill_tool
WHERE skill_id IN (
    'a1000000-0000-0000-0000-000000000001',  -- was tool, not skill
    'a1000000-0000-0000-0000-000000000004',
    'a1000000-0000-0000-0000-000000000007'
) AND tool_id IN (
    'c1000000-0000-0000-0000-000000000032',
    'c1000000-0000-0000-0000-000000000033',
    'c1000000-0000-0000-0000-000000000034',
    'c1000000-0000-0000-0000-000000000035',
    'c1000000-0000-0000-0000-000000000036',
    'c1000000-0000-0000-0000-000000000038'
);

-- Re-insert with correct skill IDs.
-- agent-management skill (b1000000-0000-0000-0001-000000000001): binding tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000032', 110, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000033', 111, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000034', 112, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000035', 113, true, NOW())
ON CONFLICT DO NOTHING;

-- chat-management skill (b1000000-0000-0000-0001-000000000007): run tool
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000036', 110, true, NOW())
ON CONFLICT DO NOTHING;

-- knowledge-base-management skill (b1000000-0000-0000-0001-000000000004): doc search
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000038', 110, true, NOW())
ON CONFLICT DO NOTHING;


-- =====================================================================
-- 2. New tools: update operations missing from 000013
-- =====================================================================
-- Several CRUD resources only had list/get/create/delete but no UPDATE tool.
-- The LLM needs these to modify existing resources without deleting and recreating.

INSERT INTO tool (id, name, type, description, config, labels, read_only, should_defer, search_hint, created_at, updated_at)
VALUES
-- Update skill
('c1000000-0000-0000-0000-000000000041', 'agenthub_update_skill', 'HTTP',
 'Updates an existing skill. ALWAYS confirm changes with the user.',
 '{"method":"PUT","urlTemplate":"/api/skills/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"Skill UUID"},"name":{"type":"string"},"description":{"type":"string"},"category":{"type":"string"},"inputSchema":{"type":"object"}},"required":["id"]}}',
 ARRAY['platform','skills','write'], FALSE, TRUE, 'update modify skill capability', NOW(), NOW()),

-- Update tool
('c1000000-0000-0000-0000-000000000042', 'agenthub_update_tool', 'HTTP',
 'Updates an existing tool. ALWAYS confirm changes with the user.',
 '{"method":"PUT","urlTemplate":"/api/tools/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"Tool UUID"},"name":{"type":"string"},"description":{"type":"string"},"type":{"type":"string"},"config":{"type":"object"}},"required":["id"]}}',
 ARRAY['platform','tools','write'], FALSE, TRUE, 'update modify tool implementation', NOW(), NOW()),

-- Update knowledge base
('c1000000-0000-0000-0000-000000000043', 'agenthub_update_knowledge_base', 'HTTP',
 'Updates an existing knowledge base. ALWAYS confirm changes with the user.',
 '{"method":"PUT","urlTemplate":"/api/knowledge-bases/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"KB UUID"},"name":{"type":"string"},"description":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','knowledge-bases','write'], FALSE, TRUE, 'update modify knowledge base', NOW(), NOW()),

-- Sync knowledge base (trigger reindexing)
('c1000000-0000-0000-0000-000000000044', 'agenthub_sync_knowledge_base', 'HTTP',
 'Triggers reindexing of a knowledge base. Use after adding or updating documents.',
 '{"method":"POST","urlTemplate":"/api/knowledge-bases/{id}/sync","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"KB UUID"}},"required":["id"]}}',
 ARRAY['platform','knowledge-bases','write'], FALSE, TRUE, 'sync reindex knowledge base documents', NOW(), NOW()),

-- Update MCP server
('c1000000-0000-0000-0000-000000000045', 'agenthub_update_mcp_server', 'HTTP',
 'Updates an MCP server configuration. ALWAYS confirm changes with the user.',
 '{"method":"PUT","urlTemplate":"/api/mcp-server-configs/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"MCP config UUID"},"name":{"type":"string"},"httpBaseUrl":{"type":"string"},"autoStart":{"type":"boolean"},"enabled":{"type":"boolean"}},"required":["id"]}}',
 ARRAY['platform','mcp','write'], FALSE, TRUE, 'update modify mcp server config', NOW(), NOW()),

-- Delete MCP server
('c1000000-0000-0000-0000-000000000046', 'agenthub_delete_mcp_server', 'HTTP',
 'Deletes an MCP server configuration. ALWAYS confirm with user.',
 '{"method":"DELETE","urlTemplate":"/api/mcp-server-configs/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"MCP config UUID"}},"required":["id"]}}',
 ARRAY['platform','mcp','write'], FALSE, TRUE, 'delete remove mcp server', NOW(), NOW()),

-- Get MCP server
('c1000000-0000-0000-0000-000000000047', 'agenthub_get_mcp_server', 'HTTP',
 'Gets detailed MCP server configuration by ID.',
 '{"method":"GET","urlTemplate":"/api/mcp-server-configs/{id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"MCP config UUID"}},"required":["id"]}}',
 ARRAY['platform','mcp','read'], TRUE, FALSE, NULL, NOW(), NOW()),

-- Upload document to KB
('c1000000-0000-0000-0000-000000000048', 'agenthub_upload_document', 'HTTP',
 'Uploads a document to a knowledge base for indexing.',
 '{"method":"POST","urlTemplate":"/api/knowledge-bases/{kb_id}/documents","useCallerToken":true,"contentType":"multipart/form-data","inputSchema":{"type":"object","properties":{"kb_id":{"type":"string","description":"Knowledge base UUID"},"file":{"type":"string","description":"File to upload (multipart)"}},"required":["kb_id","file"]}}',
 ARRAY['platform','knowledge-bases','documents','write'], FALSE, TRUE, 'upload add document file knowledge base', NOW(), NOW()),

-- Delete agent hook
('c1000000-0000-0000-0000-000000000049', 'agenthub_delete_agent_hook', 'HTTP',
 'Deletes a hook from an agent. ALWAYS confirm with user.',
 '{"method":"DELETE","urlTemplate":"/api/agents/{agent_id}/hooks/{hook_id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"},"hook_id":{"type":"string","description":"Hook UUID"}},"required":["agent_id","hook_id"]}}',
 ARRAY['platform','agents','hooks','write'], FALSE, TRUE, 'delete remove agent hook', NOW(), NOW()),

-- Update agent hook
('c1000000-0000-0000-0000-000000000050', 'agenthub_update_agent_hook', 'HTTP',
 'Updates a hook on an agent. ALWAYS confirm changes with user.',
 '{"method":"PUT","urlTemplate":"/api/agents/{agent_id}/hooks/{hook_id}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"},"hook_id":{"type":"string","description":"Hook UUID"},"event":{"type":"string"},"matcher":{"type":"string"},"hookType":{"type":"string"},"config":{"type":"object"},"enabled":{"type":"boolean"}},"required":["agent_id","hook_id"]}}',
 ARRAY['platform','agents','hooks','write'], FALSE, TRUE, 'update modify agent hook', NOW(), NOW()),

-- List agent versions
('c1000000-0000-0000-0000-000000000051', 'agenthub_list_agent_versions', 'HTTP',
 'Lists all versions of an agent.',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/versions","useCallerToken":true,"inputSchema":{"type":"object","properties":{"agent_id":{"type":"string","description":"Agent UUID"}},"required":["agent_id"]}}',
 ARRAY['platform','agents','versions','read'], TRUE, TRUE, 'list agent versions history', NOW(), NOW()),

-- Archive session
('c1000000-0000-0000-0000-000000000052', 'agenthub_archive_session', 'HTTP',
 'Archives a chat session.',
 '{"method":"POST","urlTemplate":"/api/chat/sessions/{id}/archive","useCallerToken":true,"inputSchema":{"type":"object","properties":{"id":{"type":"string","description":"Session UUID"}},"required":["id"]}}',
 ARRAY['platform','chat','write'], FALSE, TRUE, 'archive close chat session', NOW(), NOW()),

-- List session messages
('c1000000-0000-0000-0000-000000000053', 'agenthub_list_messages', 'HTTP',
 'Lists messages in a chat session with pagination.',
 '{"method":"GET","urlTemplate":"/api/chat/sessions/{session_id}/messages?page={page}&size={size}","useCallerToken":true,"inputSchema":{"type":"object","properties":{"session_id":{"type":"string","description":"Session UUID"},"page":{"type":"integer","default":0},"size":{"type":"integer","default":50}},"required":["session_id"]}}',
 ARRAY['platform','chat','read'], TRUE, TRUE, 'list messages chat history conversation', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Mark new destructive tools
UPDATE tool SET is_destructive = TRUE WHERE name IN (
    'agenthub_delete_mcp_server',
    'agenthub_delete_agent_hook'
);


-- =====================================================================
-- 3. Bind new tools to existing skills
-- =====================================================================

-- skill-management: add update tool
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000002', 'c1000000-0000-0000-0000-000000000041', 110, true, NOW())
ON CONFLICT DO NOTHING;

-- tool-management: add update tool
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000003', 'c1000000-0000-0000-0000-000000000042', 110, true, NOW())
ON CONFLICT DO NOTHING;

-- knowledge-base-management: add update, sync, upload tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000043', 111, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000044', 112, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000048', 113, true, NOW())
ON CONFLICT DO NOTHING;

-- mcp-management: add get, update, delete tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000006', 'c1000000-0000-0000-0000-000000000047', 102, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000006', 'c1000000-0000-0000-0000-000000000045', 110, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000006', 'c1000000-0000-0000-0000-000000000046', 111, true, NOW())
ON CONFLICT DO NOTHING;

-- agent-management: add hook delete, hook update, versions tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000049', 116, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000050', 117, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000051', 118, true, NOW())
ON CONFLICT DO NOTHING;

-- chat-management: add archive, list messages tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000052', 111, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000053', 112, true, NOW())
ON CONFLICT DO NOTHING;


-- =====================================================================
-- 4. Add allowed_tools column to skill table
-- =====================================================================
-- Inspired by Claude Code's BundledSkillDefinition.allowedTools — restricts
-- which tools a skill can use when invoked. Empty array = all tools allowed.
-- This enables fine-grained security: e.g. "troubleshoot" skill only needs
-- read-only tools, not write operations.

ALTER TABLE skill
  ADD COLUMN IF NOT EXISTS allowed_tools TEXT[] DEFAULT '{}';

-- Set allowed_tools for skills that should have restricted tool access.
-- troubleshoot: read-only diagnosis, should not modify anything.
UPDATE skill SET allowed_tools = ARRAY['document_search', 'memory_store', 'agenthub_search_memories']
WHERE slug = 'troubleshoot';

-- memory-recall: only needs memory search.
UPDATE skill SET allowed_tools = ARRAY['agenthub_search_memories']
WHERE slug = 'memory-recall';


-- =====================================================================
-- 5. Enhance agent_hook table with Claude Code hook patterns
-- =====================================================================
-- Inspired by Claude Code's hook schemas: async, once, timeout per hook,
-- and 'if' condition for filtering based on tool name patterns.

ALTER TABLE agent_hook
  ADD COLUMN IF NOT EXISTS timeout_seconds INTEGER,
  ADD COLUMN IF NOT EXISTS is_async BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS run_once BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS status_message VARCHAR(200);

-- Add new hook events from Claude Code patterns.
-- Existing events: pre_tool_use, post_tool_use, session_start, session_end
-- New events: notification (Claude Code's Notification hook), post_tool_failure
COMMENT ON COLUMN agent_hook.event IS
  'Hook event: pre_tool_use, post_tool_use, post_tool_failure, session_start, session_end, notification';
