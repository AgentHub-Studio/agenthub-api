-- Revert to the original constraint (without NULLS NOT DISTINCT).
ALTER TABLE agent_memory
    DROP CONSTRAINT IF EXISTS agent_memory_agent_id_user_id_key_key;

ALTER TABLE agent_memory
    ADD CONSTRAINT agent_memory_agent_id_user_id_key_key
    UNIQUE (agent_id, user_id, key);
