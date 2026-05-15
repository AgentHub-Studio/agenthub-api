-- Rollback for migration 000121 (capability fallback behaviors).
-- Removes the 9 seeded fallback behavior rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_fallback_behavior table.

DELETE FROM ah_core.capability_fallback_behavior
 WHERE (agent_slug, behavior_key) IN (
     ('core-researcher', 'search_failure'),
     ('core-researcher', 'fetch_failure'),
     ('core-researcher', 'no_results'),
     ('core-analyst',    'doc_unavailable'),
     ('core-analyst',    'analysis_timeout'),
     ('core-analyst',    'ambiguous_data'),
     ('core-planner',    'subagent_failure'),
     ('core-planner',    'scope_too_large'),
     ('core-planner',    'no_goal')
 );

DROP TABLE IF EXISTS ah_core.capability_fallback_behavior;
