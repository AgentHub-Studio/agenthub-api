-- Agent A/B Test: routes a fraction of sessions to an alternate agent version.
-- Stored in ah_{tenantID}.agent_ab_test — no tenant_id column.

-- Ensure agent_version exists (was missing from earlier migrations).
CREATE TABLE IF NOT EXISTS agent_version (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        UUID        NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    version_number  INT         NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    description     TEXT        NOT NULL DEFAULT '',
    definition_json JSONB,
    config_json     JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at    TIMESTAMPTZ,
    UNIQUE (agent_id, version_number)
);
CREATE INDEX IF NOT EXISTS idx_agent_version_agent  ON agent_version(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_version_status ON agent_version(status);

CREATE TABLE IF NOT EXISTS agent_ab_test (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id      UUID        NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    name          VARCHAR(255) NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    -- control_version_id: the current production version (nil = live agent).
    control_version_id   UUID REFERENCES agent_version(id),
    -- variant_version_id: the challenger version to route traffic to.
    variant_version_id   UUID NOT NULL REFERENCES agent_version(id),
    -- traffic_percent: 0–100, percentage of sessions routed to the variant.
    traffic_percent      INT  NOT NULL DEFAULT 10 CHECK (traffic_percent BETWEEN 0 AND 100),
    -- status: ACTIVE, PAUSED, CONCLUDED
    status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at      TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_ab_test_name UNIQUE (agent_id, name)
);

-- Records each session's variant assignment for later analysis.
CREATE TABLE IF NOT EXISTS agent_ab_assignment (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id    UUID        NOT NULL REFERENCES agent_ab_test(id) ON DELETE CASCADE,
    session_id UUID        NOT NULL,
    -- variant: 'control' or 'variant'
    variant    VARCHAR(10) NOT NULL CHECK (variant IN ('control', 'variant')),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ab_test_agent_id  ON agent_ab_test(agent_id);
CREATE INDEX IF NOT EXISTS idx_ab_test_status    ON agent_ab_test(status);
CREATE INDEX IF NOT EXISTS idx_ab_assignment_test ON agent_ab_assignment(test_id);
CREATE INDEX IF NOT EXISTS idx_ab_assignment_session ON agent_ab_assignment(session_id);
