-- Seed the `system.prompt` settings key with an empty default so the admin
-- screen at /system-prompt does not 404 on first access. The screen reads
-- from GET /api/settings/system.prompt; without this row the handler returns
-- 404 and the frontend global error interceptor surfaces a toast, even
-- though the service's own catchError silently falls back to empty.
--
-- Safe to re-run (ON CONFLICT DO NOTHING).

INSERT INTO settings (key, value, description) VALUES
    ('system.prompt', '""'::jsonb, 'Global system prompt applied to all agent conversations (empty means no global override).')
ON CONFLICT (key) DO NOTHING;
