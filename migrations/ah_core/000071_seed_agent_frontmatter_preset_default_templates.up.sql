-- EXT-006a paired seed: agent frontmatter YAML preset templates.
-- Six ready-to-use YAML presets covering common agent archetypes so fresh
-- tenants can bootstrap agent configs without writing frontmatter from scratch.
-- Each preset validates cleanly against AgentFrontmatterSchema (15-field schema).
--
-- Idempotent: ON CONFLICT (slug) DO NOTHING.

CREATE TABLE IF NOT EXISTS ah_core.agent_frontmatter_preset_template (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             TEXT        UNIQUE NOT NULL,
    label            TEXT        NOT NULL,
    description      TEXT        NOT NULL,
    frontmatter_yaml TEXT        NOT NULL,
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ah_core.agent_frontmatter_preset_template
    (id, slug, label, description, frontmatter_yaml, sort_order)
VALUES
    (
        'eeeeeeee-0001-0000-0000-000000000001',
        'fast-responder',
        'Fast Responder',
        'Low-latency preset for agents that must reply quickly: short output, low temperature, few turns.',
        E'---\ndescription: Fast-response agent optimized for low latency\nmodel: claude-haiku-4-5-20251001\ntemperature: "0.3"\nmax_tokens: "512"\nmax_turns: "3"\ntimeout: "30"\nenabled: "true"\ntags: fast,low-latency\npriority: "0"\n---',
        10
    ),
    (
        'eeeeeeee-0002-0000-0000-000000000002',
        'careful-analyst',
        'Careful Analyst',
        'Deliberate, low-temperature preset for analysis tasks requiring consistent, reproducible output.',
        E'---\ndescription: Careful analytical agent for consistent, reproducible reasoning\nmodel: claude-sonnet-4-6\ntemperature: "0.1"\nmax_tokens: "4096"\nmax_turns: "15"\ntimeout: "600"\ncontext_window_pct: "80"\nenabled: "true"\ntags: analysis,audit\npriority: "0"\n---',
        20
    ),
    (
        'eeeeeeee-0003-0000-0000-000000000003',
        'creative-writer',
        'Creative Writer',
        'High-temperature preset for creative writing tasks where diversity and novelty are desirable.',
        E'---\ndescription: Creative writing agent with high temperature for novelty\nmodel: claude-sonnet-4-6\ntemperature: "1.2"\nmax_tokens: "2048"\nmax_turns: "5"\ntimeout: "120"\nenabled: "true"\ntags: creative,writing\npriority: "0"\n---',
        30
    ),
    (
        'eeeeeeee-0004-0000-0000-000000000004',
        'security-auditor',
        'Security Auditor',
        'Zero-temperature preset for security review tasks that require deterministic, reproducible findings.',
        E'---\ndescription: Security audit agent — deterministic, zero-temperature\nmodel: claude-sonnet-4-6\ntemperature: "0.0"\nmax_tokens: "8192"\nmax_turns: "20"\ntimeout: "900"\ncontext_window_pct: "90"\nenabled: "true"\ntags: security,audit,compliance\npriority: "10"\n---',
        40
    ),
    (
        'eeeeeeee-0005-0000-0000-000000000005',
        'data-extractor',
        'Data Extractor',
        'Structured extraction preset with SQL and document-search tools, low temperature for accuracy.',
        E'---\ndescription: Data extraction agent with SQL and RAG capabilities\nmodel: claude-sonnet-4-6\ntemperature: "0.2"\nmax_tokens: "1024"\nmax_turns: "8"\ntimeout: "300"\ntools: sql,document_search\nenabled: "true"\ntags: data,extraction,sql\npriority: "0"\n---',
        50
    ),
    (
        'eeeeeeee-0006-0000-0000-000000000006',
        'research-assistant',
        'Research Assistant',
        'Balanced preset for research tasks: moderate temperature, many turns, knowledge-base access.',
        E'---\ndescription: Research assistant with knowledge base and web-search access\nmodel: claude-sonnet-4-6\ntemperature: "0.7"\nmax_tokens: "4096"\nmax_turns: "15"\ntimeout: "600"\ncontext_window_pct: "80"\ntools: document_search\nenabled: "true"\ntags: research,knowledge-base\npriority: "0"\n---',
        60
    )
ON CONFLICT (slug) DO NOTHING;
