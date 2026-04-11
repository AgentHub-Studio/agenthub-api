DELETE FROM ah_core.skill_tool WHERE tool_id IN (SELECT id FROM ah_core.tool WHERE slug LIKE 'core-%');
DELETE FROM ah_core.tool WHERE slug LIKE 'core-%';
