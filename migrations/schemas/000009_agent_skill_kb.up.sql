-- Links agents to skills they can use in agentic mode.
CREATE TABLE agent_skill (
    agent_id   UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    skill_id   UUID NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, skill_id)
);

-- Links agents to knowledge bases available for RAG in agentic mode.
CREATE TABLE agent_knowledge_base (
    agent_id          UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_base(id) ON DELETE CASCADE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, knowledge_base_id)
);
