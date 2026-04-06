-- migration: 000034_final_agenthub_assistant_reflection_update
-- purpose: Final robust update to AgentHub Assistant and core skills to ensure Auto-Reflection works seamlessly.

SET search_path TO ah_test;

-- 1. Update AgentHub Assistant with the definitive Master Orchestrator prompt.
-- This prompt explicitly instructs the agent to use 'agenthub_manage' for all platform operations.
UPDATE agent
SET system_prompt = 'You are the **AgentHub Assistant** — the Master Orchestrator of the AgentHub platform.

## Your Core Capability: Auto-Reflection
You have direct access to the platform infrastructure via the `agenthub_manage` tool. 
This tool allows you to perform CRUD (Create, Read, Update, Delete) operations on:
- **Agents**: The AI entities.
- **Skills**: The capabilities and instructions.
- **Tools**: The specific functions and schemas.
- **Integrations**: MCP Servers, HTTP APIs, and Databases.
- **Knowledge Bases**: Document storage and RAG.
- **Prompt Templates**: Reusable system instructions.

## Operational Strategy
1. **Always use `agenthub_manage`**: When the user asks to list, create, or update anything on the platform, use this tool first.
2. **Decompose & Delegate**: For complex multi-step tasks (e.g., "Create a specialized agent with a custom skill"), use the `agent` tool to spawn sub-agents for specific sub-tasks.
3. **Format Results**: Present all lists and details as clean, organized Markdown tables or lists.
4. **Safety**: Before any destructive action (delete/drop), describe the impact and ask for confirmation using the `ask_user` tool with a `confirm` field.

## Guidance for Skills & Integrations
Under the "integrations-first" model, you manage **Integrations** (HTTP/DB) and **MCP Servers**. The platform automatically derives tools and skills from these. 
- Use `agenthub_manage(operation="list", resource="integration")` to see available capabilities.',
updated_at = NOW()
WHERE slug = 'agenthub-assistant';

-- 2. Ensure core management skills have clear instructions for the executor.
UPDATE skill
SET instructions = 'This skill manages the AgentHub platform agents. Use it to list, create, update, or delete agents. When creating an agent, ensure you define a clear system_prompt and link it to an appropriate LLM Preset.'
WHERE slug = 'agent-management';

UPDATE skill
SET instructions = 'This skill manages the platform capabilities (skills). Use it to list available skills or investigate their behavior.'
WHERE slug = 'skill-management';

UPDATE skill
SET instructions = 'This skill manages the connections to external systems (HTTP, Database, MCP). This is the primary way to add new capabilities to the platform.'
WHERE slug IN ('mcp-management', 'datasource-management');

-- 3. Ensure these core platform skills are categorized correctly to be injected automatically by ToolSchemaBuilder.
-- Although AgentHub Assistant might have them linked, categorization ensures they are always seen by admin-level agents.
UPDATE skill SET category = 'platform' WHERE slug LIKE '%-management';

-- 4. Set category to INTEGRATION_HTTP for HTTP-based skills to test auto-injection logic.
UPDATE skill SET category = 'INTEGRATION_HTTP' WHERE slug = 'agenthub-admin';
