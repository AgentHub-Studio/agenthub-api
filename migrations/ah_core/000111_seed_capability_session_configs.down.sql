-- Down migration for 000111: remove capability session config rows
-- and drop the capability_session_config table.
--
-- Removes the 9 rows seeded by 000111.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_session_config
WHERE agent_slug IN (
    'core-researcher',
    'core-analyst',
    'core-planner'
);

DROP TABLE IF EXISTS ah_core.capability_session_config;
