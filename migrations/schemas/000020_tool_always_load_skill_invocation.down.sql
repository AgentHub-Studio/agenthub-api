-- Reverse migration 000019.

ALTER TABLE tool DROP COLUMN IF EXISTS always_load;
ALTER TABLE skill DROP COLUMN IF EXISTS disable_model_invocation;
