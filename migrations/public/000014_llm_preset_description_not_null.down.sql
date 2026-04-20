ALTER TABLE public.llm_config_preset
    ALTER COLUMN description DROP DEFAULT,
    ALTER COLUMN description DROP NOT NULL;
