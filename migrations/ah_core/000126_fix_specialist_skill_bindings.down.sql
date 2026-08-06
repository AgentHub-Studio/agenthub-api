SET search_path TO ah_core, public;

-- Roll back only the bindings introduced by migration 000126. The
-- core-pipeline-specialist intentionally remains unbound because its legacy
-- pipeline inspection surface is read-only.
WITH bindings(agent_slug, skill_slug) AS (
    VALUES
        ('core-agent-builder',        'core-agents-management'),
        ('core-agent-builder',        'core-skills-management'),
        ('core-agent-builder',        'core-platform-settings'),
        ('core-tool-builder',         'core-tools-management'),
        ('core-tool-builder',         'core-skills-management'),
        ('core-skills-specialist',    'core-skills-management'),
        ('core-skills-specialist',    'core-tools-management'),
        ('core-tools-specialist',     'core-tools-management'),
        ('core-kb-builder',           'core-kb-management'),
        ('core-kb-builder',           'core-agents-management'),
        ('core-kb-specialist',        'core-kb-management'),
        ('core-mcp-configurator',     'core-mcp-management'),
        ('core-api-importer',         'core-tools-management'),
        ('core-api-importer',         'core-skills-management'),
        ('core-api-importer',         'core-agents-management'),
        ('core-agents-specialist',    'core-agents-management'),
        ('core-execution-specialist', 'core-execution-management')
)
DELETE FROM agent_skill AS agent_skill
 USING bindings
  JOIN agent ON agent.slug = bindings.agent_slug
  JOIN skill ON skill.slug = bindings.skill_slug
 WHERE agent_skill.agent_id = agent.id
   AND agent_skill.skill_id = skill.id;
