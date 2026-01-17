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
    image_url TEXT,
    search_vector tsvector,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Full-text search index for products
CREATE INDEX IF NOT EXISTS idx_read_products_search ON read_products USING gin(search_vector);
CREATE INDEX IF NOT EXISTS idx_read_products_price ON read_products(price);
CREATE INDEX IF NOT EXISTS idx_read_products_name ON read_products(name);

-- Function to update search vector automatically
CREATE OR REPLACE FUNCTION update_product_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS product_search_update ON read_products;
CREATE TRIGGER product_search_update
    BEFORE INSERT OR UPDATE ON read_products
    FOR EACH ROW
    EXECUTE FUNCTION update_product_search_vector();

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

-- Users read model
CREATE TABLE IF NOT EXISTS read_users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'customer',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_read_users_email ON read_users(email);
CREATE INDEX idx_read_users_role ON read_users(role);

-- User sessions table
CREATE TABLE IF NOT EXISTS user_sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    refresh_token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    ip_address VARCHAR(45),
    user_agent TEXT
);

CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_expires ON user_sessions(expires_at);

-- Categories read model
CREATE TABLE IF NOT EXISTS read_categories (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    parent_id VARCHAR(255),
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_read_categories_parent ON read_categories(parent_id);
CREATE INDEX idx_read_categories_slug ON read_categories(slug);
CREATE INDEX idx_read_categories_sort ON read_categories(sort_order);

-- Product-Category relationship (many-to-many)
CREATE TABLE IF NOT EXISTS product_categories (
    product_id VARCHAR(255) NOT NULL,
    category_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (product_id, category_id)
);

CREATE INDEX idx_product_categories_product ON product_categories(product_id);
CREATE INDEX idx_product_categories_category ON product_categories(category_id);

-- ============================================
-- Initial Admin User
-- ============================================
-- Password: admin123 (bcrypt hash with cost 12)
INSERT INTO read_users (id, email, password_hash, name, role, is_active, created_at, updated_at)
VALUES (
    'admin-00000000-0000-0000-0000-000000000001',
    'admin@example.com',
    '$2a$12$EG4mxT0VuwfDNwqLyp0hv.Jeyt/ZRGarO7k2xA8s7qeo13Mphj0FS',
    'Administrator',
    'admin',
    true,
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;
