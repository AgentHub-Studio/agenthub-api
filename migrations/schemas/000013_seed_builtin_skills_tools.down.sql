-- Reverse seed data from 000013_seed_builtin_skills_tools.up.sql

-- Remove agent-skill bindings for AgentHub Assistant
DELETE FROM agent_skill WHERE agent_id = 'd1000000-0000-0000-0001-000000000001';

-- Remove AgentHub Assistant agent
DELETE FROM agent WHERE id = 'd1000000-0000-0000-0001-000000000001';

-- Remove prompt templates
DELETE FROM prompt_template WHERE id IN (
    'e1000000-0000-0000-0001-000000000001',
    'e1000000-0000-0000-0001-000000000002',
    'e1000000-0000-0000-0001-000000000003',
    'e1000000-0000-0000-0001-000000000004',
    'e1000000-0000-0000-0001-000000000005'
);

-- Remove skill-tool bindings (cascade from skill/tool deletes handles most,
-- but explicit cleanup for safety)
DELETE FROM skill_tool WHERE skill_id IN (
    SELECT id FROM skill WHERE id::text LIKE 'b1000000-0000-0000-0001-%'
);

-- Remove platform management tools
DELETE FROM tool WHERE id::text LIKE 'a1000000-0000-0000-000_-%';

-- Remove platform management skills
DELETE FROM skill WHERE id::text LIKE 'b1000000-0000-0000-0001-%';

-- Remove core generic skills
DELETE FROM skill WHERE id::text LIKE 'b1000000-0000-0000-0000-%';
