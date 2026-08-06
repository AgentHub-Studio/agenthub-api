ALTER TABLE agent
  ADD COLUMN IF NOT EXISTS input_processors JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS output_processors JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN agent.input_processors IS 'Ordered input processor names applied before LLM execution.';
COMMENT ON COLUMN agent.output_processors IS 'Ordered output processor names applied after LLM execution.';
