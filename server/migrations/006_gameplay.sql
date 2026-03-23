-- Phase 12: Gameplay depth — Respawn, Ship Upgrades, Bounties, Galactic Events

-- ── Respawn tracking ──────────────────────────────────────────────────────
ALTER TABLE agents ADD COLUMN IF NOT EXISTS death_tick   BIGINT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS respawn_tick BIGINT;  -- tick when auto-respawn fires

-- ── Ship upgrade levels (1–5 each) ───────────────────────────────────────
ALTER TABLE ships ADD COLUMN IF NOT EXISTS weapon_level  INTEGER NOT NULL DEFAULT 1;
ALTER TABLE ships ADD COLUMN IF NOT EXISTS shield_level  INTEGER NOT NULL DEFAULT 1;
ALTER TABLE ships ADD COLUMN IF NOT EXISTS engine_level  INTEGER NOT NULL DEFAULT 1;
ALTER TABLE ships ADD COLUMN IF NOT EXISTS cargo_level   INTEGER NOT NULL DEFAULT 1;

-- ── New ship classes ──────────────────────────────────────────────────────
ALTER TYPE gw_ship_class ADD VALUE IF NOT EXISTS 'patrol_craft';
ALTER TYPE gw_ship_class ADD VALUE IF NOT EXISTS 'destroyer';
ALTER TYPE gw_ship_class ADD VALUE IF NOT EXISTS 'battlecruiser';

-- ── Bounties ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS bounties (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    placer_id  UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    target_id  UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    amount     INTEGER     NOT NULL CHECK (amount >= 100),
    status     TEXT        NOT NULL DEFAULT 'active', -- active | claimed | expired
    claimed_by UUID        REFERENCES agents(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days'
);
CREATE INDEX IF NOT EXISTS idx_bounties_target_active ON bounties(target_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_bounties_placer        ON bounties(placer_id);

-- ── Galactic events ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS galactic_events (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type     TEXT        NOT NULL,  -- INVASION | TRADE_BOOM | NEBULA_STORM | ARMISTICE | TOURNAMENT
    galaxy_id      TEXT        NOT NULL DEFAULT 'all',
    title_en       TEXT        NOT NULL,
    title_de       TEXT        NOT NULL,
    description_en TEXT        NOT NULL,
    description_de TEXT        NOT NULL,
    -- effect JSON controls tick-engine modifiers, e.g. {"combat_mult":1.5,"trade_mult":0.5}
    effect         JSONB       NOT NULL DEFAULT '{}',
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at        TIMESTAMPTZ,            -- NULL = permanent until admin ends it
    created_by     UUID        REFERENCES accounts(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_galactic_events_active
    ON galactic_events(galaxy_id, ends_at);
