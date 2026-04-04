-- Reverse migration 000016: remove deferred/destructive columns and revert read_only flags.

-- Revert read_only for tools that were missed in 000014.
UPDATE tool SET read_only = FALSE WHERE name IN (
    'agenthub_get_agent',
    'agenthub_get_skill',
    'agenthub_get_tool',
    'agenthub_get_knowledge_base',
    'agenthub_get_settings'
);

-- Drop new columns.
ALTER TABLE tool DROP COLUMN IF EXISTS search_hint;
ALTER TABLE tool DROP COLUMN IF EXISTS is_destructive;
ALTER TABLE tool DROP COLUMN IF EXISTS should_defer;
