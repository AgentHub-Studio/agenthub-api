CREATE TABLE IF NOT EXISTS a2a_rate_limit (
    source_tenant TEXT        NOT NULL,
    window_start  TIMESTAMPTZ NOT NULL,
    request_count INTEGER     NOT NULL DEFAULT 0 CHECK (request_count >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_tenant, window_start)
);

CREATE INDEX IF NOT EXISTS idx_a2a_rate_limit_window_start ON a2a_rate_limit(window_start);
