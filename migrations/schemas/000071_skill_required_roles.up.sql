ALTER TABLE skill
    ADD COLUMN IF NOT EXISTS required_roles TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN skill.required_roles IS 'Realm roles required to expose this skill to the LLM tool schema. Empty means unrestricted.';
