-- Rollback changes to mcp_server_config
ALTER TABLE mcp_server_config ADD COLUMN IF NOT EXISTS oauth_token_url TEXT;
ALTER TABLE mcp_server_config ADD COLUMN IF NOT EXISTS oauth_client_id TEXT;
ALTER TABLE mcp_server_config ADD COLUMN IF NOT EXISTS oauth_client_secret TEXT;
ALTER TABLE mcp_server_config ADD COLUMN IF NOT EXISTS oauth_scopes JSONB NOT NULL DEFAULT '[]';

ALTER TABLE mcp_server_config DROP COLUMN IF EXISTS oauth_credential_id;
