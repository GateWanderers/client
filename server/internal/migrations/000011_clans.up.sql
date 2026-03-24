-- Clans / Guilds
CREATE TABLE clans (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE NOT NULL,
    tag         VARCHAR(5) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    leader_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    treasury    BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE clan_members (
    clan_id     UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    role        TEXT NOT NULL DEFAULT 'member', -- 'leader' | 'officer' | 'member'
    joined_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (clan_id, agent_id)
);

-- Each agent can only be in one clan at a time.
CREATE UNIQUE INDEX clan_members_agent_unique ON clan_members(agent_id);

-- Fast lookup: which clan does agent X belong to?
CREATE INDEX clan_members_agent_idx ON clan_members(agent_id);
