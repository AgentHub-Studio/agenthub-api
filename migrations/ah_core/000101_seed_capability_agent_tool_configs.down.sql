-- Down migration for 000101: remove capability agent tool config overrides
-- and drop the capability_agent_tool_config table.
--
-- Removes the 7 rows seeded by 000101.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_agent_tool_config
WHERE agent_slug IN (
    'core-researcher',
    'core-analyst',
    'core-planner'
);

DROP TABLE IF EXISTS ah_core.capability_agent_tool_config;
