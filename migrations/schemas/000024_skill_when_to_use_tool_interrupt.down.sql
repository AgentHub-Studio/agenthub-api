-- Down migration for 000024: remove when_to_use/argument_hint from skill,
-- remove interrupt_behavior/is_search_or_read from tool.

DROP INDEX IF EXISTS idx_tool_is_search_or_read;
DROP INDEX IF EXISTS idx_skill_when_to_use_exists;

ALTER TABLE tool
    DROP COLUMN IF EXISTS is_search_or_read,
    DROP COLUMN IF EXISTS interrupt_behavior;

ALTER TABLE skill
    DROP COLUMN IF EXISTS argument_hint,
    DROP COLUMN IF EXISTS when_to_use;
