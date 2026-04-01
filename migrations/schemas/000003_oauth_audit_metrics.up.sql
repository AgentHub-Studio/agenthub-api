-- OAuth credentials for outbound HTTP authentication (per-tenant, no tenant_id)
CREATE TABLE IF NOT EXISTS oauth_credential (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    auth_type     VARCHAR(50)  NOT NULL,
    token_url     TEXT         NOT NULL DEFAULT '',
    client_id     TEXT         NOT NULL DEFAULT '',
    client_secret TEXT         NOT NULL DEFAULT '',
    scopes        TEXT         NOT NULL DEFAULT '',
    api_key_header TEXT        NOT NULL DEFAULT '',
    api_key_value  TEXT        NOT NULL DEFAULT '',
    bearer_token   TEXT        NOT NULL DEFAULT '',
    username       TEXT        NOT NULL DEFAULT '',
    password       TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Append-only audit log for tenant operations (per-tenant, no tenant_id)
CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(100) NOT NULL,
    entity_id   VARCHAR(255) NOT NULL DEFAULT '',
    action      VARCHAR(50)  NOT NULL,
    actor_id    VARCHAR(255) NOT NULL DEFAULT '',
    actor_email VARCHAR(255) NOT NULL DEFAULT '',
    old_value   TEXT         NOT NULL DEFAULT '',
    new_value   TEXT         NOT NULL DEFAULT '',
    metadata    TEXT         NOT NULL DEFAULT '',
    ip_address  VARCHAR(45)  NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON audit_log (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor  ON audit_log (actor_id);

-- Token and latency metrics per agent execution (per-tenant, no tenant_id)
CREATE TABLE IF NOT EXISTS agent_metrics (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id            UUID         NOT NULL,
    agent_execution_id  UUID         NOT NULL,
    session_id          VARCHAR(255) NOT NULL DEFAULT '',
    model_name          VARCHAR(100) NOT NULL DEFAULT '',
    provider            VARCHAR(100) NOT NULL DEFAULT '',
    prompt_tokens       INT          NOT NULL DEFAULT 0,
    completion_tokens   INT          NOT NULL DEFAULT 0,
    total_tokens        INT          NOT NULL DEFAULT 0,
    estimated_cost_usd  NUMERIC(12,6) NOT NULL DEFAULT 0,
    latency_ms          BIGINT       NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_metrics_agent_id ON agent_metrics (agent_id);
