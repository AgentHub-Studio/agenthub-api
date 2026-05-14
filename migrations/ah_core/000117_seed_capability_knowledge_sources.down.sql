-- Rollback for migration 000117 (capability knowledge sources).
-- Removes the 9 seeded knowledge source rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_knowledge_source table.

DELETE FROM ah_core.capability_knowledge_source
 WHERE (agent_slug, source_key) IN (
     ('core-researcher', 'primary'),
     ('core-researcher', 'secondary'),
     ('core-researcher', 'fallback'),
     ('core-analyst',    'primary'),
     ('core-analyst',    'secondary'),
     ('core-analyst',    'fallback'),
     ('core-planner',    'primary'),
     ('core-planner',    'secondary'),
     ('core-planner',    'fallback')
 );

DROP TABLE IF EXISTS ah_core.capability_knowledge_source;
