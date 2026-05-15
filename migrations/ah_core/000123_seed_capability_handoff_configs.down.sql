-- Rollback for migration 000123 (capability handoff configs).
-- Removes the 9 seeded handoff config rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_handoff_config table.

DELETE FROM ah_core.capability_handoff_config
 WHERE (agent_slug, handoff_key) IN (
     ('core-researcher', 'needs_analysis'),
     ('core-researcher', 'needs_planning'),
     ('core-researcher', 'human_escalation'),
     ('core-analyst',    'needs_research'),
     ('core-analyst',    'needs_planning'),
     ('core-analyst',    'human_escalation'),
     ('core-planner',    'needs_research'),
     ('core-planner',    'needs_analysis'),
     ('core-planner',    'human_escalation')
 );

DROP TABLE IF EXISTS ah_core.capability_handoff_config;
