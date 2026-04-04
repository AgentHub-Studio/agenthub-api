-- Seed builtin skills, tools, skill-tool bindings, prompt templates,
-- and the AgentHub Assistant agent for agentic self-management.
--
-- Ported from Java V1.0.0.001/V1.0.0.002 and updated for the agentic model:
--   - Pipelines removed (deprecated per ADR-012)
--   - Uses system_prompt + model_config instead of legacy agent columns
--   - Uses agent_skill bindings instead of tools[] array
--   - Added new skills: mcp-management, chat-management, datasource-management
--   - Added prompt templates for common agent personas

-- =====================================================================
-- CORE SKILLS (6 generic + 9 platform management)
-- =====================================================================

-- Generic skills (used by any agent)
INSERT INTO skill (id, name, slug, description, category, input_schema, created_at, updated_at)
VALUES
('b1000000-0000-0000-0000-000000000001', 'Document Search', 'document-search',
 'Searches documents in the knowledge base by semantic similarity. Use when the user asks about information that may exist in uploaded documents or company knowledge bases. The ''query'' parameter should be a descriptive sentence or question, not isolated keywords — for example, use ''how to reset a user password'' instead of ''password reset''. Returns relevant text excerpts with similarity scores, sorted by relevance.',
 'rag', '{"type":"object","properties":{"query":{"type":"string","description":"Semantic search query"},"limit":{"type":"integer","description":"Max results (default 5)"}},"required":["query"]}', NOW(), NOW()),

('b1000000-0000-0000-0000-000000000002', 'Execute SQL', 'execute-sql',
 'Executes SQL queries against configured PostgreSQL datasources. Use when the user needs to query, analyze, or explore structured data. The ''query'' parameter must be valid SQL. Always use SELECT for reads; NEVER execute UPDATE, DELETE, or DROP without explicit user confirmation. Use LIMIT to avoid returning excessive rows. Returns rows as an array of objects with column names as keys.',
 'data', '{"type":"object","properties":{"query":{"type":"string","description":"SQL query to execute"},"datasourceId":{"type":"string","description":"Datasource UUID (optional if only one configured)"}},"required":["query"]}', NOW(), NOW()),

('b1000000-0000-0000-0000-000000000003', 'HTTP Request', 'http-request',
 'Makes HTTP requests to external APIs and services. Use when the user needs to interact with a REST API, fetch data from a URL, or trigger a webhook. Parameters include ''method'' (GET/POST/PUT/DELETE), ''url'', ''headers'' (object), and ''body'' (JSON). Always include required headers like Content-Type and Authorization. Returns the HTTP status code, response headers, and response body.',
 'integration', '{"type":"object","properties":{"method":{"type":"string","enum":["GET","POST","PUT","PATCH","DELETE"],"description":"HTTP method"},"url":{"type":"string","description":"Full URL including protocol"},"headers":{"type":"object","description":"Request headers"},"body":{"type":"object","description":"Request body (for POST/PUT/PATCH)"}},"required":["method","url"]}', NOW(), NOW()),

('b1000000-0000-0000-0000-000000000004', 'Send Email', 'send-email',
 'Sends an email via the configured email service. Use when the user explicitly asks to send, forward, or reply to an email. ALWAYS confirm recipient, subject, and content with the user before sending. Parameters: ''to'' (email address), ''subject'', ''body'' (plain text or HTML), ''cc'' (optional). Returns a confirmation with the message ID.',
 'integration', '{"type":"object","properties":{"to":{"type":"string","description":"Recipient email"},"subject":{"type":"string","description":"Email subject"},"body":{"type":"string","description":"Email body (plain text or HTML)"},"cc":{"type":"string","description":"CC recipients (comma-separated)"}},"required":["to","subject","body"]}', NOW(), NOW()),

('b1000000-0000-0000-0000-000000000005', 'Web Scraper', 'web-scraper',
 'Extracts content from a web page given its URL. Use when the user provides a URL and wants to read, summarize, or analyze its content. The ''url'' parameter should be a complete URL including the protocol (https://). Returns the page title, main text content, and metadata. Note: some pages may block automated access.',
 'data', '{"type":"object","properties":{"url":{"type":"string","description":"Full URL including https://"}},"required":["url"]}', NOW(), NOW()),

('b1000000-0000-0000-0000-000000000006', 'Code Interpreter', 'code-interpreter',
 'Executes code snippets in a sandboxed environment. Use when the user needs calculations, data transformations, or script execution. The ''code'' parameter should be the complete code to run. The ''language'' parameter specifies the runtime (python, javascript). Returns stdout, stderr, and any generated files or visualizations.',
 'compute', '{"type":"object","properties":{"code":{"type":"string","description":"Code to execute"},"language":{"type":"string","enum":["python","javascript","groovy"],"description":"Runtime language"}},"required":["code","language"]}', NOW(), NOW())
ON CONFLICT (slug) DO NOTHING;

-- Platform management skills
INSERT INTO skill (id, name, slug, description, category, input_schema, created_at, updated_at)
VALUES
('b1000000-0000-0000-0001-000000000001', 'Agent Management', 'agent-management',
 'Create, read, update, delete, publish, and archive agents in the AgentHub platform. Includes version management and cloning.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000002', 'Skill Management', 'skill-management',
 'Create and manage skills (abstract capabilities that group tools). Includes skill-tool binding management.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000003', 'Tool Management', 'tool-management',
 'Create and manage tools (concrete implementations: HTTP, SQL, Code, Documents, Blockly). Includes testing and code generation.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000004', 'Knowledge Base Management', 'knowledge-base-management',
 'Create and manage knowledge bases for RAG (Retrieval-Augmented Generation). Upload documents, manage indexing, activate/pause.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000005', 'Settings Management', 'settings-management',
 'View and update tenant settings including AI provider configuration, SMTP, and general preferences.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000006', 'MCP Server Management', 'mcp-management',
 'Create and manage MCP (Model Context Protocol) server configurations. Supports stdio and HTTP transports with OAuth.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000007', 'Chat Session Management', 'chat-management',
 'List, create, archive, and delete chat sessions. Manage conversation history and session metadata.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000008', 'Datasource Management', 'datasource-management',
 'Create and manage database datasources (PostgreSQL, MySQL, SQL Server) with optional VPN tunnel configuration.',
 'platform', '{}', NOW(), NOW()),

('b1000000-0000-0000-0001-000000000009', 'Prompt Template Management', 'prompt-template-management',
 'Create and manage prompt templates for agents. Templates provide reusable system prompts with categorization.',
 'platform', '{}', NOW(), NOW())
ON CONFLICT (slug) DO NOTHING;


-- =====================================================================
-- PLATFORM MANAGEMENT TOOLS (HTTP tools calling AgentHub API)
-- =====================================================================

-- Agent tools
INSERT INTO tool (id, name, type, description, config, labels, created_at, updated_at) VALUES
('a1000000-0000-0000-0001-000000000001', 'agenthub_list_agents', 'HTTP',
 'Lists all agents with pagination. Returns name, status, description.',
 '{"url": "/api/agents?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}}}}',
 ARRAY['platform','agents','read'], NOW(), NOW()),

('a1000000-0000-0000-0001-000000000002', 'agenthub_get_agent', 'HTTP',
 'Gets detailed agent information by ID.',
 '{"url": "/api/agents/{id}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string","description":"Agent UUID"}},"required":["id"]}}',
 ARRAY['platform','agents','read'], NOW(), NOW()),

('a1000000-0000-0000-0001-000000000003', 'agenthub_create_agent', 'HTTP',
 'Creates a new agent. ALWAYS confirm with user before calling.',
 '{"url": "/api/agents", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"name":{"type":"string"},"slug":{"type":"string"},"description":{"type":"string"},"systemPrompt":{"type":"string"}},"required":["name","description"]}}',
 ARRAY['platform','agents','write'], NOW(), NOW()),

('a1000000-0000-0000-0001-000000000004', 'agenthub_update_agent', 'HTTP',
 'Updates an existing agent. ALWAYS confirm with user before calling.',
 '{"url": "/api/agents/{id}", "method": "PUT", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"description":{"type":"string"},"systemPrompt":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','agents','write'], NOW(), NOW()),

('a1000000-0000-0000-0001-000000000005', 'agenthub_delete_agent', 'HTTP',
 'Deletes an agent permanently. ALWAYS confirm with user before calling.',
 '{"url": "/api/agents/{id}", "method": "DELETE", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','agents','write'], NOW(), NOW()),

('a1000000-0000-0000-0001-000000000006', 'agenthub_publish_agent', 'HTTP',
 'Publishes an agent (changes status from DRAFT to PUBLISHED).',
 '{"url": "/api/agents/{id}/publish", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','agents','write'], NOW(), NOW()),

('a1000000-0000-0000-0001-000000000007', 'agenthub_clone_agent', 'HTTP',
 'Clones an existing agent with a new name.',
 '{"url": "/api/agents/{id}/clone", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}},"required":["id","name"]}}',
 ARRAY['platform','agents','write'], NOW(), NOW()),

-- Skill tools
('a1000000-0000-0000-0002-000000000001', 'agenthub_list_skills', 'HTTP',
 'Lists all skills with pagination and optional category filter.',
 '{"url": "/api/skills?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":20},"category":{"type":"string"}}}}',
 ARRAY['platform','skills','read'], NOW(), NOW()),

('a1000000-0000-0000-0002-000000000002', 'agenthub_get_skill', 'HTTP',
 'Gets detailed skill information by ID.',
 '{"url": "/api/skills/{id}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','skills','read'], NOW(), NOW()),

('a1000000-0000-0000-0002-000000000003', 'agenthub_create_skill', 'HTTP',
 'Creates a new skill. ALWAYS confirm with user.',
 '{"url": "/api/skills", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"name":{"type":"string"},"slug":{"type":"string"},"description":{"type":"string"},"category":{"type":"string"}},"required":["name","slug"]}}',
 ARRAY['platform','skills','write'], NOW(), NOW()),

('a1000000-0000-0000-0002-000000000004', 'agenthub_delete_skill', 'HTTP',
 'Deletes a skill permanently. ALWAYS confirm with user.',
 '{"url": "/api/skills/{id}", "method": "DELETE", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','skills','write'], NOW(), NOW()),

-- Tool tools
('a1000000-0000-0000-0003-000000000001', 'agenthub_list_tools', 'HTTP',
 'Lists all tools with pagination and optional type filter.',
 '{"url": "/api/tools?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":20},"type":{"type":"string"}}}}',
 ARRAY['platform','tools','read'], NOW(), NOW()),

('a1000000-0000-0000-0003-000000000002', 'agenthub_get_tool', 'HTTP',
 'Gets detailed tool information by ID.',
 '{"url": "/api/tools/{id}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','tools','read'], NOW(), NOW()),

('a1000000-0000-0000-0003-000000000003', 'agenthub_create_tool', 'HTTP',
 'Creates a new tool. ALWAYS confirm with user.',
 '{"url": "/api/tools", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"name":{"type":"string"},"type":{"type":"string","enum":["HTTP","SQL","CODE","CUSTOM"]},"description":{"type":"string"},"config":{"type":"object"}},"required":["name","type"]}}',
 ARRAY['platform','tools','write'], NOW(), NOW()),

('a1000000-0000-0000-0003-000000000004', 'agenthub_delete_tool', 'HTTP',
 'Deletes a tool permanently. ALWAYS confirm with user.',
 '{"url": "/api/tools/{id}", "method": "DELETE", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','tools','write'], NOW(), NOW()),

('a1000000-0000-0000-0003-000000000005', 'agenthub_test_tool', 'HTTP',
 'Tests a tool by executing it with sample inputs. Returns the raw tool output.',
 '{"url": "/api/tools/{id}/test", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"},"inputs":{"type":"object"}},"required":["id"]}}',
 ARRAY['platform','tools','write'], NOW(), NOW()),

-- Knowledge Base tools
('a1000000-0000-0000-0004-000000000001', 'agenthub_list_knowledge_bases', 'HTTP',
 'Lists all knowledge bases with pagination.',
 '{"url": "/api/knowledge-bases?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}}}}',
 ARRAY['platform','knowledge-bases','read'], NOW(), NOW()),

('a1000000-0000-0000-0004-000000000002', 'agenthub_get_knowledge_base', 'HTTP',
 'Gets detailed knowledge base information by ID.',
 '{"url": "/api/knowledge-bases/{id}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','knowledge-bases','read'], NOW(), NOW()),

('a1000000-0000-0000-0004-000000000003', 'agenthub_create_knowledge_base', 'HTTP',
 'Creates a new knowledge base. ALWAYS confirm with user.',
 '{"url": "/api/knowledge-bases", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"name":{"type":"string"},"description":{"type":"string"}},"required":["name"]}}',
 ARRAY['platform','knowledge-bases','write'], NOW(), NOW()),

('a1000000-0000-0000-0004-000000000004', 'agenthub_delete_knowledge_base', 'HTTP',
 'Deletes a knowledge base and all its documents. ALWAYS confirm with user.',
 '{"url": "/api/knowledge-bases/{id}", "method": "DELETE", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}',
 ARRAY['platform','knowledge-bases','write'], NOW(), NOW()),

('a1000000-0000-0000-0004-000000000005', 'agenthub_list_documents', 'HTTP',
 'Lists documents in a knowledge base with status and metadata.',
 '{"url": "/api/knowledge-bases/{knowledgeBaseId}/documents?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"knowledgeBaseId":{"type":"string"},"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}},"required":["knowledgeBaseId"]}}',
 ARRAY['platform','knowledge-bases','read'], NOW(), NOW()),

-- Settings tools
('a1000000-0000-0000-0005-000000000001', 'agenthub_get_settings', 'HTTP',
 'Gets tenant settings (AI providers, SMTP, preferences).',
 '{"url": "/api/settings", "method": "GET", "useCallerToken": true}',
 ARRAY['platform','settings','read'], NOW(), NOW()),

('a1000000-0000-0000-0005-000000000002', 'agenthub_update_settings', 'HTTP',
 'Updates tenant settings. ALWAYS confirm with user.',
 '{"url": "/api/settings/{key}", "method": "PUT", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"key":{"type":"string"},"value":{"type":"object"}},"required":["key","value"]}}',
 ARRAY['platform','settings','write'], NOW(), NOW()),

-- MCP tools
('a1000000-0000-0000-0006-000000000001', 'agenthub_list_mcp_servers', 'HTTP',
 'Lists all MCP server configurations.',
 '{"url": "/api/mcp-server-configs?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}}}}',
 ARRAY['platform','mcp','read'], NOW(), NOW()),

('a1000000-0000-0000-0006-000000000002', 'agenthub_create_mcp_server', 'HTTP',
 'Creates a new MCP server configuration. ALWAYS confirm with user.',
 '{"url": "/api/mcp-server-configs", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"name":{"type":"string"},"transportType":{"type":"string","enum":["stdio","http"]},"httpBaseUrl":{"type":"string"},"command":{"type":"string"},"autoStart":{"type":"boolean"}},"required":["name","transportType"]}}',
 ARRAY['platform','mcp','write'], NOW(), NOW()),

-- Chat tools
('a1000000-0000-0000-0007-000000000001', 'agenthub_list_sessions', 'HTTP',
 'Lists chat sessions with pagination.',
 '{"url": "/api/chat/sessions?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":50}}}}',
 ARRAY['platform','chat','read'], NOW(), NOW()),

('a1000000-0000-0000-0007-000000000002', 'agenthub_create_session', 'HTTP',
 'Creates a new chat session, optionally linked to an agent.',
 '{"url": "/api/chat/sessions", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"title":{"type":"string"},"agentId":{"type":"string"}},"required":["title"]}}',
 ARRAY['platform','chat','write'], NOW(), NOW()),

-- Datasource tools
('a1000000-0000-0000-0008-000000000001', 'agenthub_list_datasources', 'HTTP',
 'Lists all database datasources.',
 '{"url": "/api/datasources?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}}}}',
 ARRAY['platform','datasources','read'], NOW(), NOW()),

('a1000000-0000-0000-0008-000000000002', 'agenthub_create_datasource', 'HTTP',
 'Creates a new database datasource. ALWAYS confirm with user.',
 '{"url": "/api/datasources", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"name":{"type":"string"},"type":{"type":"string","enum":["POSTGRESQL","MYSQL","SQL_SERVER"]},"host":{"type":"string"},"port":{"type":"integer"},"database":{"type":"string"},"dbUser":{"type":"string"},"dbPassword":{"type":"string"}},"required":["name","type","host","port","database","dbUser","dbPassword"]}}',
 ARRAY['platform','datasources','write'], NOW(), NOW()),

-- Prompt Template tools
('a1000000-0000-0000-0009-000000000001', 'agenthub_list_prompt_templates', 'HTTP',
 'Lists all prompt templates with pagination.',
 '{"url": "/api/prompt-templates?page={page}&size={size}", "method": "GET", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"page":{"type":"integer","default":0},"size":{"type":"integer","default":20}}}}',
 ARRAY['platform','templates','read'], NOW(), NOW()),

('a1000000-0000-0000-0009-000000000002', 'agenthub_create_prompt_template', 'HTTP',
 'Creates a new prompt template. ALWAYS confirm with user.',
 '{"url": "/api/prompt-templates", "method": "POST", "useCallerToken": true, "inputSchema": {"type":"object","properties":{"name":{"type":"string"},"slug":{"type":"string"},"description":{"type":"string"},"content":{"type":"string"},"category":{"type":"string","enum":["general","rag","data","api","custom"]}},"required":["name","slug","content"]}}',
 ARRAY['platform','templates','write'], NOW(), NOW())
ON CONFLICT DO NOTHING;


-- =====================================================================
-- SKILL-TOOL BINDINGS (platform skills → platform tools)
-- =====================================================================

-- Agent Management (7 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000001', 'a1000000-0000-0000-0001-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000001', 'a1000000-0000-0000-0001-000000000002', 100, true),
('b1000000-0000-0000-0001-000000000001', 'a1000000-0000-0000-0001-000000000003', 100, true),
('b1000000-0000-0000-0001-000000000001', 'a1000000-0000-0000-0001-000000000004', 100, true),
('b1000000-0000-0000-0001-000000000001', 'a1000000-0000-0000-0001-000000000005', 100, true),
('b1000000-0000-0000-0001-000000000001', 'a1000000-0000-0000-0001-000000000006', 100, true),
('b1000000-0000-0000-0001-000000000001', 'a1000000-0000-0000-0001-000000000007', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Skill Management (4 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000002', 'a1000000-0000-0000-0002-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000002', 'a1000000-0000-0000-0002-000000000002', 100, true),
('b1000000-0000-0000-0001-000000000002', 'a1000000-0000-0000-0002-000000000003', 100, true),
('b1000000-0000-0000-0001-000000000002', 'a1000000-0000-0000-0002-000000000004', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Tool Management (5 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000003', 'a1000000-0000-0000-0003-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000003', 'a1000000-0000-0000-0003-000000000002', 100, true),
('b1000000-0000-0000-0001-000000000003', 'a1000000-0000-0000-0003-000000000003', 100, true),
('b1000000-0000-0000-0001-000000000003', 'a1000000-0000-0000-0003-000000000004', 100, true),
('b1000000-0000-0000-0001-000000000003', 'a1000000-0000-0000-0003-000000000005', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Knowledge Base Management (5 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000004', 'a1000000-0000-0000-0004-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000004', 'a1000000-0000-0000-0004-000000000002', 100, true),
('b1000000-0000-0000-0001-000000000004', 'a1000000-0000-0000-0004-000000000003', 100, true),
('b1000000-0000-0000-0001-000000000004', 'a1000000-0000-0000-0004-000000000004', 100, true),
('b1000000-0000-0000-0001-000000000004', 'a1000000-0000-0000-0004-000000000005', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Settings Management (2 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000005', 'a1000000-0000-0000-0005-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000005', 'a1000000-0000-0000-0005-000000000002', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- MCP Management (2 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000006', 'a1000000-0000-0000-0006-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000006', 'a1000000-0000-0000-0006-000000000002', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Chat Management (2 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000007', 'a1000000-0000-0000-0007-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000007', 'a1000000-0000-0000-0007-000000000002', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Datasource Management (2 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000008', 'a1000000-0000-0000-0008-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000008', 'a1000000-0000-0000-0008-000000000002', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;

-- Prompt Template Management (2 tools)
INSERT INTO skill_tool (skill_id, tool_id, priority, is_active) VALUES
('b1000000-0000-0000-0001-000000000009', 'a1000000-0000-0000-0009-000000000001', 100, true),
('b1000000-0000-0000-0001-000000000009', 'a1000000-0000-0000-0009-000000000002', 100, true)
ON CONFLICT (skill_id, tool_id) DO NOTHING;


-- =====================================================================
-- AGENT: AgentHub Assistant (agentic mode — no pipeline)
-- =====================================================================

INSERT INTO agent (id, name, slug, description, status, system_prompt, model_config, permission_rules, config, created_at, updated_at)
VALUES (
    'd1000000-0000-0000-0001-000000000001',
    'AgentHub Assistant',
    'agenthub-assistant',
    'AI assistant that manages the AgentHub platform itself. Creates agents, skills, tools, knowledge bases, datasources, and more via conversational interface.',
    'PUBLISHED',
    'You are the **AgentHub Assistant** — an AI that manages the AgentHub platform through its REST API.

## Capabilities
You can manage: agents, skills, tools, knowledge bases, datasources, MCP servers, chat sessions, prompt templates, and tenant settings.

## Rules
1. **Before ANY write operation** (create, update, delete, publish), show exactly what will be done and ask for confirmation.
2. For **list** operations, format results as clean markdown tables.
3. For **detail** operations, format as structured markdown with key fields highlighted.
4. When **creating** resources, suggest reasonable defaults if the user does not specify all fields.
5. **Never** expose internal IDs unless the user asks for them.
6. Be concise but thorough. Explain what each operation does when the user seems unsure.

## Format
- Use markdown for all responses.
- Tables for lists, bullets for details.
- Code blocks for SQL, JSON, or configuration examples.',
    '{"provider": "anthropic", "model": "claude-sonnet-4-6", "temperature": 0.7, "maxTokens": 4096, "maxIterations": 15, "maxDepth": 2}'::jsonb,
    '{"deny": ["execute-sql(DROP)", "execute-sql(DELETE)", "execute-sql(TRUNCATE)"], "mode": "allow_edits"}'::jsonb,
    '{"tags": ["platform", "self-management"]}'::jsonb,
    NOW(), NOW()
) ON CONFLICT (slug) DO NOTHING;

-- Link AgentHub Assistant to all platform management skills
INSERT INTO agent_skill (agent_id, skill_id) VALUES
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000001'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000002'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000003'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000004'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000005'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000006'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000007'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000008'),
('d1000000-0000-0000-0001-000000000001', 'b1000000-0000-0000-0001-000000000009')
ON CONFLICT DO NOTHING;


-- =====================================================================
-- PROMPT TEMPLATES (5 built-in personas)
-- =====================================================================

INSERT INTO prompt_template (id, name, slug, description, content, category, created_at, updated_at)
VALUES
('e1000000-0000-0000-0001-000000000001', 'General Assistant', 'general-assistant',
 'A helpful general-purpose AI assistant.',
 'You are a helpful, knowledgeable assistant. Answer questions clearly and concisely.

## Rules
- Be direct and helpful
- When you do not know something, say so — never invent information
- Use tools when available to provide accurate, current information
- Format responses in markdown when it improves readability',
 'general', NOW(), NOW()),

('e1000000-0000-0000-0001-000000000002', 'RAG Document Analyst', 'rag-document-analyst',
 'Specializes in document search and analysis using knowledge bases.',
 'You are a document analyst assistant. Your primary job is to find and synthesize information from the knowledge bases available to you.

## Rules
1. **ALWAYS search** the knowledge base before answering questions about documented topics
2. Cite the source document when providing information
3. If the knowledge base does not contain the answer, say so clearly
4. Synthesize information from multiple documents when relevant
5. Present findings in a structured format with clear sections',
 'rag', NOW(), NOW()),

('e1000000-0000-0000-0001-000000000003', 'Data Analyst', 'data-analyst',
 'Analyzes data using SQL queries against configured datasources.',
 'You are a data analyst assistant with access to SQL databases.

## Rules
1. Always explain the SQL query before executing it
2. Use SELECT with LIMIT for initial exploration (max 100 rows)
3. **NEVER** execute UPDATE, DELETE, DROP, or TRUNCATE without explicit user confirmation
4. Present query results as formatted tables
5. Provide insights and observations about the data
6. Suggest follow-up queries when patterns emerge',
 'data', NOW(), NOW()),

('e1000000-0000-0000-0001-000000000004', 'API Integration Specialist', 'api-integration',
 'Manages REST API integrations and HTTP requests.',
 'You are an API integration specialist. You help users interact with external REST APIs.

## Rules
1. Validate URLs and parameters before making requests
2. Handle authentication (Bearer tokens, API keys) carefully — never log credentials
3. Parse and format JSON responses for readability
4. Explain HTTP status codes and error responses
5. Suggest retry strategies for transient failures',
 'api', NOW(), NOW()),

('e1000000-0000-0000-0001-000000000005', 'Platform Administrator', 'platform-admin',
 'Manages the AgentHub platform: agents, skills, tools, KBs, settings.',
 'You are the AgentHub Platform Administrator. You manage all aspects of the platform.

## Capabilities
Agents, skills, tools, knowledge bases, datasources, MCP servers, chat sessions, prompt templates, and tenant settings.

## Rules
1. Before ANY write operation, show exactly what will be done and ask for confirmation
2. Format lists as markdown tables
3. Suggest reasonable defaults when creating resources
4. Warn about destructive operations (delete, archive)
5. Track related resources (e.g., deleting a skill may affect agents using it)',
 'custom', NOW(), NOW())
ON CONFLICT DO NOTHING;
