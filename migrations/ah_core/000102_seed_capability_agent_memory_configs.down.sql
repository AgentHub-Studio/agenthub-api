-- Down migration for 000102: remove capability agent memory config rows
-- and drop the capability_agent_memory_config table.
--
-- Removes the 9 rows seeded by 000102.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_agent_memory_config
WHERE agent_slug IN (
    'core-researcher',
    'core-analyst',
    'core-planner'
);

DROP TABLE IF EXISTS ah_core.capability_agent_memory_config;
