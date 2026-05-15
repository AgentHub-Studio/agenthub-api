-- ah_core: seed context management approach default templates
-- Source: arXiv:2604.14228v1 Table 6 (§13.2 — Context Management Design Space)
-- Five strategies from coarse truncation to very-fine graduated compaction.
-- AgentHub uses graduated_compaction, adapted from Claude Code's §7.3 five-stage pipeline.

CREATE TABLE IF NOT EXISTS ah_core.context_management_approach_template (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                VARCHAR(64) NOT NULL UNIQUE,
    label               VARCHAR(128) NOT NULL,
    description         TEXT,
    mechanism           VARCHAR(128) NOT NULL,
    granularity         VARCHAR(16)  NOT NULL CHECK (granularity IN ('coarse','medium','fine','very_fine')),
    is_agenthub_approach BOOLEAN     NOT NULL DEFAULT FALSE,
    sort_order          INT         NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO ah_core.context_management_approach_template
    (slug, label, description, mechanism, granularity, is_agenthub_approach, sort_order)
VALUES
    (
        'simple_truncation',
        'Simple Truncation',
        'Drops the oldest messages when the context window fills. Straightforward but loses earlier context permanently.',
        'Drop oldest messages',
        'coarse',
        FALSE,
        10
    ),
    (
        'sliding_window',
        'Sliding Window',
        'Maintains a fixed-size window of the most recent conversation history, discarding older turns wholesale.',
        'Fixed-size recent history',
        'medium',
        FALSE,
        20
    ),
    (
        'rag',
        'Retrieval-Augmented Generation',
        'Retrieves relevant conversation snippets from a persistent store rather than holding all history in context.',
        'Retrieve relevant snippets',
        'fine',
        FALSE,
        30
    ),
    (
        'single_summarization',
        'Single Summarization',
        'Performs a one-pass LLM-generated summary of the conversation. Fast but lossy and non-reversible.',
        'One-pass compress',
        'coarse',
        FALSE,
        40
    ),
    (
        'graduated_compaction',
        'Graduated Compaction',
        'Multi-layer pipeline that escalates through progressively stronger compression strategies (§7.3). The approach used by Claude Code and AgentHub: budget reduction → snip → microcompact → context collapse → auto-compact.',
        'Multi-layer pipeline',
        'very_fine',
        TRUE,
        50
    )
ON CONFLICT (slug) DO NOTHING;
