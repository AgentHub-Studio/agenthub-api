-- Seed platform management skills in ah_core.
-- Each skill groups related tools for a specific platform management domain.

-- ============================
-- INSERT SKILLS
-- ============================
INSERT INTO ah_core.skill (name, slug, description, instructions, category, when_to_use) VALUES

('Agents Management', 'core-agents-management',
 'Create, update, delete, and publish agents on the AgentHub platform.',
 'You help users manage agents on the AgentHub platform. You can list existing agents, create new agents with specific capabilities, update agent configurations, delete agents, and publish draft agents to make them available.',
 'platform',
 'Use when the user wants to create, modify, or manage AI agents on the platform.'),

('Skills Management', 'core-skills-management',
 'Create, update, and delete skills that agents can use.',
 'You help users manage skills on the AgentHub platform. Skills are capabilities that agents can use to accomplish tasks. You can list existing skills, create new skills with instructions and tool bindings, update skill configurations, and delete skills.',
 'platform',
 'Use when the user wants to create, update, or organize skills for agents.'),

('Tools Management', 'core-tools-management',
 'Create, update, and delete tools that implement skill capabilities.',
 'You help users manage tools on the AgentHub platform. Tools are the concrete implementations of skills — HTTP endpoints, SQL queries, document searches. You can list existing tools, create new HTTP tools pointing to external APIs, update tool configurations, and delete tools.',
 'platform',
 'Use when the user wants to create or modify HTTP tools, SQL tools, or other tool types.'),

('Knowledge Base Management', 'core-kb-management',
 'Create knowledge bases and upload documents for RAG retrieval.',
 'You help users manage knowledge bases on the AgentHub platform. You can create new knowledge bases, update their descriptions, delete them, and upload documents for indexing. Documents are processed automatically for embedding and retrieval.',
 'platform',
 'Use when the user wants to create a knowledge base or upload documents for their agents to search.'),

('Execution Management', 'core-execution-management',
 'View and analyze agent execution history.',
 'You help users view and understand agent execution history. You can list recent executions, get details of specific executions including tool calls and results, and help diagnose failed or slow executions.',
 'platform',
 'Use when the user wants to see what an agent has done, debug failed runs, or analyze execution performance.'),

('MCP Server Management', 'core-mcp-management',
 'Configure MCP server connections for external tool integrations.',
 'You help users configure MCP (Model Context Protocol) server connections. These connect agents to external tools and data sources via the MCP standard. You can list existing MCP servers, create new server configurations, update connection settings, and delete server configs.',
 'platform',
 'Use when the user wants to connect an external MCP-compatible server or tool to their agents.'),

('Platform Settings', 'core-platform-settings',
 'View and update tenant platform configuration.',
 'You help users view and update their AgentHub tenant settings including LLM provider configurations, default model settings, and other platform preferences.',
 'platform',
 'Use when the user wants to configure LLM providers, update API keys, or adjust platform-level settings.')

ON CONFLICT (slug) DO NOTHING;

-- ============================
-- BIND TOOLS TO SKILLS
-- ============================

-- Agents Management → agent tools
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, ROW_NUMBER() OVER (ORDER BY t.slug)
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-agents-management'
   AND t.slug IN ('core-list-agents','core-get-agent','core-create-agent','core-update-agent','core-delete-agent','core-publish-agent','core-bind-agent-skills','core-list-agent-skills','core-export-agent','core-import-agent')
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Skills Management → skill tools
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, ROW_NUMBER() OVER (ORDER BY t.slug)
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-skills-management'
   AND t.slug IN ('core-list-skills','core-create-skill','core-update-skill','core-delete-skill')
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Tools Management → tool tools
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, ROW_NUMBER() OVER (ORDER BY t.slug)
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-tools-management'
   AND t.slug IN ('core-list-tools','core-create-tool','core-update-tool','core-delete-tool')
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Knowledge Base Management → kb tools
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, ROW_NUMBER() OVER (ORDER BY t.slug)
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-kb-management'
   AND t.slug IN ('core-list-kbs','core-create-kb','core-update-kb','core-delete-kb','core-upload-document')
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Execution Management → execution tools
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, ROW_NUMBER() OVER (ORDER BY t.slug)
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-execution-management'
   AND t.slug IN ('core-list-executions','core-get-execution')
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- MCP Management → mcp tools
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, ROW_NUMBER() OVER (ORDER BY t.slug)
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-mcp-management'
   AND t.slug IN ('core-list-mcp-servers','core-create-mcp-server','core-update-mcp-server','core-delete-mcp-server')
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Platform Settings → settings tools
INSERT INTO ah_core.skill_tool (skill_id, tool_id, priority)
SELECT s.id, t.id, ROW_NUMBER() OVER (ORDER BY t.slug)
  FROM ah_core.skill s, ah_core.tool t
 WHERE s.slug = 'core-platform-settings'
   AND t.slug IN ('core-get-settings','core-update-settings')
ON CONFLICT (skill_id, tool_id) DO NOTHING;
