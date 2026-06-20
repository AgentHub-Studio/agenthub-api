DROP INDEX IF EXISTS idx_agent_trigger_notification_webhook_id;

ALTER TABLE agent_trigger
    DROP COLUMN IF EXISTS notification_webhook_id;
