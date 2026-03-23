CREATE TABLE research_queue (
    agent_id UUID PRIMARY KEY REFERENCES agents(id) ON DELETE CASCADE,
    tech_id TEXT NOT NULL,
    started_at_tick BIGINT NOT NULL,
    completes_at_tick BIGINT NOT NULL
);
