-- Add tools for the new agent binding endpoints (agent_skill / agent_knowledge_base)
-- and missing skills from the skilldesc.go catalog.
-- Also adds tools for chat, memory, document search, and hook management.

-- =====================================================================
-- NEW SKILLS: file-upload, calendar-event (from skilldesc.go catalog)
-- =====================================================================

INSERT INTO skill (id, name, slug, description, category, input_schema, created_at, updated_at)
VALUES
('b1000000-0000-0000-0000-000000000007', 'File Upload', 'file-upload',
 'Uploads a file to the agent''s storage (MinIO). Use when the user wants to save, export, or share a generated file. Parameters: ''filename'', ''content'' (base64-encoded), ''content_type'' (MIME type). Returns the file URL and metadata (size, type, upload timestamp).',
 'storage', '{"type":"object","properties":{"filename":{"type":"string","description":"Name of the file"},"content":{"type":"string","description":"Base64-encoded file content"},"content_type":{"type":"string","description":"MIME type (e.g. application/pdf)"}},"required":["filename","content"]}', NOW(), NOW()),
('b1000000-0000-0000-0000-000000000008', 'Calendar Event', 'calendar-event',
 'Creates, reads, or updates calendar events. Use when the user asks about scheduling, meetings, or availability. Parameters: ''action'' (create/list/update), ''title'', ''start_time'', ''end_time'' (ISO 8601), ''attendees''. ALWAYS confirm event details with the user before creating or modifying. Returns event details including any conflicts detected.',
 'productivity', '{"type":"object","properties":{"action":{"type":"string","enum":["create","list","update"],"description":"Action to perform"},"title":{"type":"string","description":"Event title"},"start_time":{"type":"string","description":"Start time (ISO 8601)"},"end_time":{"type":"string","description":"End time (ISO 8601)"},"attendees":{"type":"array","items":{"type":"string"},"description":"Email addresses of attendees"}},"required":["action"]}', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- NEW SKILLS: troubleshoot, memory-recall (inspired by Claude Code)
-- =====================================================================

INSERT INTO skill (id, name, slug, description, category, input_schema, created_at, updated_at)
VALUES
('b1000000-0000-0000-0000-000000000009', 'Troubleshoot', 'troubleshoot',
 'Auto-diagnoses errors and failures during tool execution. Use when a previous tool call failed and the user needs help understanding what went wrong. Analyzes the error, suggests fixes, and optionally retries with corrected parameters. Parameters: ''error'' (the error message), ''tool_name'' (which tool failed), ''original_input'' (what was sent). Returns diagnosis and suggested next steps.',
 'system', '{"type":"object","properties":{"error":{"type":"string","description":"Error message from the failed tool"},"tool_name":{"type":"string","description":"Name of the tool that failed"},"original_input":{"type":"object","description":"Original input that caused the failure"}},"required":["error","tool_name"]}', NOW(), NOW()),
('b1000000-0000-0000-0000-000000000010', 'Memory Recall', 'memory-recall',
 'Searches the agent''s long-term memory for relevant facts, preferences, and decisions from past conversations. Use when the user references something discussed before, or when context from previous sessions would improve the response. Parameters: ''query'' (semantic search query), ''limit'' (max results). Returns matching memories with timestamps and relevance scores.',
 'system', '{"type":"object","properties":{"query":{"type":"string","description":"Semantic search query for memories"},"limit":{"type":"integer","description":"Max results (default 5)"}},"required":["query"]}', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- NEW TOOLS: Agent binding management (GET/PUT skills & KBs)
-- =====================================================================

INSERT INTO tool (id, name, type, config, description, labels, read_only, created_at, updated_at)
VALUES
-- Agent-Skill bindings
('c1000000-0000-0000-0000-000000000032', 'agenthub_list_agent_skills', 'HTTP',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/skills","useCallerToken":true}',
 'Lists skill IDs bound to an agent via the agent_skill join table.',
 ARRAY['platform','agents','skills','read'], TRUE, NOW(), NOW()),
('c1000000-0000-0000-0000-000000000033', 'agenthub_sync_agent_skills', 'HTTP',
 '{"method":"PUT","urlTemplate":"/api/agents/{agent_id}/skills","useCallerToken":true,"bodyTemplate":{"ids":"{skill_ids}"}}',
 'Replaces all skill bindings for an agent. Send array of skill UUIDs.',
 ARRAY['platform','agents','skills','write'], FALSE, NOW(), NOW()),
-- Agent-Knowledge Base bindings
('c1000000-0000-0000-0000-000000000034', 'agenthub_list_agent_kbs', 'HTTP',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/knowledge-bases","useCallerToken":true}',
 'Lists knowledge base IDs bound to an agent.',
 ARRAY['platform','agents','knowledge-bases','read'], TRUE, NOW(), NOW()),
('c1000000-0000-0000-0000-000000000035', 'agenthub_sync_agent_kbs', 'HTTP',
 '{"method":"PUT","urlTemplate":"/api/agents/{agent_id}/knowledge-bases","useCallerToken":true,"bodyTemplate":{"ids":"{kb_ids}"}}',
 'Replaces all knowledge base bindings for an agent.',
 ARRAY['platform','agents','knowledge-bases','write'], FALSE, NOW(), NOW()),
-- Chat run (trigger agentic execution)
('c1000000-0000-0000-0000-000000000036', 'agenthub_run_chat', 'HTTP',
 '{"method":"POST","urlTemplate":"/api/chat/sessions/{session_id}/run","useCallerToken":true,"bodyTemplate":{"message":"{message}"},"accept":"text/event-stream"}',
 'Sends a message to a chat session and triggers the agentic loop. Returns SSE stream of events (text_delta, tool_call_start, tool_result, turn_complete, run_complete).',
 ARRAY['platform','chat','write'], FALSE, NOW(), NOW()),
-- Memory search
('c1000000-0000-0000-0000-000000000037', 'agenthub_search_memories', 'HTTP',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/memories?query={query}&limit={limit}","useCallerToken":true}',
 'Searches agent memories by semantic similarity.',
 ARRAY['platform','agents','memory','read'], TRUE, NOW(), NOW()),
-- Document search
('c1000000-0000-0000-0000-000000000038', 'agenthub_search_documents', 'HTTP',
 '{"method":"POST","urlTemplate":"/api/knowledge-bases/{kb_id}/search","useCallerToken":true,"bodyTemplate":{"query":"{query}","limit":"{limit}"}}',
 'Searches documents in a specific knowledge base by semantic similarity.',
 ARRAY['platform','knowledge-bases','rag','read'], TRUE, NOW(), NOW()),
-- Agent hooks
('c1000000-0000-0000-0000-000000000039', 'agenthub_list_agent_hooks', 'HTTP',
 '{"method":"GET","urlTemplate":"/api/agents/{agent_id}/hooks","useCallerToken":true}',
 'Lists hooks configured for an agent.',
 ARRAY['platform','agents','hooks','read'], TRUE, NOW(), NOW()),
('c1000000-0000-0000-0000-000000000040', 'agenthub_create_agent_hook', 'HTTP',
 '{"method":"POST","urlTemplate":"/api/agents/{agent_id}/hooks","useCallerToken":true,"bodyTemplate":{"event":"{event}","matcher":"{matcher}","hookType":"{hook_type}","config":"{config}"}}',
 'Creates a new hook for an agent. Events: pre_tool_use, post_tool_use, session_start, session_end.',
 ARRAY['platform','agents','hooks','write'], FALSE, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- BIND NEW TOOLS TO EXISTING PLATFORM SKILLS
-- =====================================================================

-- agent-management: add binding tools
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000032', 110, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000033', 111, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000034', 112, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000035', 113, true, NOW())
ON CONFLICT DO NOTHING;

-- chat-management: add run tool
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000007', 'c1000000-0000-0000-0000-000000000036', 110, true, NOW())
ON CONFLICT DO NOTHING;

-- knowledge-base-management: add document search tool
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000004', 'c1000000-0000-0000-0000-000000000038', 110, true, NOW())
ON CONFLICT DO NOTHING;

-- =====================================================================
-- BIND NEW SKILLS TO AGENTHUB ASSISTANT AGENT
-- =====================================================================

-- The AgentHub Assistant (d1000000-0000-0000-0001-000000000001) should also
-- have the troubleshoot and memory-recall skills available.
INSERT INTO agent_skill (agent_id, skill_id, created_at)
VALUES
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0000-000000000009', NOW()),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0000-000000000010', NOW())
ON CONFLICT DO NOTHING;

-- =====================================================================
-- UPDATE READ-ONLY FLAGS FOR NEW TOOLS
-- =====================================================================

-- Already set via column default in INSERT above, but ensure consistency.
UPDATE tool SET read_only = TRUE WHERE name IN (
    'agenthub_list_agent_skills',
    'agenthub_list_agent_kbs',
    'agenthub_search_memories',
    'agenthub_search_documents',
    'agenthub_list_agent_hooks'
) AND read_only = FALSE;
