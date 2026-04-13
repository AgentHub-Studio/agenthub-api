-- Fix agent_memory unique constraint to treat NULL user_id as equal.
-- In PostgreSQL, NULLs are not considered equal in unique constraints by default,
-- which causes multiple rows to accumulate when user_id IS NULL and the same key
-- is upserted multiple times (ON CONFLICT does not fire).
-- NULLS NOT DISTINCT (PG 15+) fixes this so (agent_id, NULL, key) is unique.

ALTER TABLE agent_memory
    DROP CONSTRAINT IF EXISTS agent_memory_agent_id_user_id_key_key;

ALTER TABLE agent_memory
    ADD CONSTRAINT agent_memory_agent_id_user_id_key_key
    UNIQUE NULLS NOT DISTINCT (agent_id, user_id, key);
