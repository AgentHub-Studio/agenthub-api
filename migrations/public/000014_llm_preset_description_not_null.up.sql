-- Make llm_config_preset.description NOT NULL with empty-string default.
-- The Go repository scans description into a non-nullable `string`, so any
-- NULL row makes the listing endpoint return 500:
--   "can't scan into dest[3] (col: description): cannot scan NULL into *string"

UPDATE public.llm_config_preset SET description = '' WHERE description IS NULL;
ALTER TABLE public.llm_config_preset
    ALTER COLUMN description SET DEFAULT '',
    ALTER COLUMN description SET NOT NULL;
