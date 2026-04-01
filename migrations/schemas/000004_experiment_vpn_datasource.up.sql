-- Prompt A/B experiments for agent variants (per-tenant, no tenant_id)
CREATE TABLE IF NOT EXISTS prompt_experiment (
    id            UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id      UUID             NOT NULL,
    name          VARCHAR(255)     NOT NULL,
    status        VARCHAR(50)      NOT NULL DEFAULT 'DRAFT',
    traffic_split TEXT             NOT NULL DEFAULT '{}',
    variants      TEXT             NOT NULL DEFAULT '[]',
    start_date    TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    end_date      TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

-- Observations recorded for each experiment variant
CREATE TABLE IF NOT EXISTS experiment_result (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    experiment_id UUID         NOT NULL REFERENCES prompt_experiment(id) ON DELETE CASCADE,
    variant_key   VARCHAR(100) NOT NULL,
    session_id    VARCHAR(255) NOT NULL DEFAULT '',
    user_feedback INT          NOT NULL DEFAULT 0,
    latency_ms    BIGINT       NOT NULL DEFAULT 0,
    token_count   INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_experiment_result_experiment ON experiment_result (experiment_id);

-- VPN tunnel configurations (per-tenant, no tenant_id)
CREATE TABLE IF NOT EXISTS vpn_resource (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) NOT NULL,
    description      TEXT         NOT NULL DEFAULT '',
    enabled          BOOLEAN      NOT NULL DEFAULT TRUE,
    ovpn_config_path TEXT         NOT NULL DEFAULT '',
    auth_file_path   TEXT         NOT NULL DEFAULT '',
    secret_name      VARCHAR(255) NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Database connection configurations (per-tenant, no tenant_id)
CREATE TABLE IF NOT EXISTS data_source (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50)  NOT NULL,
    host            VARCHAR(255) NOT NULL,
    port            INT          NOT NULL,
    database        VARCHAR(255) NOT NULL,
    db_user         VARCHAR(255) NOT NULL,
    db_password     TEXT         NOT NULL DEFAULT '',
    vpn_resource_id UUID         REFERENCES vpn_resource(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
