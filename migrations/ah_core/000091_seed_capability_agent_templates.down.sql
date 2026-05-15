-- Remove capability agent-skill bindings seeded in 000091.
DELETE FROM ah_core.agent_skill
 WHERE agent_id IN (
     SELECT id FROM ah_core.agent
      WHERE slug IN ('core-researcher', 'core-analyst', 'core-planner')
 );

-- Remove capability agents seeded in 000091.
DELETE FROM ah_core.agent
 WHERE slug IN ('core-researcher', 'core-analyst', 'core-planner');
