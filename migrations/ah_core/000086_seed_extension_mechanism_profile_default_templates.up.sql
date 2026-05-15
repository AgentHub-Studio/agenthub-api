CREATE TABLE IF NOT EXISTS ah_core.extension_mechanism_profile_template (
    id                       UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                     VARCHAR(32)  NOT NULL UNIQUE,
    label                    VARCHAR(64)  NOT NULL,
    description              TEXT         NOT NULL,
    unique_capability        TEXT         NOT NULL,
    context_cost_category    VARCHAR(16)  NOT NULL CHECK (context_cost_category IN ('micro','small','medium','large','heavy')),
    insertion_point          VARCHAR(16)  NOT NULL CHECK (insertion_point IN ('assemble','model','execute','all')),
    is_zero_cost_by_default  BOOLEAN      NOT NULL DEFAULT false,
    covers_all_insert_points BOOLEAN      NOT NULL DEFAULT false,
    sort_order               INTEGER      NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Table 2 from arXiv:2604.14228v1: four extension mechanisms ordered by context cost.
-- context_cost_category: micro=zero|small=low|medium=varies|large=high (per §6.3 ordering)
-- insertion_point: which of the three agent-loop phases (Figure 5: assemble/model/execute/all)
-- is_zero_cost_by_default: hooks only — context cost only incurred when injection is opted-in
-- covers_all_insert_points: plugins only — apply at all three loop phases
INSERT INTO ah_core.extension_mechanism_profile_template
    (slug, label, description, unique_capability,
     context_cost_category, insertion_point,
     is_zero_cost_by_default, covers_all_insert_points, sort_order)
VALUES
    ('hooks',
     'Hooks',
     'Lifecycle interception and event-driven automation. Hooks intercept the tool execution lifecycle (pre/post tool, permission, session events). Zero context cost by default — hooks can opt-in to context injection but are never required to.',
     'Lifecycle interception and event-driven automation',
     'micro', 'execute', true, false, 0),

    ('skills',
     'Skills',
     'Domain-specific instructions and meta-tool invocation. Each skill injects its frontmatter description into the context window via the SkillTool meta-tool. Low context cost: only descriptions are loaded, not full content.',
     'Domain-specific instructions and meta-tool invocation',
     'small', 'assemble', false, false, 1),

    ('plugins',
     'Plugins',
     'Multi-component packaging and distribution. Plugins bundle any combination of commands, agents, skills, hooks, MCP servers, output styles, settings, and user configuration into a single installable package. Medium context cost (varies by included components). Unique in applying at all three agent-loop injection points.',
     'Multi-component packaging and distribution',
     'medium', 'all', false, true, 2),

    ('mcp_servers',
     'MCP Servers',
     'External service integration with multi-transport support (stdio, SSE, HTTP, WebSocket, SDK). Each connected MCP server contributes full tool schemas to the model''s tool pool. High context cost because tool schemas are structurally larger than skill descriptions.',
     'External service integration with multi-transport support',
     'large', 'model', false, false, 3)
ON CONFLICT (slug) DO NOTHING;
