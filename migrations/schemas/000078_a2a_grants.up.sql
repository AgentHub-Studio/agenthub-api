CREATE TABLE IF NOT EXISTS a2a_grant (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_tenant TEXT        NOT NULL,
    agent_id       UUID        NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    actions        TEXT[]      NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (subject_tenant, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_a2a_grant_subject_tenant ON a2a_grant(subject_tenant);
CREATE INDEX IF NOT EXISTS idx_a2a_grant_agent_id ON a2a_grant(agent_id);
