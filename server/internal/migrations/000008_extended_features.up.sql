-- Extended features: Alliances, Chat, Admin, Respawn, Upgrades, Bounties, Galactic Events

-- ── Alliances ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS alliances (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    proposer_id UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    target_id   UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    status      TEXT        NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposer_id, target_id)
);
CREATE INDEX IF NOT EXISTS idx_alliances_target_id   ON alliances(target_id);
CREATE INDEX IF NOT EXISTS idx_alliances_proposer_id ON alliances(proposer_id);

-- ── Veto / Override tracking ───────────────────────────────────────────────
ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS last_veto_at     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_override_at TIMESTAMPTZ;

-- ── Chat ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS chat_messages (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    channel    TEXT        NOT NULL,
    sender_id  UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL CHECK (char_length(content) BETWEEN 1 AND 500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_channel ON chat_messages(channel, created_at DESC);

CREATE TABLE IF NOT EXISTS chat_mutes (
    muter_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    muted_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (muter_id, muted_id)
);

CREATE TABLE IF NOT EXISTS chat_reports (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    message_id  UUID        NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Admin ─────────────────────────────────────────────────────────────────
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS is_admin   BOOLEAN     NOT NULL DEFAULT false;
ALTER TABLE agents   ADD COLUMN IF NOT EXISTS banned_at  TIMESTAMPTZ;
ALTER TABLE agents   ADD COLUMN IF NOT EXISTS ban_reason TEXT;

CREATE TABLE IF NOT EXISTS admin_audit_log (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_account_id UUID        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    action           TEXT        NOT NULL,
    target_id        TEXT,
    details          JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_created_at ON admin_audit_log(created_at DESC);

-- ── Respawn tracking ──────────────────────────────────────────────────────
ALTER TABLE agents ADD COLUMN IF NOT EXISTS death_tick   BIGINT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS respawn_tick BIGINT;

-- ── Ship upgrade levels ───────────────────────────────────────────────────
ALTER TABLE ships ADD COLUMN IF NOT EXISTS weapon_level INTEGER NOT NULL DEFAULT 1;
ALTER TABLE ships ADD COLUMN IF NOT EXISTS shield_level INTEGER NOT NULL DEFAULT 1;
ALTER TABLE ships ADD COLUMN IF NOT EXISTS engine_level INTEGER NOT NULL DEFAULT 1;
ALTER TABLE ships ADD COLUMN IF NOT EXISTS cargo_level  INTEGER NOT NULL DEFAULT 1;

-- ── New ship classes ──────────────────────────────────────────────────────
ALTER TYPE gw_ship_class ADD VALUE IF NOT EXISTS 'patrol_craft';
ALTER TYPE gw_ship_class ADD VALUE IF NOT EXISTS 'destroyer';
ALTER TYPE gw_ship_class ADD VALUE IF NOT EXISTS 'battlecruiser';

-- ── Bounties ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS bounties (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    placer_id  UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    target_id  UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    amount     INTEGER     NOT NULL CHECK (amount >= 250),
    status     TEXT        NOT NULL DEFAULT 'active',
    claimed_by UUID        REFERENCES agents(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days'
);
CREATE INDEX IF NOT EXISTS idx_bounties_target_active ON bounties(target_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_bounties_placer        ON bounties(placer_id);

-- ── Galactic events ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS galactic_events (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type     TEXT        NOT NULL,
    galaxy_id      TEXT        NOT NULL DEFAULT 'all',
    title_en       TEXT        NOT NULL,
    title_de       TEXT        NOT NULL,
    description_en TEXT        NOT NULL,
    description_de TEXT        NOT NULL,
    effect         JSONB       NOT NULL DEFAULT '{}',
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at        TIMESTAMPTZ,
    created_by     UUID        REFERENCES accounts(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_galactic_events_active ON galactic_events(galaxy_id, ends_at);
