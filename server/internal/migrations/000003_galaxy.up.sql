CREATE TABLE planets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    galaxy_id TEXT NOT NULL,
    system_id TEXT NOT NULL,
    system_name TEXT NOT NULL,
    system_x REAL NOT NULL,
    system_y REAL NOT NULL,
    name TEXT NOT NULL,
    gate_address TEXT UNIQUE NOT NULL,
    resource_nodes JSONB NOT NULL DEFAULT '[]',
    npc_presence JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_planets_galaxy ON planets(galaxy_id);
CREATE INDEX idx_planets_system ON planets(system_id);
CREATE INDEX idx_planets_gate ON planets(gate_address);

CREATE TABLE agent_known_planets (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    planet_id UUID NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, planet_id)
);
