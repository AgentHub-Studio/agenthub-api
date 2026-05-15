-- Rollback for migration 000122 (capability interaction modes).
-- Removes the 9 seeded interaction mode rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_interaction_mode table.

DELETE FROM ah_core.capability_interaction_mode
 WHERE (agent_slug, mode_key) IN (
     ('core-researcher', 'primary_mode'),
     ('core-researcher', 'proactive_questions'),
     ('core-researcher', 'response_style'),
     ('core-analyst',    'primary_mode'),
     ('core-analyst',    'proactive_questions'),
     ('core-analyst',    'response_style'),
     ('core-planner',    'primary_mode'),
     ('core-planner',    'proactive_questions'),
     ('core-planner',    'response_style')
 );

DROP TABLE IF EXISTS ah_core.capability_interaction_mode;
