-- DSR-04: normalize chat session modes and enforce mode/agent consistency.
-- Depends on 000084_dynamic_skill_retrieval.

UPDATE chat_session
SET mode = CASE
    WHEN agent_id IS NULL THEN 'DYNAMIC_SKILL'
    ELSE 'AGENT_FIXED'
END
WHERE mode IS NULL
   OR mode = ''
   OR mode = 'AGENT';

ALTER TABLE chat_session
    ALTER COLUMN mode SET DEFAULT 'AGENT_FIXED';

ALTER TABLE chat_session
    DROP CONSTRAINT IF EXISTS chk_chat_session_mode_agent;

ALTER TABLE chat_session
    ADD CONSTRAINT chk_chat_session_mode_agent
    CHECK (
        (mode = 'AGENT_FIXED' AND agent_id IS NOT NULL)
        OR
        (mode = 'DYNAMIC_SKILL' AND agent_id IS NULL)
    );
