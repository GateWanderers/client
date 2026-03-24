CREATE TABLE auctions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_agent_id  UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    resource_type    TEXT NOT NULL,
    quantity         BIGINT NOT NULL CHECK (quantity > 0),
    starting_price   BIGINT NOT NULL CHECK (starting_price > 0),
    buyout_price     BIGINT,           -- NULL = kein Sofortkauf
    current_bid      BIGINT,           -- NULL = noch kein Gebot
    current_bidder_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    system_id        UUID REFERENCES systems(id) ON DELETE CASCADE,
    expires_at_tick  BIGINT NOT NULL,  -- Tick-Nummer bei der die Auktion abläuft
    status           TEXT NOT NULL DEFAULT 'active', -- active | sold | expired | cancelled
    created_at       TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX auctions_status_idx ON auctions(status);
CREATE INDEX auctions_seller_idx ON auctions(seller_agent_id);
