-- Remove the seed only if it is still the untouched default empty string.
DELETE FROM settings WHERE key = 'system.prompt' AND value = '""'::jsonb;
