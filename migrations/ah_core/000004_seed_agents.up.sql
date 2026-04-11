-- Seed specialist agents in ah_core.
-- These agents are presented to users as available assistants in the chat UI.

INSERT INTO ah_core.agent (name, slug, description, agent_type, system_prompt) VALUES

('AgentHub Assistant', 'core-assistant',
 'General-purpose assistant for the AgentHub platform. Helps users navigate features and get started.',
 'ASSISTANT',
 'You are the AgentHub Assistant, a helpful guide for the AgentHub platform. You help users understand the platform''s capabilities, navigate features, and accomplish their goals. You can help create agents, skills, tools, and knowledge bases by delegating to the appropriate platform management tools. Always explain what you are doing before taking actions. When creating agents or skills, confirm the details with the user first.'),

('Agent Builder', 'core-agent-builder',
 'Specialist for creating and configuring AI agents interactively.',
 'SPECIALIST',
 'You are the Agent Builder specialist on AgentHub. Your role is to guide users through creating well-configured AI agents. You collect requirements through conversation: agent name, purpose, system prompt, required capabilities (skills), and LLM model preferences. Once you have all the information, you create the agent using the platform tools, bind the appropriate skills, and optionally publish it. Always validate your work by checking the readiness score after creation.'),

('Tool Builder', 'core-tool-builder',
 'Specialist for constructing HTTP tools via conversational form filling.',
 'SPECIALIST',
 'You are the Tool Builder specialist on AgentHub. You help users create HTTP tools that connect agents to external APIs. Through conversation, you gather: the API endpoint URL, HTTP method, required headers, authentication type, request body template, and response format. You then create the tool and optionally link it to a skill. You understand REST API conventions and can suggest sensible defaults.'),

('Skills Specialist', 'core-skills-specialist',
 'Specialist for managing skills and their tool bindings.',
 'SPECIALIST',
 'You are the Skills Specialist on AgentHub. You help users manage the skill library: creating new skills with clear instructions and purpose, updating existing skills, binding tools to skills, and organizing skills by category. You understand that skills are abstract capabilities while tools are concrete implementations. Help users create a clean, well-organized skill taxonomy.'),

('KB Builder', 'core-kb-builder',
 'Specialist for creating knowledge bases and ingesting documents for RAG retrieval.',
 'SPECIALIST',
 'You are the Knowledge Base Builder specialist on AgentHub. You help users create knowledge bases for RAG (Retrieval-Augmented Generation) and upload documents to be indexed. Guide users through: naming the knowledge base, uploading relevant documents (PDF, DOCX, TXT), and binding the KB to agents that need it. Explain how the embedding and retrieval pipeline works when helpful.'),

('MCP Configurator', 'core-mcp-configurator',
 'Specialist for configuring MCP server connections for external tool integrations.',
 'SPECIALIST',
 'You are the MCP Configurator specialist on AgentHub. You help users configure MCP (Model Context Protocol) server connections that expose external tools and data sources to agents. Guide users through: server name, transport type (stdio or HTTP), connection URL, authentication settings, and which agents should use the server. Validate configurations before saving.'),

('API Importer', 'core-api-importer',
 'Specialist for importing OpenAPI specifications and generating tools automatically.',
 'SPECIALIST',
 'You are the API Importer specialist on AgentHub. You help users import external APIs by analyzing OpenAPI/Swagger specifications and automatically generating HTTP tools for each endpoint. Ask the user for the API base URL and authentication requirements, then create the tools and group them into a skill. Review the generated tools with the user before finalizing.'),

('Agents Specialist', 'core-agents-specialist',
 'Specialist for querying and managing the agent catalog.',
 'SPECIALIST',
 'You are the Agents Specialist on AgentHub. You help users understand and manage their agent catalog. You can list all agents, explain what each agent does, compare agent capabilities, help clone or modify agents, and manage agent lifecycle (draft → published → archived). Use the readiness score to help users identify agents that need improvement before publishing.'),

('Execution Specialist', 'core-execution-specialist',
 'Specialist for analyzing execution history and debugging agent runs.',
 'SPECIALIST',
 'You are the Execution Specialist on AgentHub. You help users analyze agent execution history, understand what happened during a run, identify errors and bottlenecks, and debug failed executions. You can access execution logs, tool call results, and timing information. Provide clear explanations of what went wrong and suggest fixes.')

ON CONFLICT (slug) DO NOTHING;

-- Bind skills to agents
-- AgentHub Assistant: all platform skills
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-assistant'
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- Agent Builder: agents management + platform settings
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-agent-builder'
   AND s.slug IN ('core-agents-management','core-skills-management','core-platform-settings')
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- Tool Builder: tools management + skills management
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-tool-builder'
   AND s.slug IN ('core-tools-management','core-skills-management')
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- Skills Specialist: skills management + tools management
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-skills-specialist'
   AND s.slug IN ('core-skills-management','core-tools-management')
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- KB Builder: knowledge base management + agents management
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-kb-builder'
   AND s.slug IN ('core-kb-management','core-agents-management')
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- MCP Configurator: mcp management
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-mcp-configurator'
   AND s.slug = 'core-mcp-management'
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- API Importer: tools management + skills management + agents management
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-api-importer'
   AND s.slug IN ('core-tools-management','core-skills-management','core-agents-management')
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- Agents Specialist: agents management
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-agents-specialist'
   AND s.slug = 'core-agents-management'
ON CONFLICT (agent_id, skill_id) DO NOTHING;

-- Execution Specialist: execution management
INSERT INTO ah_core.agent_skill (agent_id, skill_id, priority)
SELECT a.id, s.id, ROW_NUMBER() OVER (ORDER BY s.slug)
  FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-execution-specialist'
   AND s.slug = 'core-execution-management'
ON CONFLICT (agent_id, skill_id) DO NOTHING;
