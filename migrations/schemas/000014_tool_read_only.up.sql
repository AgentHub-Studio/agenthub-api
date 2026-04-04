-- Add read_only flag to tool table so tools can declare concurrency safety
-- without hardcoding in Go source. Read-only tools execute in parallel.
ALTER TABLE tool ADD COLUMN IF NOT EXISTS read_only BOOLEAN NOT NULL DEFAULT FALSE;

-- Mark known read-only tools from seed data
UPDATE tool SET read_only = TRUE WHERE name IN (
    'agenthub_list_agents', 'agenthub_get_agent',
    'agenthub_list_skills', 'agenthub_get_skill',
    'agenthub_list_tools', 'agenthub_get_tool',
    'agenthub_list_knowledge_bases', 'agenthub_get_knowledge_base', 'agenthub_list_documents',
    'agenthub_get_settings',
    'agenthub_list_mcp_servers',
    'agenthub_list_sessions',
    'agenthub_list_datasources',
    'agenthub_list_prompt_templates'
);
