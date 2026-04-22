-- Reverse the onboarding-flag seed. The "Meu Assistente" agent is NOT removed
-- on down-migrate because tenants may have customized its instructions; prefer
-- explicit deletion via admin if rollback is required.

DELETE FROM settings WHERE key = 'onboarding.completed';
DELETE FROM settings WHERE key = 'openai.model' AND value = '"gpt-4o-mini"'::jsonb;
