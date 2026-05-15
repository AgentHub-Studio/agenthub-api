-- Down migration for 000099: remove capability agent–webhook bindings
-- and drop the capability_agent_webhook_binding table.
--
-- Removes the 3 bindings seeded by 000099.up.sql and drops the table
-- itself (which is created by the up migration).

DELETE FROM ah_core.capability_agent_webhook_binding
WHERE agent_slug IN ('core-researcher', 'core-analyst', 'core-planner');

DROP TABLE IF EXISTS ah_core.capability_agent_webhook_binding;
