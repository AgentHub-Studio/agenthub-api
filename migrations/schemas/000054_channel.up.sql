-- Channel adapters allow external messaging platforms (Slack, Telegram, Discord,
-- custom HTTP webhooks) to send messages to an agent and receive replies.
-- Each channel is bound to one agent and has a platform-specific config JSON.
CREATE TABLE IF NOT EXISTS channel (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    type        VARCHAR(50)  NOT NULL,     -- SLACK | TELEGRAM | DISCORD | CUSTOM
    agent_id    UUID         NOT NULL,     -- FK to agent.id (not enforced; schema isolation)
    config      JSONB        NOT NULL DEFAULT '{}',
    -- token is the inbound secret used to authenticate incoming messages.
    -- It is auto-generated at creation and must be kept secret.
    token       VARCHAR(255) NOT NULL UNIQUE,
    enabled     BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_channel_agent   ON channel (agent_id);
CREATE INDEX IF NOT EXISTS idx_channel_type    ON channel (type);
CREATE INDEX IF NOT EXISTS idx_channel_enabled ON channel (enabled);
