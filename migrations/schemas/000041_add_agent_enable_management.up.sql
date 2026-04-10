-- P-C184-2 / P-C281-1: gate agenthub_manage tool to agents that explicitly opt in.
-- Default false ensures all existing agents are unaffected by this change.
ALTER TABLE agent
    ADD COLUMN IF NOT EXISTS enable_management BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN agent.enable_management IS
    'When true, the agenthub_manage builtin tool is included in the agent toolset '
    '(subject to the caller having the admin role). '
    'Default false — most agents should never manage other agents.';
