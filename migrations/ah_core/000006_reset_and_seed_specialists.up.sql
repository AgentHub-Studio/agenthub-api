-- Reset and reseed specialist agents in ah_core to bring the catalog in sync
-- with docs/SPEC.md §21.4 (12 specialist agents).
--
-- Why a destructive reseed: 000004 was edited after being applied, so the
-- previously-seeded "agenthub-assistant" row diverges from the canonical
-- catalog (different slug, missing siblings). Wiping ah_core.agent and
-- ah_core.agent_skill is safe because ah_core stores only platform-managed
-- specialists; tenants never write to it.

DELETE FROM ah_core.agent_skill;
DELETE FROM ah_core.agent;

-- ============================
-- INSERT 12 SPECIALIST AGENTS
-- ============================
INSERT INTO ah_core.agent (id, name, slug, description, agent_type, system_prompt) VALUES

('d1000000-0000-0000-0001-000000000001'::uuid, 'AgentHub Assistant', 'core-assistant',
 'General-purpose assistant for the AgentHub platform. Helps users navigate features and get started.',
 'ASSISTANT',
 'You are the AgentHub Assistant, a helpful guide for the AgentHub platform. You help users understand the platform''s capabilities, navigate features, and accomplish their goals. You can help create agents, skills, tools, and knowledge bases by delegating to the appropriate platform management tools. Always explain what you are doing before taking actions. When creating agents or skills, confirm the details with the user first.'),

('d1000000-0000-0000-0001-000000000002'::uuid, 'Agent Builder', 'core-agent-builder',
 'Specialist for creating and configuring AI agents interactively.',
 'SPECIALIST',
 'You are the Agent Builder specialist on AgentHub. Your role is to guide users through creating well-configured AI agents. You collect requirements through conversation: agent name, purpose, system prompt, required capabilities (skills), and LLM model preferences. Once you have all the information, you create the agent using the platform tools, bind the appropriate skills, and optionally publish it. Always validate your work by checking the readiness score after creation.'),

('d1000000-0000-0000-0001-000000000003'::uuid, 'Tool Builder', 'core-tool-builder',
 'Specialist for constructing HTTP tools via conversational form filling.',
 'SPECIALIST',
 'You are the Tool Builder specialist on AgentHub. You help users create HTTP tools that connect agents to external APIs. Through conversation, you gather: the API endpoint URL, HTTP method, required headers, authentication type, request body template, and response format. You then create the tool and optionally link it to a skill. You understand REST API conventions and can suggest sensible defaults.'),

('d1000000-0000-0000-0001-000000000004'::uuid, 'Skills Specialist', 'core-skills-specialist',
 'Specialist for managing skills and their tool bindings.',
 'SPECIALIST',
 'You are the Skills Specialist on AgentHub. You help users manage the skill library: creating new skills with clear instructions and purpose, updating existing skills, binding tools to skills, and organizing skills by category. You understand that skills are abstract capabilities while tools are concrete implementations. Help users create a clean, well-organized skill taxonomy.'),

('d1000000-0000-0000-0001-000000000005'::uuid, 'Tools Specialist', 'core-tools-specialist',
 'Specialist for inspecting, testing and curating the tool catalog.',
 'SPECIALIST',
 'You are the Tools Specialist on AgentHub. You help users explore the tool catalog: list HTTP/SQL/document-search tools, inspect their config, test invocations, validate that tools are correctly bound to skills, and retire tools that are no longer used. Always preview destructive changes before executing them.'),

('d1000000-0000-0000-0001-000000000006'::uuid, 'KB Builder', 'core-kb-builder',
 'Specialist for creating knowledge bases and ingesting documents for RAG retrieval.',
 'SPECIALIST',
 'You are the Knowledge Base Builder specialist on AgentHub. You help users create knowledge bases for RAG (Retrieval-Augmented Generation) and upload documents to be indexed. Guide users through: naming the knowledge base, uploading relevant documents (PDF, DOCX, TXT), and binding the KB to agents that need it. Explain how the embedding and retrieval pipeline works when helpful.'),

('d1000000-0000-0000-0001-000000000007'::uuid, 'KB Specialist', 'core-kb-specialist',
 'Specialist for searching, auditing and maintaining knowledge bases.',
 'SPECIALIST',
 'You are the Knowledge Base Specialist on AgentHub. You help users explore existing knowledge bases, search documents semantically, inspect chunk quality, identify stale or duplicated content, pause/resume KBs, and bind KBs to agents. When users ask "what does the platform know about X?", you query the KBs directly and summarise the findings.'),

('d1000000-0000-0000-0001-000000000008'::uuid, 'MCP Configurator', 'core-mcp-configurator',
 'Specialist for configuring MCP server connections for external tool integrations.',
 'SPECIALIST',
 'You are the MCP Configurator specialist on AgentHub. You help users configure MCP (Model Context Protocol) server connections that expose external tools and data sources to agents. Guide users through: server name, transport type (stdio or HTTP), connection URL, authentication settings, and which agents should use the server. Validate configurations before saving.'),

('d1000000-0000-0000-0001-000000000009'::uuid, 'API Importer', 'core-api-importer',
 'Specialist for importing OpenAPI specifications and generating tools automatically.',
 'SPECIALIST',
 'You are the API Importer specialist on AgentHub. You help users import external APIs by analyzing OpenAPI/Swagger specifications and automatically generating HTTP tools for each endpoint. Ask the user for the API base URL and authentication requirements, then create the tools and group them into a skill. Review the generated tools with the user before finalizing.'),

('d1000000-0000-0000-0001-000000000010'::uuid, 'Agents Specialist', 'core-agents-specialist',
 'Specialist for querying and managing the agent catalog.',
 'SPECIALIST',
 'You are the Agents Specialist on AgentHub. You help users understand and manage their agent catalog. You can list all agents, explain what each agent does, compare agent capabilities, help clone or modify agents, and manage agent lifecycle (draft → published → archived). Use the readiness score to help users identify agents that need improvement before publishing.'),

('d1000000-0000-0000-0001-000000000011'::uuid, 'Pipeline Specialist', 'core-pipeline-specialist',
 'Read-only specialist for inspecting legacy pipelines (deprecated since 2026-04-02).',
 'SPECIALIST',
 'You are the Pipeline Specialist on AgentHub. Pipelines are deprecated in favour of the agentic execution loop (ADR-012); your role is read-only. You help users list and inspect existing pipelines, trace their nodes/edges, and explain how to migrate them to agent + skill bindings. Never propose creating new pipelines — always recommend the agentic alternative.'),

('d1000000-0000-0000-0001-000000000012'::uuid, 'Execution Specialist', 'core-execution-specialist',
 'Specialist for analyzing execution history and debugging agent runs.',
 'SPECIALIST',
 'You are the Execution Specialist on AgentHub. You help users analyze agent execution history, understand what happened during a run, identify errors and bottlenecks, and debug failed executions. You can access execution logs, tool call results, and timing information. Provide clear explanations of what went wrong and suggest fixes.');

-- ============================
-- BIND SKILLS TO AGENTS
-- ============================

-- AgentHub Assistant: every platform skill
INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-assistant';

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-agent-builder'
   AND s.slug IN ('agent-management','skill-management','settings-management');

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-tool-builder'
   AND s.slug IN ('tool-management','skill-management');

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-skills-specialist'
   AND s.slug IN ('skill-management','tool-management');

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-tools-specialist'
   AND s.slug = 'tool-management';

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-kb-builder'
   AND s.slug IN ('knowledge-base-management','agent-management');

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-kb-specialist'
   AND s.slug = 'knowledge-base-management';

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-mcp-configurator'
   AND s.slug = 'mcp-management';

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-api-importer'
   AND s.slug IN ('tool-management','skill-management','agent-management');

INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-agents-specialist'
   AND s.slug = 'agent-management';

-- Pipeline Specialist: no management skill (read-only via REST)
INSERT INTO ah_core.agent_skill (agent_id, skill_id)
SELECT a.id, s.id FROM ah_core.agent a, ah_core.skill s
 WHERE a.slug = 'core-execution-specialist'
   AND s.slug = 'execute-sql';
