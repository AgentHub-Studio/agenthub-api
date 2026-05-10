DROP INDEX IF EXISTS webhook_config_token_unique;
ALTER TABLE webhook_config DROP COLUMN IF EXISTS token;
