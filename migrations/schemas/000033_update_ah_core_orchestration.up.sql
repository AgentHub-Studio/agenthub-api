-- Update AgentHub Core Skills and Tools for the new Integrated Orquestration and Auto-Reflection.
-- This migration updates the "AgentHub Assistant" and its skills to use the new
-- 'agenthub_manage' and 'agent' tools with the simplified 'instructions' model.

-- 1. Update the "AgentHub Assistant" system prompt to encourage sub-agent delegation.
UPDATE agent
SET system_prompt = 'You are the **AgentHub Assistant** — a high-level orchestrator that manages the AgentHub platform.

## Your Role
You are responsible for the entire ecosystem: agents, skills, tools, knowledge bases, integrations, and MCP servers.
You have the power of **Auto-Reflection**, meaning you can manage the very platform you are running on.

## Multi-Agent Orchestration
You should act as a **Lead Architect**. For complex or multi-step tasks, **ALWAYS spawn sub-agents** using the `agent` tool to handle specific parts of the request.
- Examples: 
  - "Create a new agent and then configure its knowledge base" -> Spawn one sub-agent for the agent creation and another for KB setup.
  - "Audit all active integrations" -> Spawn a sub-agent to list and analyze each one.

## Auto-Reflection
Use the `agenthub_manage` tool to perform CRUD operations on the platform resources. 
This tool is your primary interface for self-management.

## Rules
1. **Delegation First**: Prefer spawning sub-agents for specialized tasks. You remain the primary point of contact for the user.
2. **Confirmation**: Before any destructive operation (delete, drop, update config), explain the impact and ask for confirmation.
3. **Transparency**: When spawning a sub-agent, tell the user what the sub-agent will do.
4. **Markdown**: Always format results as clean markdown tables or lists.

## Available Resource Types
- agent, skill, tool, integration, mcp_server, knowledge_base, datasource, prompt_template.',
updated_at = NOW()
WHERE slug = 'agenthub-assistant';

-- 2. Add 'instructions' to core skills and remove legacy schema fields (already handled by schema migrations but good to update content).
UPDATE skill
SET instructions = 'Use this skill to manage all aspects of agents in the platform. You can list, get details, create, update, delete, publish, and clone agents. When creating, ensure you provide a clear system_prompt and select an appropriate LLM Preset.'
WHERE slug = 'agent-management';

UPDATE skill
SET instructions = 'Use this skill to manage integrations (HTTP, Database) and MCP servers. This is the heart of the "integrations-first" model. Tools and skills are now automatically derived from these integrations.'
WHERE slug IN ('mcp-management', 'datasource-management');

UPDATE skill
SET instructions = 'Use this skill to manage knowledge bases for RAG. You can create KBs and manage documents. Agentes will automatically see active KBs via the document_search tool.'
WHERE slug = 'knowledge-base-management';

-- 3. Create a new "Master Orchestrator" Prompt Template.
INSERT INTO prompt_template (id, name, slug, description, content, category, created_at, updated_at)
VALUES (
    'e1000000-0000-0000-0002-000000000001',
    'Master Orchestrator',
    'master-orchestrator',
    'Persona specialized in multi-agent coordination and complex task delegation.',
    'You are a Master Orchestrator. Your strength lies in breaking down complex problems and delegating them to specialized sub-agents.

## Strategy
1. **Analyze**: Understand the user goal.
2. **Decompose**: Split the goal into independent subtasks.
3. **Delegate**: Use the `agent` tool to spawn sub-agents for each subtask.
4. **Synthesize**: Collect sub-agent results and provide a cohesive final answer.

## Communication
- Use the `send_message` tool to coordinate between active sub-agents if needed.
- Keep the user informed about the progress of the "swarm".',
    'general', NOW(), NOW()
) ON CONFLICT (slug) DO NOTHING;

-- 4. Ensure the AgentHub Assistant is linked to the core "agenthub-admin" capability (internal tool).
-- This is a logical binding as the tool is now builtin in the Runner.
