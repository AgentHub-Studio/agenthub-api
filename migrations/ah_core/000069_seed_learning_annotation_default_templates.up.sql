-- HUMAN-007 paired seed: starter learning annotation templates.
-- These 6 templates represent teachable moments for common AgentHub
-- operator workflows, covering each kind once (pattern, anti_pattern,
-- tip, optimization, knowledge_gap) with audience-appropriate tagging.
--
-- Idempotent: ON CONFLICT (slug) DO NOTHING.

CREATE TABLE IF NOT EXISTS ah_core.learning_annotation_template (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                 TEXT        UNIQUE NOT NULL,
    kind                 TEXT        NOT NULL
                             CHECK (kind IN ('pattern','anti_pattern','tip','optimization','knowledge_gap')),
    audience             TEXT        NOT NULL
                             CHECK (audience IN ('beginner','intermediate','advanced')),
    title                TEXT        NOT NULL,
    content              TEXT        NOT NULL,
    related_feature_slugs JSONB      NOT NULL DEFAULT '[]',
    sort_order           INTEGER     NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ah_core.learning_annotation_template
    (id, slug, kind, audience, title, content, related_feature_slugs, sort_order)
VALUES
    (
        'bbbbbbbb-0001-0000-0000-000000000001',
        'agent-tool-binding-pattern',
        'pattern',
        'beginner',
        'Linking a Tool to an Agent',
        'Binding a concrete tool to an agent via a skill is the fundamental composition unit in AgentHub. Create a skill, attach one or more tools, then add the skill to your agent — the LLM will receive the tool schema automatically.',
        '["agent","skill","tool"]',
        10
    ),
    (
        'bbbbbbbb-0002-0000-0000-000000000002',
        'no-system-prompt-antipattern',
        'anti_pattern',
        'beginner',
        'Agent Without a System Prompt',
        'Publishing an agent without a system_prompt results in generic, unfocused behavior. Even a two-sentence prompt describing the agent''s role and constraints dramatically improves result quality and reduces hallucinations.',
        '["agent","system-prompt"]',
        20
    ),
    (
        'bbbbbbbb-0003-0000-0000-000000000003',
        'skill-reuse-tip',
        'tip',
        'intermediate',
        'Reuse Skills Across Multiple Agents',
        'A skill is a reusable capability, not an agent-specific resource. Once configured and tested, bind the same skill to multiple agents instead of duplicating tool configurations. This keeps changes consistent and reduces maintenance surface.',
        '["skill","agent","tool"]',
        30
    ),
    (
        'bbbbbbbb-0004-0000-0000-000000000004',
        'context-budget-optimization',
        'optimization',
        'intermediate',
        'Trim Context Budget to Reduce LLM Cost',
        'Each agent run injects the full context window into the LLM call. Review context_section_budget settings and disable sections you do not need (e.g., tool_result_budget, memory_include_directives). Smaller context = fewer tokens = lower cost per run.',
        '["context-budget","memory-hierarchy","tool-result-budget"]',
        40
    ),
    (
        'bbbbbbbb-0005-0000-0000-000000000005',
        'knowledge-base-chunking-gap',
        'knowledge_gap',
        'intermediate',
        'Knowledge Base Chunking Strategy',
        'AgentHub chunks documents before embedding. The default strategy is semantic (paragraph boundaries). For dense technical PDFs consider a smaller fixed-size chunk with 15% overlap to preserve context across section boundaries. Configure via knowledge_base.chunk_strategy.',
        '["knowledge-base","chunking","rag"]',
        50
    ),
    (
        'bbbbbbbb-0006-0000-0000-000000000006',
        'escalation-workflow-pattern',
        'pattern',
        'advanced',
        'Escalation Workflows for High-Risk Operations',
        'Pair a PolicyEvaluatorRule (decision=escalate) with a PermissionHook that routes to a human-approval queue. The agent blocks, sends an audit event, and resumes only after the operator approves. This gives you fine-grained control over destructive or irreversible tool calls without disabling automation entirely.',
        '["policy-evaluator","permission-hook","escalation"]',
        60
    )
ON CONFLICT (slug) DO NOTHING;
