CREATE TABLE IF NOT EXISTS ah_core.tool_pool_assembly_step_template (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                 VARCHAR(64)  NOT NULL UNIQUE,
    label                VARCHAR(128) NOT NULL,
    description          TEXT         NOT NULL,
    step_order           INTEGER      NOT NULL CHECK (step_order >= 1 AND step_order <= 5),
    is_always_active     BOOLEAN      NOT NULL DEFAULT true,
    can_filter_tools     BOOLEAN      NOT NULL DEFAULT false,
    always_precedes_mcp  BOOLEAN      NOT NULL DEFAULT false,
    sort_order           INTEGER      NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- §6.2: five-step tool pool assembly pipeline executed at the start of each turn.
-- step_order: canonical position 1..5
-- is_always_active: false = skipped when the relevant source is absent (e.g. no MCP servers)
-- can_filter_tools: true = step may remove tools from the pool (filtering steps)
-- always_precedes_mcp: true = step is guaranteed to finish before MCP tools are merged
INSERT INTO ah_core.tool_pool_assembly_step_template
    (slug, label, description, step_order,
     is_always_active, can_filter_tools, always_precedes_mcp, sort_order)
VALUES
    ('base_tool_enumeration',
     'Base Tool Enumeration',
     'Collects the 19 core built-in tools plus up to 35 optional built-in tools from the global registry, yielding up to 54 initial candidates. Always runs unconditionally.',
     1, true, false, true, 0),

    ('mode_filtering',
     'Mode Filtering',
     'Removes tools not applicable to the current operating mode (e.g., worktree tools excluded in non-CLI/web environments). Always active; determines which built-ins survive to the MCP merge.',
     2, true, true, true, 1),

    ('deny_rule_prefiltering',
     'Deny Rule Pre-Filtering',
     'Applies permission-layer deny rules to the built-in pool before MCP tools are merged. Prevents denied built-ins from polluting the deduplication namespace. Always runs even when no MCP servers are present.',
     3, true, true, true, 2),

    ('mcp_tool_integration',
     'MCP Tool Integration',
     'Merges tools advertised by all enabled and connected MCP servers into the filtered built-in pool. Conditional: skipped entirely when no MCP servers are configured or all servers are offline.',
     4, false, false, false, 3),

    ('deduplication',
     'Deduplication',
     'Enforces name-uniqueness across all sources using the deterministic precedence ladder: builtin > skill > mcp > subagent > extension. The last step; always runs to produce the final immutable pool snapshot.',
     5, true, true, false, 4)
ON CONFLICT (slug) DO NOTHING;
