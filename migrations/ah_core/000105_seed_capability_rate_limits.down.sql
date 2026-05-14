-- Down migration for 000105: remove capability rate limit rows
-- and drop the capability_rate_limit table.
--
-- Removes the 9 rows seeded by 000105.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_rate_limit
WHERE agent_slug IN (
    'core-researcher',
    'core-analyst',
    'core-planner'
);

DROP TABLE IF EXISTS ah_core.capability_rate_limit;
