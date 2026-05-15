-- Seed capability agent–webhook bindings in ah_core
-- (migration 000099). These 3 bindings link each capability agent
-- (introduced in migration 000091) to its corresponding webhook notification
-- template (introduced in migration 000097):
--
--   core-researcher  →  capability-research-complete  (sort_order 1)
--   core-analyst     →  capability-analysis-done      (sort_order 2)
--   core-planner     →  capability-tasks-updated      (sort_order 3)
--
-- Each binding declares which webhook event a capability agent emits upon
-- completing its primary task. The platform uses these bindings at runtime to:
--
--   1. Resolve outbound webhook notification targets for a given agent slug.
--   2. Validate that a registered webhook template matches the agent's event
--      contract before wiring up a live delivery endpoint.
--   3. Drive the capability-layer documentation (one binding per agent keeps
--      the event surface discoverable and auditable).
--
-- Design notes:
--   - PRIMARY KEY (agent_slug, webhook_slug) — each (agent, webhook) pair is
--     unique; an agent may be bound to multiple webhooks in future iterations.
--   - is_active — allows disabling a binding without dropping the row.
--   - sort_order — controls display ordering in the UI event catalogue.
--   - ON CONFLICT DO NOTHING — migration is safe to re-apply (idempotent).
--
-- The capability_agent_webhook_binding table does not exist before this migration;
-- it is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_webhook_binding (
    agent_slug   VARCHAR(120) NOT NULL,
    webhook_slug VARCHAR(120) NOT NULL,
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order   INTEGER      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_slug, webhook_slug)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_agent_webhook_binding_agent
    ON ah_core.capability_agent_webhook_binding (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_agent_webhook_binding_webhook
    ON ah_core.capability_agent_webhook_binding (webhook_slug);

-- ============================
-- 3 capability agent–webhook bindings (one per capability agent)
-- ============================
INSERT INTO ah_core.capability_agent_webhook_binding
    (agent_slug, webhook_slug, is_active, sort_order)
VALUES
    -- 1. Researcher → research-complete
    ('core-researcher', 'capability-research-complete', TRUE, 1),
    -- 2. Analyst    → analysis-done
    ('core-analyst',    'capability-analysis-done',     TRUE, 2),
    -- 3. Planner    → tasks-updated
    ('core-planner',    'capability-tasks-updated',     TRUE, 3)

ON CONFLICT (agent_slug, webhook_slug) DO NOTHING;
