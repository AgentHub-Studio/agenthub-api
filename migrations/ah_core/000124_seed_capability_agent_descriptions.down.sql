-- Rollback for migration 000124 (capability agent descriptions).
-- Removes the 9 seeded description rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_agent_description table.

DELETE FROM ah_core.capability_agent_description
 WHERE (agent_slug, desc_key) IN (
     ('core-researcher', 'tagline'),
     ('core-researcher', 'long_description'),
     ('core-researcher', 'use_cases'),
     ('core-analyst',    'tagline'),
     ('core-analyst',    'long_description'),
     ('core-analyst',    'use_cases'),
     ('core-planner',    'tagline'),
     ('core-planner',    'long_description'),
     ('core-planner',    'use_cases')
 );

DROP TABLE IF EXISTS ah_core.capability_agent_description;
