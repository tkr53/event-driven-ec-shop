-- Event Store table
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY,
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    data JSONB NOT NULL,
    version INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Ensure events are appended in order per aggregate
    UNIQUE (aggregate_id, version)
);

-- Index for querying events by aggregate
CREATE INDEX idx_events_aggregate_id ON events(aggregate_id);
CREATE INDEX idx_events_aggregate_type ON events(aggregate_type);
CREATE INDEX idx_events_created_at ON events(created_at);

-- Optimistic locking: ensure version is sequential
CREATE OR REPLACE FUNCTION check_event_version()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.version != (
        SELECT COALESCE(MAX(version), 0) + 1
        FROM events
        WHERE aggregate_id = NEW.aggregate_id
    ) THEN
        RAISE EXCEPTION 'Optimistic locking failure: expected version %, got %',
            (SELECT COALESCE(MAX(version), 0) + 1 FROM events WHERE aggregate_id = NEW.aggregate_id),
            NEW.version;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ensure_event_version
    BEFORE INSERT ON events
    FOR EACH ROW
    EXECUTE FUNCTION check_event_version();

-- ============================================
-- Read Model Tables (Query Side)
-- ============================================

-- Products read model
CREATE TABLE IF NOT EXISTS read_products (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price INT NOT NULL,
    stock INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Carts read model
CREATE TABLE IF NOT EXISTS read_carts (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    items JSONB NOT NULL DEFAULT '[]',
    total INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_read_carts_user_id ON read_carts(user_id);

-- Orders read model
CREATE TABLE IF NOT EXISTS read_orders (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    items JSONB NOT NULL DEFAULT '[]',
    total INT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_read_orders_user_id ON read_orders(user_id);
CREATE INDEX idx_read_orders_status ON read_orders(status);

-- Inventory read model
CREATE TABLE IF NOT EXISTS read_inventory (
    product_id VARCHAR(255) PRIMARY KEY,
    total_stock INT NOT NULL DEFAULT 0,
    reserved_stock INT NOT NULL DEFAULT 0,
    available_stock INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
