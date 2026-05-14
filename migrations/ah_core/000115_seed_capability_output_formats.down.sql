-- Rollback for migration 000115 (capability output formats).
-- Removes the 9 seeded output format rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_output_format table.

DELETE FROM ah_core.capability_output_format
 WHERE agent_slug IN ('core-researcher', 'core-analyst', 'core-planner');

DROP TABLE IF EXISTS ah_core.capability_output_format;
