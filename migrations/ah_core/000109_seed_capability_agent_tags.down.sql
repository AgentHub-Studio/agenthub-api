-- Down migration for 000109: remove capability agent tag rows
-- and drop the capability_agent_tag table.
--
-- Removes the 12 rows seeded by 000109.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_agent_tag
WHERE agent_slug IN (
    'core-researcher',
    'core-analyst',
    'core-planner'
);

DROP TABLE IF EXISTS ah_core.capability_agent_tag;
