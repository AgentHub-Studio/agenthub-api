-- Down migration for 000104: remove capability agent persona rows
-- and drop the capability_agent_persona table.
--
-- Removes the 3 rows seeded by 000104.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_agent_persona
WHERE slug IN (
    'capability-researcher-persona',
    'capability-analyst-persona',
    'capability-planner-persona'
);

DROP TABLE IF EXISTS ah_core.capability_agent_persona;
