ALTER TABLE agent_trigger
    ADD COLUMN IF NOT EXISTS notification_webhook_id UUID REFERENCES webhook_config(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_agent_trigger_notification_webhook_id
    ON agent_trigger(notification_webhook_id);
