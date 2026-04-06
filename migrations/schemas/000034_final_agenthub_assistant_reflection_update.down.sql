-- migration: 000034_final_agenthub_assistant_reflection_update
-- down

SET search_path TO ah_test;

-- No easy way to revert system_prompt exactly without previous content, 
-- but we can set it back to something simpler.
UPDATE agent
SET system_prompt = 'You are the AgentHub Assistant.'
WHERE slug = 'agenthub-assistant';
