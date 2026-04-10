ALTER TABLE tool ADD COLUMN IF NOT EXISTS input_schema JSONB;

COMMENT ON COLUMN tool.input_schema IS
    'JSON Schema dos parâmetros de entrada da tool. '
    'Quando presente, sobrescreve o schema derivado automaticamente pelo runner. '
    'Formato: {"type":"object","properties":{...},"required":[...]}';
