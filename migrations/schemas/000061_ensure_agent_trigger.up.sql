-- Repair migration: ensures agent_trigger tables exist for tenants that were
-- at version > 13 when 000013_agent_trigger.up.sql was retroactively added to
-- the repo. golang-migrate skips files whose version is below the current
-- high-water mark, so existing tenants never received migration 13.
CREATE TABLE IF NOT EXISTS agent_trigger (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    cron_expression VARCHAR(100) NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    input_template  JSONB,
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ,
    run_count       INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_trigger_agent_id ON agent_trigger(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_trigger_next_run ON agent_trigger(next_run_at) WHERE enabled = true;

CREATE TABLE IF NOT EXISTS agent_trigger_run (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger_id      UUID NOT NULL REFERENCES agent_trigger(id) ON DELETE CASCADE,
    session_id      UUID NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'running',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    total_turns     INTEGER,
    total_tokens    INTEGER,
    error           TEXT
);

CREATE INDEX IF NOT EXISTS idx_agent_trigger_run_trigger_id ON agent_trigger_run(trigger_id);
