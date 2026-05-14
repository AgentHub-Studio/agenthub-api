-- Down migration for 000110: remove capability agent prerequisite rows
-- and drop the capability_agent_prerequisite table.
--
-- Removes the 6 rows seeded by 000110.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_agent_prerequisite
WHERE agent_slug IN (
    'core-researcher',
    'core-analyst',
    'core-planner'
);

DROP TABLE IF EXISTS ah_core.capability_agent_prerequisite;
