CREATE TABLE npc_factions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    name_de TEXT NOT NULL,
    galaxy_id TEXT NOT NULL,
    fleet_strength INTEGER NOT NULL DEFAULT 100,
    agenda TEXT NOT NULL DEFAULT 'neutral',
    territory_systems TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE world_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    faction_id TEXT NOT NULL REFERENCES npc_factions(id),
    event_type TEXT NOT NULL,
    galaxy_id TEXT NOT NULL,
    system_id TEXT,
    payload_en TEXT NOT NULL,
    payload_de TEXT NOT NULL,
    tick_number BIGINT NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_world_events_tick ON world_events(tick_number DESC);
CREATE INDEX idx_world_events_galaxy ON world_events(galaxy_id);
