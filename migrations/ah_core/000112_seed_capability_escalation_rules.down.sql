-- Rollback for migration 000112 (capability escalation rules).
-- Removes the 9 seeded escalation rule rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_escalation_rule table.

DELETE FROM ah_core.capability_escalation_rule
 WHERE agent_slug IN ('core-researcher', 'core-analyst', 'core-planner');

DROP TABLE IF EXISTS ah_core.capability_escalation_rule;
