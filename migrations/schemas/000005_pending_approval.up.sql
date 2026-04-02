-- Pending approvals for APPROVAL pipeline nodes (per-tenant, no tenant_id)
CREATE TABLE IF NOT EXISTS pending_approval (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id     UUID         NOT NULL,
    node_id          TEXT         NOT NULL,
    title            TEXT         NOT NULL,
    description      TEXT         NOT NULL DEFAULT '',
    details          TEXT         NOT NULL DEFAULT '',
    status           VARCHAR(50)  NOT NULL DEFAULT 'PENDING',
    responded_by     TEXT,
    responded_at     TIMESTAMPTZ,
    comment          TEXT,
    timeout_at       TIMESTAMPTZ,
    callback_url     TEXT,
    notify_channels  JSONB        NOT NULL DEFAULT '[]',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_approval_execution ON pending_approval (execution_id);
CREATE INDEX IF NOT EXISTS idx_pending_approval_status    ON pending_approval (status);
