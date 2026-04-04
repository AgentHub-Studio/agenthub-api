-- Rollback migration 000023

-- Remove skill_tool bindings for new tools
DELETE FROM skill_tool
WHERE tool_id IN (
    'c1000000-0000-0000-0000-000000000065',
    'c1000000-0000-0000-0000-000000000066',
    'c1000000-0000-0000-0000-000000000067',
    'c1000000-0000-0000-0000-000000000068',
    'c1000000-0000-0000-0000-000000000069',
    'c1000000-0000-0000-0000-000000000070'
);

-- Remove new tools
DELETE FROM tool
WHERE id IN (
    'c1000000-0000-0000-0000-000000000065',
    'c1000000-0000-0000-0000-000000000066',
    'c1000000-0000-0000-0000-000000000067',
    'c1000000-0000-0000-0000-000000000068',
    'c1000000-0000-0000-0000-000000000069',
    'c1000000-0000-0000-0000-000000000070'
);

-- Remove injected memory-eval-prompt template (if inserted by ON CONFLICT path)
DELETE FROM prompt_template
WHERE id = 'f1000000-0000-0000-0001-000000000002';

-- Revert curate-memory skill description
UPDATE skill
SET description = 'Reviews and curates an agent''s long-term memory store. Finds duplicate or conflicting memories, removes outdated entries, and organizes memory by relevance. Use when the user wants to clean up what the agent has learned or verify its memory contents. Parameters: ''agent_id'' (target agent UUID), ''action'' (optional: ''audit'', ''deduplicate'', ''clean'', ''list''). Returns a structured memory report with counts and recommended operations.',
    updated_at = NOW()
WHERE slug = 'curate-memory';
