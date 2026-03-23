-- +migrate Up
CREATE TABLE chat_messages (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    channel    TEXT        NOT NULL,  -- 'global' | faction slug, e.g. 'tau_ri'
    sender_id  UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL CHECK (char_length(content) BETWEEN 1 AND 500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_channel ON chat_messages(channel, created_at DESC);

CREATE TABLE chat_mutes (
    muter_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    muted_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (muter_id, muted_id)
);

CREATE TABLE chat_reports (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    message_id  UUID        NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
