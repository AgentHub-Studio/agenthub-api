-- Seed platform management tools in ah_core.
-- All tools use use_caller_token=true so they execute in the caller's tenant context.
-- Base URL is resolved at runtime from the BACKEND_BASE_URL env var.

-- ============================
-- AGENTS
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('List Agents', 'core-list-agents', 'List all agents in the tenant', 'HTTP', '{
  "url": "/api/agents",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Get Agent', 'core-get-agent', 'Get a specific agent by ID', 'HTTP', '{
  "url": "/api/agents/{{input.agentId}}",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Create Agent', 'core-create-agent', 'Create a new agent', 'HTTP', '{
  "url": "/api/agents",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Update Agent', 'core-update-agent', 'Update an existing agent by ID', 'HTTP', '{
  "url": "/api/agents/{{input.agentId}}",
  "method": "PATCH",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Delete Agent', 'core-delete-agent', 'Delete an agent by ID', 'HTTP', '{
  "url": "/api/agents/{{input.agentId}}",
  "method": "DELETE",
  "useCallerToken": true
}'::jsonb),
('Publish Agent', 'core-publish-agent', 'Publish a draft agent', 'HTTP', '{
  "url": "/api/agents/{{input.agentId}}/publish",
  "method": "POST",
  "useCallerToken": true
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- SKILLS
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('List Skills', 'core-list-skills', 'List all skills in the tenant', 'HTTP', '{
  "url": "/api/skills",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Create Skill', 'core-create-skill', 'Create a new skill', 'HTTP', '{
  "url": "/api/skills",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Update Skill', 'core-update-skill', 'Update an existing skill by ID', 'HTTP', '{
  "url": "/api/skills/{{input.skillId}}",
  "method": "PUT",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Delete Skill', 'core-delete-skill', 'Delete a skill by ID', 'HTTP', '{
  "url": "/api/skills/{{input.skillId}}",
  "method": "DELETE",
  "useCallerToken": true
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- TOOLS
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('List Tools', 'core-list-tools', 'List all tools in the tenant', 'HTTP', '{
  "url": "/api/tools",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Create Tool', 'core-create-tool', 'Create a new tool', 'HTTP', '{
  "url": "/api/tools",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Update Tool', 'core-update-tool', 'Update an existing tool by ID', 'HTTP', '{
  "url": "/api/tools/{{input.toolId}}",
  "method": "PUT",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Delete Tool', 'core-delete-tool', 'Delete a tool by ID', 'HTTP', '{
  "url": "/api/tools/{{input.toolId}}",
  "method": "DELETE",
  "useCallerToken": true
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- KNOWLEDGE BASES
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('List Knowledge Bases', 'core-list-kbs', 'List all knowledge bases in the tenant', 'HTTP', '{
  "url": "/api/knowledge-bases",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Create Knowledge Base', 'core-create-kb', 'Create a new knowledge base', 'HTTP', '{
  "url": "/api/knowledge-bases",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Update Knowledge Base', 'core-update-kb', 'Update an existing knowledge base by ID', 'HTTP', '{
  "url": "/api/knowledge-bases/{{input.kbId}}",
  "method": "PUT",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Delete Knowledge Base', 'core-delete-kb', 'Delete a knowledge base by ID', 'HTTP', '{
  "url": "/api/knowledge-bases/{{input.kbId}}",
  "method": "DELETE",
  "useCallerToken": true
}'::jsonb),
('Upload Document', 'core-upload-document', 'Upload a document to a knowledge base', 'HTTP', '{
  "url": "/api/knowledge-bases/{{input.kbId}}/documents",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- EXECUTIONS
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('List Executions', 'core-list-executions', 'List recent agent executions', 'HTTP', '{
  "url": "/api/executions",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Get Execution', 'core-get-execution', 'Get details of a specific execution', 'HTTP', '{
  "url": "/api/executions/{{input.executionId}}",
  "method": "GET",
  "useCallerToken": true
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- MCP SERVERS
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('List MCP Servers', 'core-list-mcp-servers', 'List all MCP server configurations', 'HTTP', '{
  "url": "/api/mcp-servers",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Create MCP Server', 'core-create-mcp-server', 'Create a new MCP server configuration', 'HTTP', '{
  "url": "/api/mcp-servers",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Update MCP Server', 'core-update-mcp-server', 'Update an existing MCP server configuration', 'HTTP', '{
  "url": "/api/mcp-servers/{{input.serverId}}",
  "method": "PUT",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('Delete MCP Server', 'core-delete-mcp-server', 'Delete an MCP server configuration', 'HTTP', '{
  "url": "/api/mcp-servers/{{input.serverId}}",
  "method": "DELETE",
  "useCallerToken": true
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- AGENT BINDINGS
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('Bind Skills to Agent', 'core-bind-agent-skills', 'Set which skills are bound to an agent', 'HTTP', '{
  "url": "/api/agents/{{input.agentId}}/skills",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb),
('List Agent Skills', 'core-list-agent-skills', 'List skills bound to an agent', 'HTTP', '{
  "url": "/api/agents/{{input.agentId}}/skills",
  "method": "GET",
  "useCallerToken": true
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- PLATFORM SETTINGS
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('Get Settings', 'core-get-settings', 'Get tenant platform settings', 'HTTP', '{
  "url": "/api/settings",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Update Settings', 'core-update-settings', 'Update tenant platform settings', 'HTTP', '{
  "url": "/api/settings",
  "method": "PUT",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;

-- ============================
-- EXPORT / IMPORT
-- ============================
INSERT INTO ah_core.tool (name, slug, description, type, config) VALUES
('Export Agent Bundle', 'core-export-agent', 'Export an agent as a portable bundle', 'HTTP', '{
  "url": "/api/agents/{{input.agentId}}/export",
  "method": "GET",
  "useCallerToken": true
}'::jsonb),
('Import Agent Bundle', 'core-import-agent', 'Import an agent bundle into this tenant', 'HTTP', '{
  "url": "/api/agents/import",
  "method": "POST",
  "useCallerToken": true,
  "bodyTemplate": "{{input.body}}"
}'::jsonb)
ON CONFLICT (slug) DO NOTHING;
