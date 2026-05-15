-- Rollback for migration 000118 (capability tool permissions).
-- Removes the 12 seeded tool permission rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_tool_permission table.

DELETE FROM ah_core.capability_tool_permission
 WHERE (agent_slug, tool_slug) IN (
     ('core-researcher', 'core-web-search'),
     ('core-researcher', 'core-web-fetch'),
     ('core-researcher', 'core-doc-search'),
     ('core-researcher', 'core-subagent-run'),
     ('core-analyst',    'core-doc-search'),
     ('core-analyst',    'core-web-search'),
     ('core-analyst',    'core-web-fetch'),
     ('core-analyst',    'core-subagent-run'),
     ('core-planner',    'core-subagent-run'),
     ('core-planner',    'core-doc-search'),
     ('core-planner',    'core-web-search'),
     ('core-planner',    'core-web-fetch')
 );

DROP TABLE IF EXISTS ah_core.capability_tool_permission;
