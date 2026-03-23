CREATE TABLE tick_state (
    id INT PRIMARY KEY DEFAULT 1,
    tick_number BIGINT NOT NULL DEFAULT 0,
    last_tick_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO tick_state VALUES (1, 0, NOW());

CREATE TABLE tick_queue (
    agent_id UUID PRIMARY KEY REFERENCES agents(id) ON DELETE CASCADE,
    galaxy_id TEXT NOT NULL,
    action_type TEXT NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}',
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tick_number BIGINT NOT NULL,
    type TEXT NOT NULL,
    payload_en TEXT NOT NULL,
    payload_de TEXT NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_agent_id_tick ON events(agent_id, tick_number DESC);
