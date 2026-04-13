-- BUG-CONCURRENT: add partial unique index to prevent two concurrent runs on the same session.
-- Without this, the TOCTOU race in EnqueueRun allows two requests arriving simultaneously
-- to both pass the GetActiveRunBySession check and create duplicate active runs.
CREATE UNIQUE INDEX IF NOT EXISTS uq_chat_run_one_active_per_session
    ON chat_run (session_id)
    WHERE status IN ('queued', 'active');
