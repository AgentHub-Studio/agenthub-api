CREATE TABLE IF NOT EXISTS ah_core.coding_agent_category_template (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                  VARCHAR(64)  NOT NULL UNIQUE,
    label                 VARCHAR(128) NOT NULL,
    description           TEXT         NOT NULL,
    gradient_index        INTEGER      NOT NULL CHECK (gradient_index >= 0 AND gradient_index <= 3),
    execution_pattern     VARCHAR(64)  NOT NULL,
    isolation_model       VARCHAR(64)  NOT NULL,
    example_systems       JSONB        NOT NULL DEFAULT '[]',
    is_agenthub_target    BOOLEAN      NOT NULL DEFAULT false,
    recommended_for       JSONB        NOT NULL DEFAULT '[]',
    sort_order            INTEGER      NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- §13.1 Table 5: four AI coding tool categories ordered by degree of autonomous action.
-- gradient_index: 0=passive (inline completion) → 3=autonomous (fully autonomous)
-- AgentHub targets gradient_index 1 (chat_integrated) and 2 (agentic_cli).
INSERT INTO ah_core.coding_agent_category_template
    (slug, label, description, gradient_index,
     execution_pattern, isolation_model, example_systems,
     is_agenthub_target, recommended_for, sort_order)
VALUES
    ('inline_completion',
     'Inline Completion',
     'Suggests code fragments within the editor without autonomous action. No tool-use loop; purely reactive to cursor position and surrounding context.',
     0,
     'editor_plugin',
     'none',
     '["GitHub Copilot","Tabnine"]',
     false,
     '["editor integrations","low-autonomy suggestions","code completion plugins"]',
     0),

    ('chat_integrated',
     'Chat-Integrated',
     'Adds conversational interaction and multi-file edits while remaining coupled to an IDE or web interface. Primary deployment model for AgentHub.',
     1,
     'ide_coupled_product',
     'ide_environment',
     '["Cursor","Windsurf","Cody","AgentHub"]',
     true,
     '["web agent chat","interactive multi-turn workflows","AgentHub standard agents"]',
     1),

    ('agentic_cli',
     'Agentic CLI',
     'Operates from the command line with autonomous tool-use loops and a layered permission system. Background agents in AgentHub use this pattern.',
     2,
     'tool_use_loop',
     'permission_gates',
     '["Claude Code","Codex CLI","Aider"]',
     true,
     '["autonomous background agents","KAIROS heartbeat agents","agentic_cli tier agents","pipeline automation"]',
     2),

    ('fully_autonomous',
     'Fully Autonomous',
     'Aims for minimal human supervision in sandboxed cloud environments with planning layers. Not the current AgentHub model; aspirational for future sandbox execution.',
     3,
     'sandbox_planning',
     'container_sandbox',
     '["Devin","SWE-Agent","OpenHands"]',
     false,
     '["fully sandboxed execution","long-horizon autonomous research","future AgentHub enterprise tier"]',
     3)
ON CONFLICT (slug) DO NOTHING;
