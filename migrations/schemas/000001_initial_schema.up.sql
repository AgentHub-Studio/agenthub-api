-- Extensions (idempotent)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;

-- Settings
CREATE TABLE IF NOT EXISTS settings (
    key         VARCHAR(255) PRIMARY KEY,
    value       JSONB        NOT NULL DEFAULT '{}',
    description TEXT,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Agent
CREATE TABLE IF NOT EXISTS agent (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    description     TEXT,
    status          VARCHAR(50)  NOT NULL DEFAULT 'DRAFT',
    current_version INTEGER,
    pipeline_id     UUID,
    config          JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_status   ON agent (status);
CREATE INDEX IF NOT EXISTS idx_agent_slug     ON agent (slug);

-- Pipeline
CREATE TABLE IF NOT EXISTS pipeline (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    agent_id    UUID         REFERENCES agent (id) ON DELETE SET NULL,
    status      VARCHAR(50)  NOT NULL DEFAULT 'DRAFT',
    config      JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_agent_id ON pipeline (agent_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_status   ON pipeline (status);

-- Pipeline Node
CREATE TABLE IF NOT EXISTS pipeline_node (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id UUID         NOT NULL REFERENCES pipeline (id) ON DELETE CASCADE,
    node_type   VARCHAR(50)  NOT NULL,
    name        VARCHAR(255) NOT NULL,
    config      JSONB        NOT NULL DEFAULT '{}',
    position_x  NUMERIC,
    position_y  NUMERIC,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_node_pipeline_id ON pipeline_node (pipeline_id);

-- Pipeline Edge
CREATE TABLE IF NOT EXISTS pipeline_edge (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id    UUID        NOT NULL REFERENCES pipeline (id) ON DELETE CASCADE,
    source_node_id UUID        NOT NULL REFERENCES pipeline_node (id) ON DELETE CASCADE,
    target_node_id UUID        NOT NULL REFERENCES pipeline_node (id) ON DELETE CASCADE,
    label          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_edge_pipeline_id ON pipeline_edge (pipeline_id);

-- Skill
CREATE TABLE IF NOT EXISTS skill (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    slug          VARCHAR(255) NOT NULL UNIQUE,
    description   TEXT,
    category      VARCHAR(100),
    input_schema  JSONB        NOT NULL DEFAULT '{}',
    output_schema JSONB        NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_skill_slug     ON skill (slug);
CREATE INDEX IF NOT EXISTS idx_skill_category ON skill (category);

-- Tool
CREATE TABLE IF NOT EXISTS tool (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    type        VARCHAR(50)  NOT NULL,
    description TEXT,
    config      JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tool_type ON tool (type);

-- Skill-Tool binding
CREATE TABLE IF NOT EXISTS skill_tool (
    id        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id  UUID        NOT NULL REFERENCES skill (id) ON DELETE CASCADE,
    tool_id   UUID        NOT NULL REFERENCES tool  (id) ON DELETE CASCADE,
    priority  INTEGER     NOT NULL DEFAULT 0,
    is_active BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (skill_id, tool_id)
);

CREATE INDEX IF NOT EXISTS idx_skill_tool_skill_id ON skill_tool (skill_id);

-- Knowledge Base
CREATE TABLE IF NOT EXISTS knowledge_base (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    status      VARCHAR(50)  NOT NULL DEFAULT 'ACTIVE',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Document
CREATE TABLE IF NOT EXISTS document (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id UUID         NOT NULL REFERENCES knowledge_base (id) ON DELETE CASCADE,
    file_name         VARCHAR(500) NOT NULL,
    content_type      VARCHAR(255) NOT NULL,
    status            VARCHAR(50)  NOT NULL DEFAULT 'PENDING',
    storage_path      TEXT         NOT NULL,
    file_size         BIGINT       NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_document_knowledge_base_id ON document (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_document_status            ON document (status);

-- Document chunks with vector embeddings
CREATE TABLE IF NOT EXISTS document_chunk (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID        NOT NULL REFERENCES document (id) ON DELETE CASCADE,
    content     TEXT        NOT NULL,
    chunk_index INTEGER     NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS document_chunk_embedding (
    chunk_id  UUID                    PRIMARY KEY REFERENCES document_chunk (id) ON DELETE CASCADE,
    embedding vector(1024)
);

-- MCP Server Config
CREATE TABLE IF NOT EXISTS mcp_server_config (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) NOT NULL UNIQUE,
    transport_type      VARCHAR(50)  NOT NULL DEFAULT 'http',
    http_base_url       TEXT,
    command             TEXT,
    args                JSONB        NOT NULL DEFAULT '[]',
    env                 JSONB        NOT NULL DEFAULT '{}',
    oauth_token_url     TEXT,
    oauth_client_id     TEXT,
    oauth_client_secret TEXT,
    oauth_scopes        JSONB        NOT NULL DEFAULT '[]',
    auto_start          BOOLEAN      NOT NULL DEFAULT FALSE,
    enabled             BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Chat Session
CREATE TABLE IF NOT EXISTS chat_session (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id   UUID         REFERENCES agent (id) ON DELETE SET NULL,
    title      VARCHAR(500) NOT NULL DEFAULT 'New Chat',
    status     VARCHAR(50)  NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_session_agent_id ON chat_session (agent_id);

-- Chat Message
CREATE TABLE IF NOT EXISTS chat_message (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID        NOT NULL REFERENCES chat_session (id) ON DELETE CASCADE,
    role       VARCHAR(50) NOT NULL,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_message_session_id ON chat_message (session_id);

-- Agent Memory
CREATE TABLE IF NOT EXISTS agent_memory (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id   UUID         NOT NULL REFERENCES agent (id) ON DELETE CASCADE,
    user_id    VARCHAR(255),
    key        VARCHAR(255) NOT NULL,
    value      JSONB        NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_id, user_id, key)
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_agent_id ON agent_memory (agent_id);

-- Agent Execution
CREATE TABLE IF NOT EXISTS agent_execution (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id      UUID        REFERENCES agent    (id) ON DELETE SET NULL,
    pipeline_id   UUID        REFERENCES pipeline (id) ON DELETE SET NULL,
    status        VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    input         JSONB       NOT NULL DEFAULT '{}',
    output        JSONB,
    error_message TEXT,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    duration_ms   BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_execution_agent_id  ON agent_execution (agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_execution_status    ON agent_execution (status);

-- Agent Execution Node
CREATE TABLE IF NOT EXISTS agent_execution_node (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id   UUID        NOT NULL REFERENCES agent_execution (id) ON DELETE CASCADE,
    node_id        UUID        REFERENCES pipeline_node (id) ON DELETE SET NULL,
    node_type      VARCHAR(50) NOT NULL,
    status         VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    input          JSONB       NOT NULL DEFAULT '{}',
    output         JSONB,
    error_message  TEXT,
    started_at     TIMESTAMPTZ,
    finished_at    TIMESTAMPTZ,
    duration_ms    BIGINT
);

CREATE INDEX IF NOT EXISTS idx_exec_node_execution_id ON agent_execution_node (execution_id);

-- Tool Execution
CREATE TABLE IF NOT EXISTS tool_execution (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    node_execution_id UUID        REFERENCES agent_execution_node (id) ON DELETE SET NULL,
    tool_id           UUID        REFERENCES tool (id) ON DELETE SET NULL,
    status            VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    input             JSONB       NOT NULL DEFAULT '{}',
    output            JSONB,
    error_message     TEXT,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at       TIMESTAMPTZ,
    duration_ms       BIGINT
);

CREATE INDEX IF NOT EXISTS idx_tool_execution_node_id ON tool_execution (node_execution_id);

-- Webhook Config
CREATE TABLE IF NOT EXISTS webhook_config (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    url         TEXT         NOT NULL,
    events      TEXT[]       NOT NULL DEFAULT '{}',
    secret      TEXT,
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    retry_count INTEGER      NOT NULL DEFAULT 3,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Webhook Delivery Log
CREATE TABLE IF NOT EXISTS webhook_delivery_log (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id      UUID        REFERENCES webhook_config (id) ON DELETE CASCADE,
    event_type      VARCHAR(100) NOT NULL,
    payload         JSONB        NOT NULL DEFAULT '{}',
    status          VARCHAR(50)  NOT NULL DEFAULT 'PENDING',
    response_status INTEGER,
    response_body   TEXT,
    attempts        INTEGER      NOT NULL DEFAULT 0,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_delivery_webhook_id ON webhook_delivery_log (webhook_id);

-- OAuth Credential
CREATE TABLE IF NOT EXISTS oauth_credential (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) NOT NULL,
    auth_type        VARCHAR(50)  NOT NULL,
    token_url        TEXT,
    client_id        TEXT,
    client_secret    TEXT,
    scopes           TEXT,
    api_key_header   TEXT,
    api_key_value    TEXT,
    bearer_token     TEXT,
    username         TEXT,
    password         TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Audit Log
CREATE TABLE IF NOT EXISTS audit_log (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type  VARCHAR(100) NOT NULL,
    entity_id    VARCHAR(255) NOT NULL,
    action       VARCHAR(100) NOT NULL,
    actor_id     VARCHAR(255),
    actor_email  TEXT,
    old_value    TEXT,
    new_value    TEXT,
    metadata     JSONB,
    ip_address   VARCHAR(50),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_entity     ON audit_log (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor      ON audit_log (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at DESC);

-- Agent Metrics
CREATE TABLE IF NOT EXISTS agent_metrics (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id            UUID         REFERENCES agent          (id) ON DELETE SET NULL,
    agent_execution_id  UUID         REFERENCES agent_execution(id) ON DELETE SET NULL,
    session_id          UUID         REFERENCES chat_session   (id) ON DELETE SET NULL,
    model_name          VARCHAR(255) NOT NULL,
    provider            VARCHAR(100) NOT NULL,
    prompt_tokens       INTEGER      NOT NULL DEFAULT 0,
    completion_tokens   INTEGER      NOT NULL DEFAULT 0,
    total_tokens        INTEGER      NOT NULL DEFAULT 0,
    estimated_cost_usd  NUMERIC(10,6) NOT NULL DEFAULT 0,
    latency_ms          INTEGER      NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_metrics_agent_id ON agent_metrics (agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_metrics_created  ON agent_metrics (created_at DESC);

-- Prompt Experiment
CREATE TABLE IF NOT EXISTS prompt_experiment (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id       UUID         REFERENCES agent (id) ON DELETE SET NULL,
    name           VARCHAR(255) NOT NULL,
    status         VARCHAR(50)  NOT NULL DEFAULT 'DRAFT',
    traffic_split  JSONB,
    variants       JSONB        NOT NULL DEFAULT '[]',
    start_date     TIMESTAMPTZ,
    end_date       TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prompt_experiment_agent_id ON prompt_experiment (agent_id);
CREATE INDEX IF NOT EXISTS idx_prompt_experiment_status   ON prompt_experiment (status);

-- Experiment Result
CREATE TABLE IF NOT EXISTS experiment_result (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    experiment_id   UUID         NOT NULL REFERENCES prompt_experiment (id) ON DELETE CASCADE,
    variant_key     VARCHAR(100) NOT NULL,
    session_id      UUID         REFERENCES chat_session (id) ON DELETE SET NULL,
    user_feedback   TEXT,
    latency_ms      INTEGER      NOT NULL DEFAULT 0,
    token_count     INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_experiment_result_experiment_id ON experiment_result (experiment_id);

-- VPN Resource
CREATE TABLE IF NOT EXISTS vpn_resource (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    enabled          BOOLEAN      NOT NULL DEFAULT TRUE,
    ovpn_config_path TEXT         NOT NULL DEFAULT '',
    auth_file_path   TEXT         NOT NULL DEFAULT '',
    secret_name      TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Data Source
CREATE TABLE IF NOT EXISTS data_source (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50)  NOT NULL,
    host            TEXT         NOT NULL,
    port            INTEGER      NOT NULL,
    database        VARCHAR(255) NOT NULL,
    db_user         VARCHAR(255) NOT NULL,
    db_password     TEXT         NOT NULL DEFAULT '',
    vpn_resource_id UUID         REFERENCES vpn_resource (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_data_source_vpn_resource_id ON data_source (vpn_resource_id);
