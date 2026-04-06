-- Add oauth_credential_id to mcp_server_config and remove redundant fields
ALTER TABLE mcp_server_config ADD COLUMN IF NOT EXISTS oauth_credential_id UUID REFERENCES oauth_credential (id) ON DELETE SET NULL;

-- Remove redundant OAuth fields from mcp_server_config
ALTER TABLE mcp_server_config DROP COLUMN IF EXISTS oauth_token_url;
ALTER TABLE mcp_server_config DROP COLUMN IF EXISTS oauth_client_id;
ALTER TABLE mcp_server_config DROP COLUMN IF EXISTS oauth_client_secret;
ALTER TABLE mcp_server_config DROP COLUMN IF EXISTS oauth_scopes;
