-- Revert migration 000027

DROP INDEX IF EXISTS idx_tool_strict;
DROP INDEX IF EXISTS idx_skill_effort;

UPDATE tool SET is_open_world = FALSE, strict = FALSE;
DELETE FROM skill_tool WHERE tool_id = 'c1000000-0000-0000-0000-000000000089';
DELETE FROM tool WHERE id = 'c1000000-0000-0000-0000-000000000089';

ALTER TABLE tool DROP COLUMN IF EXISTS strict;
ALTER TABLE tool DROP COLUMN IF EXISTS is_open_world;

ALTER TABLE agent_hook
    DROP COLUMN IF EXISTS shell,
    DROP COLUMN IF EXISTS if_condition;

ALTER TABLE skill
    DROP COLUMN IF EXISTS loaded_from,
    DROP COLUMN IF EXISTS model_override,
    DROP COLUMN IF EXISTS effort;
