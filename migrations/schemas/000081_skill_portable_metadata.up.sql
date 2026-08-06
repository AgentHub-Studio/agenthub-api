ALTER TABLE skill
    ADD COLUMN IF NOT EXISTS model_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS effort_level VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS associated_agents TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS dynamic_hooks TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE skill
    DROP CONSTRAINT IF EXISTS chk_skill_effort_level;

ALTER TABLE skill
    ADD CONSTRAINT chk_skill_effort_level
    CHECK (effort_level IN ('', 'lowest', 'low', 'medium', 'high', 'highest'));
