SET search_path TO ah_core, public;

-- Repair the specialist-to-skill catalog created by migration 000006.
--
-- Migration 000006 used pre-namespace skill slugs (for example
-- "agent-management") while the canonical skill seed uses "core-" prefixed
-- slugs. As a result, only core-assistant received bindings. This additive
-- migration repairs already-installed catalogs without rewriting migration
-- history and remains idempotent for new installations.
WITH bindings(agent_slug, skill_slug, priority) AS (
    VALUES
        ('core-agent-builder',        'core-agents-management',     0),
        ('core-agent-builder',        'core-skills-management',     1),
        ('core-agent-builder',        'core-platform-settings',     2),
        ('core-tool-builder',         'core-tools-management',      0),
        ('core-tool-builder',         'core-skills-management',     1),
        ('core-skills-specialist',    'core-skills-management',     0),
        ('core-skills-specialist',    'core-tools-management',      1),
        ('core-tools-specialist',     'core-tools-management',      0),
        ('core-kb-builder',           'core-kb-management',         0),
        ('core-kb-builder',           'core-agents-management',     1),
        ('core-kb-specialist',        'core-kb-management',         0),
        ('core-mcp-configurator',     'core-mcp-management',        0),
        ('core-api-importer',         'core-tools-management',      0),
        ('core-api-importer',         'core-skills-management',     1),
        ('core-api-importer',         'core-agents-management',     2),
        ('core-agents-specialist',    'core-agents-management',     0),
        ('core-execution-specialist', 'core-execution-management',  0)
)
INSERT INTO agent_skill (agent_id, skill_id, priority)
SELECT agent.id, skill.id, bindings.priority
  FROM bindings
  JOIN agent ON agent.slug = bindings.agent_slug
  JOIN skill ON skill.slug = bindings.skill_slug
ON CONFLICT (agent_id, skill_id) DO UPDATE
    SET priority = EXCLUDED.priority;
