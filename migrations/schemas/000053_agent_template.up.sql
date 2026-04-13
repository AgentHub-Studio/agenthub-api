-- Creates the agent_template table for pre-built agent configurations.
-- Templates provide ready-to-use agent definitions (system prompt, model config,
-- skill slugs) that tenants can instantiate into full agents with one call.
-- is_builtin=true templates are seeded by the platform; false = tenant-created.
CREATE TABLE IF NOT EXISTS agent_template (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    description     TEXT,
    category        VARCHAR(100),   -- e.g. 'rag', 'approval', 'data', 'support', 'coding'
    is_builtin      BOOLEAN      NOT NULL DEFAULT false,
    -- definition_json stores the full agent bundle:
    -- { "systemPrompt": "...", "modelConfig": {...}, "skills": ["slug1", "slug2"], "permissionRules": {...} }
    definition_json JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_template_category ON agent_template (category);
CREATE INDEX IF NOT EXISTS idx_agent_template_builtin  ON agent_template (is_builtin);

-- Seed 5 built-in templates covering the most common agent patterns.

INSERT INTO agent_template (name, slug, description, category, is_builtin, definition_json) VALUES
(
    'RAG Knowledge Assistant',
    'rag-knowledge-assistant',
    'An agent that searches knowledge bases and answers questions based on retrieved documents. Wire it to your knowledge bases for instant Q&A.',
    'rag',
    true,
    '{
        "systemPrompt": "You are a knowledgeable assistant. Use the document-search tool to find relevant information in the knowledge base before answering. Always cite the documents you referenced.\n\nInstructions:\n1. Search the knowledge base for relevant context\n2. Synthesize the results into a clear, accurate answer\n3. If no relevant documents are found, say so explicitly",
        "modelConfig": {"provider": "", "model": ""},
        "skills": ["document-search"],
        "permissionRules": {"allow": ["document-search"], "mode": "default"}
    }'::jsonb
),
(
    'SQL Data Analyst',
    'sql-data-analyst',
    'An agent that translates natural language questions into SQL queries and returns structured results. Requires a SQL skill bound to your database.',
    'data',
    true,
    '{
        "systemPrompt": "You are a data analyst assistant. Translate user questions into SQL queries and execute them using the sql-query tool. Always validate the query before executing.\n\nGuidelines:\n- Use SELECT only; never modify data without explicit user confirmation\n- Explain the query and results in plain language\n- Suggest follow-up analyses when useful",
        "modelConfig": {"provider": "", "model": ""},
        "skills": ["sql-query"],
        "permissionRules": {
            "allow": ["sql-query(SELECT)"],
            "confirm": ["sql-query(UPDATE)", "sql-query(DELETE)", "sql-query(DROP)"],
            "mode": "default"
        }
    }'::jsonb
),
(
    'Approval Workflow Agent',
    'approval-workflow-agent',
    'An agent that handles multi-step approval flows. Collects structured input, routes for human review, and executes follow-up actions on approval.',
    'approval',
    true,
    '{
        "systemPrompt": "You are an approval workflow coordinator. Your job is to:\n1. Collect the required information from the requester\n2. Present a clear summary for the approver\n3. Wait for explicit approval or rejection\n4. Execute the approved action or notify of rejection\n\nAlways be transparent about what action you are about to take before taking it.",
        "modelConfig": {"provider": "", "model": ""},
        "skills": [],
        "permissionRules": {
            "confirm": ["*"],
            "mode": "default"
        }
    }'::jsonb
),
(
    'Customer Support Agent',
    'customer-support-agent',
    'A friendly customer support agent that searches your knowledge base for answers and escalates when it cannot resolve the issue.',
    'support',
    true,
    '{
        "systemPrompt": "You are a helpful customer support agent. Your goals:\n1. Understand the customer''s issue completely\n2. Search the knowledge base for relevant solutions\n3. Provide clear, step-by-step guidance\n4. If the issue cannot be resolved, collect details and escalate\n\nTone: friendly, patient, and professional. Never make up information — if unsure, say so.",
        "modelConfig": {"provider": "", "model": ""},
        "skills": ["document-search"],
        "permissionRules": {"allow": ["document-search"], "mode": "default"}
    }'::jsonb
),
(
    'HTTP API Integration Agent',
    'http-api-integration-agent',
    'An agent that calls external HTTP APIs to retrieve or post data. Requires an HTTP tool configured with the target API.',
    'integration',
    true,
    '{
        "systemPrompt": "You are an API integration agent. You call external HTTP APIs to retrieve or submit data on behalf of the user.\n\nSecurity rules:\n- Only call APIs explicitly listed in your tools\n- Never include credentials in visible output\n- Always confirm with the user before POST/PUT/DELETE operations\n- Report errors clearly and suggest corrective actions",
        "modelConfig": {"provider": "", "model": ""},
        "skills": ["http-request"],
        "permissionRules": {
            "allow": ["http-request(GET)"],
            "confirm": ["http-request(POST)", "http-request(PUT)", "http-request(DELETE)"],
            "mode": "default"
        }
    }'::jsonb
)
ON CONFLICT (slug) DO NOTHING;
