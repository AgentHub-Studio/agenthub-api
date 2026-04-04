-- Reverse migration 000021.

ALTER TABLE tool DROP COLUMN IF EXISTS concurrency_safe;
ALTER TABLE tool DROP COLUMN IF EXISTS max_result_chars;
ALTER TABLE skill DROP COLUMN IF EXISTS context_mode;
