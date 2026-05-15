-- Rollback for migration 000120 (capability model configs).
-- Removes the 9 seeded model config rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_model_config table.

DELETE FROM ah_core.capability_model_config
 WHERE (agent_slug, config_key) IN (
     ('core-researcher', 'default_model'),
     ('core-researcher', 'temperature'),
     ('core-researcher', 'max_tokens'),
     ('core-analyst',    'default_model'),
     ('core-analyst',    'temperature'),
     ('core-analyst',    'max_tokens'),
     ('core-planner',    'default_model'),
     ('core-planner',    'temperature'),
     ('core-planner',    'max_tokens')
 );

DROP TABLE IF EXISTS ah_core.capability_model_config;
