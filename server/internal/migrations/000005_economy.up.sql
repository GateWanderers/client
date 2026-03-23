CREATE TABLE inventories (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    PRIMARY KEY (agent_id, resource_type)
);

CREATE TABLE trading_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    planet_id UUID NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL,
    base_price INTEGER NOT NULL,
    current_price INTEGER NOT NULL,
    supply INTEGER NOT NULL DEFAULT 100,
    demand INTEGER NOT NULL DEFAULT 100,
    UNIQUE (planet_id, resource_type)
);

CREATE TABLE market_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    buyer_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    resource_type TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    price_per_unit INTEGER NOT NULL CHECK (price_per_unit > 0),
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_market_orders_status ON market_orders(status);
CREATE INDEX idx_inventories_agent ON inventories(agent_id);
