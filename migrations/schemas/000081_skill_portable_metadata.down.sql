ALTER TABLE skill
    DROP CONSTRAINT IF EXISTS chk_skill_effort_level;

ALTER TABLE skill
    DROP COLUMN IF EXISTS dynamic_hooks,
    DROP COLUMN IF EXISTS associated_agents,
    DROP COLUMN IF EXISTS effort_level,
    DROP COLUMN IF EXISTS model_overrides;
