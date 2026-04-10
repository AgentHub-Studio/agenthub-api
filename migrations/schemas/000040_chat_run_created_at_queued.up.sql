-- P-C299-1: Add created_at to chat_run for queued-run stale detection,
-- and allow started_at to be NULL (queued runs have not started yet).
ALTER TABLE chat_run ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Backfill existing rows: set created_at = started_at for already-completed/failed runs.
UPDATE chat_run SET created_at = started_at WHERE created_at = NOW() AND started_at IS NOT NULL;

-- Allow started_at to be NULL for queued runs (was NOT NULL via DEFAULT).
-- Drop the default so new INSERTs can pass NULL explicitly.
ALTER TABLE chat_run ALTER COLUMN started_at DROP DEFAULT;
ALTER TABLE chat_run ALTER COLUMN started_at DROP NOT NULL;
