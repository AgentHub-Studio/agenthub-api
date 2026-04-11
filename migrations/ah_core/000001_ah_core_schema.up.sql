-- Global ah_core schema: platform-managed agents, skills, and tools.
-- These are available to ALL tenants via CoreToolLoader.
-- Tools use use_caller_token = true so operations run in the caller's tenant context.

CREATE SCHEMA IF NOT EXISTS ah_core;

-- Tools: HTTP tools that proxy REST calls back to agenthub-api
CREATE TABLE IF NOT EXISTS ah_core.tool (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    type        VARCHAR(50)  NOT NULL DEFAULT 'HTTP',
    config      JSONB        NOT NULL DEFAULT '{}',
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_tool_slug      ON ah_core.tool (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_tool_is_active ON ah_core.tool (is_active);

-- Skills: grouped capabilities for platform management
CREATE TABLE IF NOT EXISTS ah_core.skill (
    id                       UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name                     VARCHAR(255) NOT NULL,
    slug                     VARCHAR(255) NOT NULL UNIQUE,
    description              TEXT,
    instructions             TEXT,
    category                 VARCHAR(100) NOT NULL DEFAULT 'platform',
    disable_model_invocation BOOLEAN      NOT NULL DEFAULT FALSE,
    context_mode             VARCHAR(50)  NOT NULL DEFAULT 'inline',
    when_to_use              TEXT,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_skill_slug ON ah_core.skill (slug);

-- Skill-Tool bindings
CREATE TABLE IF NOT EXISTS ah_core.skill_tool (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id   UUID        NOT NULL REFERENCES ah_core.skill (id) ON DELETE CASCADE,
    tool_id    UUID        NOT NULL REFERENCES ah_core.tool  (id) ON DELETE CASCADE,
    priority   INTEGER     NOT NULL DEFAULT 0,
    is_active  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (skill_id, tool_id)
);

-- Agents: specialist agents for platform management
CREATE TABLE IF NOT EXISTS ah_core.agent (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(255) NOT NULL,
    slug              VARCHAR(255) NOT NULL UNIQUE,
    description       TEXT,
    agent_type        VARCHAR(50)  NOT NULL DEFAULT 'ASSISTANT',
    system_prompt     TEXT,
    model_config      JSONB        NOT NULL DEFAULT '{}',
    enable_management BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active         BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_agent_slug      ON ah_core.agent (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_agent_type      ON ah_core.agent (agent_type);
CREATE INDEX IF NOT EXISTS idx_ah_core_agent_is_active ON ah_core.agent (is_active);

-- Agent-Skill bindings
CREATE TABLE IF NOT EXISTS ah_core.agent_skill (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id   UUID        NOT NULL REFERENCES ah_core.agent (id) ON DELETE CASCADE,
    skill_id   UUID        NOT NULL REFERENCES ah_core.skill (id) ON DELETE CASCADE,
    priority   INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (agent_id, skill_id)
);
