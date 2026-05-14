-- Rollback for migration 000119 (capability agent capabilities).
-- Removes the 12 seeded capability rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_agent_capability table.

DELETE FROM ah_core.capability_agent_capability
 WHERE (agent_slug, capability_key) IN (
     ('core-researcher', 'web_search'),
     ('core-researcher', 'document_analysis'),
     ('core-researcher', 'code_generation'),
     ('core-researcher', 'task_planning'),
     ('core-analyst',    'data_analysis'),
     ('core-analyst',    'document_analysis'),
     ('core-analyst',    'code_generation'),
     ('core-analyst',    'web_search'),
     ('core-planner',    'task_planning'),
     ('core-planner',    'subagent_delegation'),
     ('core-planner',    'code_generation'),
     ('core-planner',    'web_search')
 );

DROP TABLE IF EXISTS ah_core.capability_agent_capability;
