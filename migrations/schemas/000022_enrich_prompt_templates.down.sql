-- Reverse migration 000022.

ALTER TABLE prompt_template DROP COLUMN IF EXISTS is_builtin;

DELETE FROM prompt_template WHERE id IN (
    'e1000000-0000-0000-0001-000000000006',
    'e1000000-0000-0000-0001-000000000007'
);

-- Note: content updates to existing templates are not reversed.
-- The abbreviated versions from 000013 remain valid but less detailed.
