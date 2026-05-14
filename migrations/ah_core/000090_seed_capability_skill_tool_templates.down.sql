-- Remove capability skill-tool bindings
DELETE FROM ah_core.skill_tool
WHERE skill_id IN (
    SELECT id FROM ah_core.skill
     WHERE slug IN ('core-web-research', 'core-doc-analysis', 'core-task-workflow')
);

-- Remove capability skills
DELETE FROM ah_core.skill
WHERE slug IN ('core-web-research', 'core-doc-analysis', 'core-task-workflow');

-- Remove capability tools
DELETE FROM ah_core.tool
WHERE slug IN (
    'core-web-search', 'core-web-fetch', 'core-doc-search',
    'core-doc-read', 'core-todo-create', 'core-todo-list', 'core-subagent-run'
);
