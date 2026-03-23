-- +migrate Up
CREATE TABLE alliances (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    proposer_id UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    target_id   UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    status      TEXT        NOT NULL DEFAULT 'pending',  -- 'pending' | 'active'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposer_id, target_id)
);

CREATE INDEX idx_alliances_target_id  ON alliances(target_id);
CREATE INDEX idx_alliances_proposer_id ON alliances(proposer_id);
