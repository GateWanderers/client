-- Mining nodes: finite resource deposits per system with depletion and regen
CREATE TABLE mining_nodes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id        TEXT NOT NULL,
    resource_type    TEXT NOT NULL,
    richness         TEXT NOT NULL DEFAULT 'normal', -- poor | normal | rich | bonanza
    current_reserves INTEGER NOT NULL DEFAULT 500,
    max_reserves     INTEGER NOT NULL DEFAULT 500,
    regen_per_tick   INTEGER NOT NULL DEFAULT 5,
    last_mined_tick  BIGINT,
    UNIQUE(system_id, resource_type)
);

CREATE INDEX mining_nodes_system_idx ON mining_nodes(system_id);

-- Survey results: agent has scanned a system's mining nodes
CREATE TABLE surveys (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id         UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    system_id        TEXT NOT NULL,
    surveyed_at_tick BIGINT NOT NULL,
    expires_at_tick  BIGINT NOT NULL,
    UNIQUE(agent_id, system_id)
);

CREATE INDEX surveys_agent_idx ON surveys(agent_id);
CREATE INDEX surveys_expires_idx ON surveys(expires_at_tick);

-- Agent skills: active abilities with level progression and cooldowns
CREATE TABLE agent_skills (
    agent_id              UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_id              TEXT NOT NULL,
    level                 INTEGER NOT NULL DEFAULT 1,
    xp                    INTEGER NOT NULL DEFAULT 0,
    cooldown_expires_tick BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (agent_id, skill_id)
);

CREATE INDEX agent_skills_agent_idx ON agent_skills(agent_id);

-- Active skill boosts: pending one-shot effects (e.g. overcharge_drill, cargo_compress)
CREATE TABLE skill_boosts (
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_id        TEXT NOT NULL,
    magnitude       FLOAT NOT NULL DEFAULT 1.0,
    expires_at_tick BIGINT NOT NULL,
    PRIMARY KEY (agent_id, skill_id)
);

-- Cargo capacity per ship (different classes carry different amounts)
ALTER TABLE ships ADD COLUMN IF NOT EXISTS cargo_capacity INTEGER NOT NULL DEFAULT 80;
