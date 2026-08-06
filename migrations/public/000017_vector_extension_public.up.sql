DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_extension e
        JOIN pg_namespace n ON n.oid = e.extnamespace
        WHERE e.extname = 'vector' AND n.nspname <> 'public'
    ) THEN
        ALTER EXTENSION vector SET SCHEMA public;
    ELSE
        CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;
    END IF;
END $$;
