-- Down migration for 000100: remove capability system prompt templates
-- and drop the system_prompt_template table.
--
-- Removes the 3 templates seeded by 000100.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.system_prompt_template
WHERE slug IN (
    'capability-researcher-system-prompt',
    'capability-analyst-system-prompt',
    'capability-planner-system-prompt'
);

DROP TABLE IF EXISTS ah_core.system_prompt_template;
