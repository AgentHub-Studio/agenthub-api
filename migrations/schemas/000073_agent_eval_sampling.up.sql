ALTER TABLE agent
  ADD COLUMN IF NOT EXISTS eval_config JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN agent.eval_config IS 'Production eval sampling config: scorers and sample_rate.';

CREATE TABLE IF NOT EXISTS eval_run (
  id UUID PRIMARY KEY,
  chat_run_id UUID REFERENCES chat_run(id) ON DELETE SET NULL,
  session_id UUID NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
  agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
  scorers JSONB NOT NULL DEFAULT '[]'::jsonb,
  status VARCHAR(32) NOT NULL DEFAULT 'queued',
  score DOUBLE PRECISION NOT NULL DEFAULT 1,
  sample_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_eval_run_agent_created_at ON eval_run(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_eval_run_session_created_at ON eval_run(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_eval_run_chat_run ON eval_run(chat_run_id);
