ALTER TABLE agents ADD COLUMN status TEXT NOT NULL DEFAULT 'active';

CREATE TABLE combat_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attacker_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    defender_type TEXT NOT NULL,
    defender_id TEXT NOT NULL,
    galaxy_id TEXT NOT NULL,
    system_id TEXT NOT NULL,
    rounds JSONB NOT NULL DEFAULT '[]',
    outcome TEXT NOT NULL,
    loot JSONB NOT NULL DEFAULT '[]',
    tick_number BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_combat_logs_attacker ON combat_logs(attacker_id);
