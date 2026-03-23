-- System Control: territorial ownership of star systems

CREATE TABLE IF NOT EXISTS system_control (
    system_id          TEXT        NOT NULL,
    galaxy_id          TEXT        NOT NULL,
    controller_faction TEXT        NOT NULL DEFAULT 'unclaimed',
    controller_type    TEXT        NOT NULL DEFAULT 'unclaimed', -- 'unclaimed' | 'npc' | 'player'
    defense_strength   INTEGER     NOT NULL DEFAULT 0 CHECK (defense_strength >= 0),
    income_per_tick    INTEGER     NOT NULL DEFAULT 50,
    last_contested_at  BIGINT,
    captured_at        BIGINT,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (system_id, galaxy_id)
);

CREATE INDEX IF NOT EXISTS idx_system_control_galaxy     ON system_control(galaxy_id);
CREATE INDEX IF NOT EXISTS idx_system_control_controller ON system_control(controller_faction, controller_type);
