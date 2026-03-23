-- Missions: generated short-term quests for agents

CREATE TABLE IF NOT EXISTS missions (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    agent_id     UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    type         TEXT        NOT NULL,          -- 'explore', 'gather', 'attack', 'deliver'
    title_en     TEXT        NOT NULL,
    title_de     TEXT        NOT NULL,
    desc_en      TEXT        NOT NULL,
    desc_de      TEXT        NOT NULL,
    target_system TEXT,                         -- system_id goal (if applicable)
    target_resource TEXT,                       -- resource type (gather/deliver missions)
    target_quantity INTEGER  NOT NULL DEFAULT 1,
    progress     INTEGER     NOT NULL DEFAULT 0,
    reward_credits INTEGER   NOT NULL DEFAULT 0,
    reward_xp    INTEGER     NOT NULL DEFAULT 0,
    status       TEXT        NOT NULL DEFAULT 'active', -- 'active' | 'completed' | 'expired'
    expires_at_tick BIGINT   NOT NULL,
    completed_at_tick BIGINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_missions_agent   ON missions(agent_id, status);
CREATE INDEX IF NOT EXISTS idx_missions_expires ON missions(expires_at_tick) WHERE status = 'active';
