-- Reverse the provider seed from the up-migration.
--
-- The default agent's slug is NOT reverted to 'meu-assistente': renaming back
-- would be fragile (there is no way to distinguish the auto-seeded agent from a
-- user-created agent that happens to use that slug), and the up-migration is
-- idempotent so re-applying it is safe. This mirrors the philosophy of
-- 000063.down, which also leaves the seeded agent untouched on rollback.

DELETE FROM settings WHERE key = 'llm.defaultProvider' AND value = '"openrouter"'::jsonb;
DELETE FROM settings WHERE key = 'openrouter.model' AND value = '"mistralai/mistral-nemo"'::jsonb;
