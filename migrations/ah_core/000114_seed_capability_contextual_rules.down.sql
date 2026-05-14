-- Rollback for migration 000114 (capability contextual rules).
-- Removes the 9 seeded contextual rule rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_contextual_rule table.

DELETE FROM ah_core.capability_contextual_rule
 WHERE agent_slug IN ('core-researcher', 'core-analyst', 'core-planner');

DROP TABLE IF EXISTS ah_core.capability_contextual_rule;
