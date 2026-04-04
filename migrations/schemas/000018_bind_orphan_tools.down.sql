-- Reverse migration 000017: remove orphan tool bindings.

-- Remove hook tools from agent-management.
DELETE FROM skill_tool
WHERE skill_id = 'b1000000-0000-0000-0001-000000000001'
  AND tool_id IN ('c1000000-0000-0000-0000-000000000039', 'c1000000-0000-0000-0000-000000000040');

-- Remove memory search tool from memory-recall skill.
DELETE FROM skill_tool
WHERE skill_id = 'b1000000-0000-0000-0000-000000000010'
  AND tool_id = 'c1000000-0000-0000-0000-000000000037';
