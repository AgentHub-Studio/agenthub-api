-- Down migration for 000108: remove capability example prompt rows
-- and drop the capability_example_prompt table.
--
-- Removes the 12 rows seeded by 000108.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_example_prompt
WHERE agent_slug IN (
    'core-researcher',
    'core-analyst',
    'core-planner'
);

DROP TABLE IF EXISTS ah_core.capability_example_prompt;
