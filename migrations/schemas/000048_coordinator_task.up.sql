-- IMPROVEMENT-TASK-03: persist CoordinatorTask and TaskNotification for multi-step
-- execution tracking. These tables live per-tenant so they are scoped to the
-- tenant's session data. coordinator_task stores the unit-of-work records
-- created by a coordinator agent; task_notification stores progress reports
-- sent by worker agents back to the coordinator.

CREATE TABLE IF NOT EXISTS coordinator_task (
    id          TEXT        NOT NULL,
    session_id  UUID        NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    description TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending',
    phase       TEXT        NOT NULL DEFAULT 'research',
    depends_on  TEXT[]      NOT NULL DEFAULT '{}',
    assigned_to TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_coordinator_task_session ON coordinator_task(session_id);
CREATE INDEX IF NOT EXISTS idx_coordinator_task_status  ON coordinator_task(session_id, status);

CREATE TABLE IF NOT EXISTS task_notification (
    id          TEXT        NOT NULL,
    task_id     TEXT        NOT NULL REFERENCES coordinator_task(id) ON DELETE CASCADE,
    worker_id   TEXT        NOT NULL,
    worker_name TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL,
    summary     TEXT        NOT NULL DEFAULT '',
    findings    JSONB,
    error       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_task_notification_task ON task_notification(task_id);
