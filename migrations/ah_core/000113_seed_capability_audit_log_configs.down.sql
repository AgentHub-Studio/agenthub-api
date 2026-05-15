-- Rollback for migration 000113 (capability audit log configs).
-- Removes the 9 seeded audit log config rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_audit_log_config table.

DELETE FROM ah_core.capability_audit_log_config
 WHERE agent_slug IN ('core-researcher', 'core-analyst', 'core-planner');

DROP TABLE IF EXISTS ah_core.capability_audit_log_config;
