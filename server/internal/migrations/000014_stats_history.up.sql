CREATE TABLE IF NOT EXISTS stats_history (
    tick_number           BIGINT      PRIMARY KEY,
    total_credits         BIGINT      NOT NULL DEFAULT 0,
    active_agents         INT         NOT NULL DEFAULT 0,
    total_ships           INT         NOT NULL DEFAULT 0,
    player_systems        INT         NOT NULL DEFAULT 0,
    system_control        JSONB       NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
