-- The router selects one active experiment for an agent. Preserve that
-- invariant in storage so concurrent create/reactivate requests cannot leave
-- the session path dependent on an unordered LIMIT 1.
--
-- For legacy rows, retain the latest started experiment and pause older active
-- rows. PAUSED is non-routing and preserves the record for administrator
-- review instead of deleting experiment history.
WITH ranked_active_tests AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY agent_id
            ORDER BY started_at DESC, created_at DESC, id DESC
        ) AS active_rank
    FROM agent_ab_test
    WHERE status = 'ACTIVE'
)
UPDATE agent_ab_test AS test
SET status = 'PAUSED', updated_at = NOW()
FROM ranked_active_tests AS ranked
WHERE test.id = ranked.id
  AND ranked.active_rank > 1;

CREATE UNIQUE INDEX uq_agent_ab_test_active_per_agent
    ON agent_ab_test (agent_id)
    WHERE status = 'ACTIVE';
