CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE gw_faction AS ENUM ('tau_ri', 'free_jaffa', 'gate_nomad', 'system_lord_remnant', 'wraith_brood', 'ancient_seeker');
CREATE TYPE gw_playstyle AS ENUM ('fighter', 'trader', 'researcher');
CREATE TYPE gw_ship_class AS ENUM ('gate_runner_mk1');

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT 'en',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID UNIQUE NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT UNIQUE NOT NULL,
    faction gw_faction NOT NULL,
    playstyle gw_playstyle NOT NULL,
    credits INTEGER NOT NULL DEFAULT 750,
    experience INTEGER NOT NULL DEFAULT 0,
    skills JSONB NOT NULL DEFAULT '{}',
    research JSONB NOT NULL DEFAULT '[]',
    reputation JSONB NOT NULL DEFAULT '{}',
    mission_brief TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT 'Gate Runner Mk.I',
    class gw_ship_class NOT NULL DEFAULT 'gate_runner_mk1',
    hull_points INTEGER NOT NULL DEFAULT 100,
    max_hull_points INTEGER NOT NULL DEFAULT 100,
    galaxy_id TEXT NOT NULL DEFAULT 'milky_way',
    system_id TEXT NOT NULL DEFAULT 'chulak',
    planet_id TEXT,
    equipment JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ships_agent_id ON ships(agent_id);
