-- CTX-007-paired: memory include directive default templates.
-- 6 starter @include{key} entries that fresh tenants can reference from
-- their memory facts. CTX-007 MemoryIncludeResolver expands these
-- recursively at run time (cycle-detected, max-depth-guarded).
--
-- Key format constraint: matches CTX-007 regex
--   @include\{([a-zA-Z0-9._-]+)\}
-- so admin UI and seed authors must use only [A-Za-z0-9._-] in keys.

CREATE TABLE IF NOT EXISTS ah_core.memory_include_directive_default_template (
    id                  UUID PRIMARY KEY,
    slug                TEXT NOT NULL UNIQUE,
    include_key         TEXT NOT NULL UNIQUE, -- must match CTX-007 regex
    content_template    TEXT NOT NULL,
    category            TEXT NOT NULL,
    description         TEXT NOT NULL,
    contains_placeholders BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended      BOOLEAN NOT NULL DEFAULT TRUE,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order          INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_memory_include_dt_category
    ON ah_core.memory_include_directive_default_template(category);
CREATE INDEX IF NOT EXISTS idx_memory_include_dt_active
    ON ah_core.memory_include_directive_default_template(is_active);

INSERT INTO ah_core.memory_include_directive_default_template
    (id, slug, include_key, content_template, category, description,
     contains_placeholders, is_recommended, is_active, sort_order)
VALUES
    ('00000007-0000-0000-0000-000000000001',
     'core-org-identity',
     'core.org.identity',
     'Organization: {{tenant.org_name}}. Default timezone: {{tenant.timezone}}. Primary contact: {{tenant.admin_email}}.',
     'organization',
     'Org identity preamble injected at the top of any agent prompt that needs tenant-specific framing. Placeholders are resolved before include expansion.',
     TRUE, TRUE, TRUE, 10),

    ('00000007-0000-0000-0000-000000000002',
     'core-cite-format',
     'core.cite.format',
     'When citing sources, use the format: [Title — file_path or URL, accessed YYYY-MM-DD]. Inline citations are preferred over footnotes.',
     'citation',
     'Citation style guide for research and review subagents. Plain text, no placeholders.',
     FALSE, TRUE, TRUE, 20),

    ('00000007-0000-0000-0000-000000000003',
     'core-code-style-preamble',
     'core.code.style.preamble',
     'Coding standards: keep edits surgical and scoped to the parent task; do not introduce abstractions without need; tests are mandatory for new public APIs; document only WHY when non-obvious.',
     'code_style',
     'Coding style preamble for coder subagents. References AgentHub project standards. Plain text.',
     FALSE, TRUE, TRUE, 30),

    ('00000007-0000-0000-0000-000000000004',
     'core-safety-do-not',
     'core.safety.do_not',
     'Never execute destructive shell commands (rm -rf, drop database, force-push to main) without explicit user authorization. Never bypass pre-commit hooks. Never commit secrets or credentials.',
     'safety',
     'Safety guard rail injected as a constant tail-of-prompt to remind agents of immutable do-nots.',
     FALSE, TRUE, TRUE, 40),

    ('00000007-0000-0000-0000-000000000005',
     'core-locale-pt-br-summary',
     'core.locale.pt_br.summary',
     'Comunicação com o usuário, documentação e mensagens de commit DEVEM ser em português brasileiro. Código-fonte (variáveis, funções, tipos, comentários inline, GoDoc) DEVE estar em inglês.',
     'locale',
     'PT-BR vs EN policy summary aligned with AgentHub CLAUDE.md. Plain text.',
     FALSE, TRUE, TRUE, 50),

    ('00000007-0000-0000-0000-000000000006',
     'core-runner-handoff',
     'core.runner.handoff',
     'When handing off to another subagent, include: the parent goal, what was tried, what worked, what failed, and a single concrete next action. Cite file paths for any artifacts referenced.',
     'handoff',
     'Subagent-to-subagent handoff format for multi-agent coordination (SUB-011). Plain text.',
     FALSE, TRUE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
