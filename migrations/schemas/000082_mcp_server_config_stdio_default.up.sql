-- Keep the tenant runtime default aligned with the canonical MCP schema.
ALTER TABLE mcp_server_config
    ALTER COLUMN transport_type SET DEFAULT 'stdio';
