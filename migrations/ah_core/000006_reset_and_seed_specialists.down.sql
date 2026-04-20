-- Revert specialist agents seed: drop the 12 canonical specialists.
-- Bindings are removed by FK ON DELETE CASCADE (agent_skill.agent_id).
DELETE FROM ah_core.agent
 WHERE slug IN (
  'core-assistant',
  'core-agent-builder',
  'core-tool-builder',
  'core-skills-specialist',
  'core-tools-specialist',
  'core-kb-builder',
  'core-kb-specialist',
  'core-mcp-configurator',
  'core-api-importer',
  'core-agents-specialist',
  'core-pipeline-specialist',
  'core-execution-specialist'
 );
