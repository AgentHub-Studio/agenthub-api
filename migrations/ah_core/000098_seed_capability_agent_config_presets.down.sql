-- Rollback migration 000098: remove the 3 capability agent config presets.
-- The agent_config_preset table itself is NOT dropped — there may be other presets
-- seeded by future migrations. Only the 3 capability rows seeded here are removed.

DELETE FROM ah_core.agent_config_preset
 WHERE slug IN (
     'capability-research-config',
     'capability-analysis-config',
     'capability-planning-config'
 );
