-- Fix orphan tools from migration 000015 that were created but not bound to skills.
-- Also adds missing skill-tool bindings for memory and hook management.

-- =====================================================================
-- 1. Bind memory tools to a platform memory-management skill
-- =====================================================================

-- The memory-recall skill (b1...10) exists but agenthub_search_memories
-- tool was not bound to it.
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0000-000000000010', 'c1000000-0000-0000-0000-000000000037', 100, true, NOW())
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 2. Bind hook tools to agent-management skill
-- =====================================================================

-- Hook management is part of agent configuration, so bind to agent-management
-- skill (b1000000-0000-0000-0001-000000000001 = agent-management).
INSERT INTO skill_tool (id, skill_id, tool_id, priority, is_active, created_at)
VALUES
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000039', 114, true, NOW()),
(gen_random_uuid(), 'b1000000-0000-0000-0001-000000000001', 'c1000000-0000-0000-0000-000000000040', 115, true, NOW())
ON CONFLICT DO NOTHING;
