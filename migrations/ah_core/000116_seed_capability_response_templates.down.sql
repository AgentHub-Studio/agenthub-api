-- Rollback for migration 000116 (capability response templates).
-- Removes the 9 seeded response template rows for the three capability agents
-- (core-researcher, core-analyst, core-planner) and drops the
-- ah_core.capability_response_template table.

DELETE FROM ah_core.capability_response_template
 WHERE (agent_slug, template_key) IN (
     ('core-researcher', 'greeting'),
     ('core-researcher', 'not_found'),
     ('core-researcher', 'source_disclaimer'),
     ('core-analyst',    'greeting'),
     ('core-analyst',    'ambiguity_prompt'),
     ('core-analyst',    'confidence_footer'),
     ('core-planner',    'greeting'),
     ('core-planner',    'clarification_request'),
     ('core-planner',    'handoff')
 );

DROP TABLE IF EXISTS ah_core.capability_response_template;
