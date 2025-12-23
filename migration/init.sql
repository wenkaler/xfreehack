CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(255) UNIQUE
);

CREATE TABLE IF NOT EXISTS stores (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(255) UNIQUE,
    url TEXT,
    category_id INTEGER REFERENCES categories(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS coupons (
    id SERIAL PRIMARY KEY,
    store_id INTEGER REFERENCES stores(id) ON DELETE CASCADE,
    code VARCHAR(100),
    description TEXT,
    expiry_date BIGINT,
    link TEXT NOT NULL UNIQUE,
    is_exclusive BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(store_id, code, description)
);

CREATE TABLE IF NOT EXISTS chats (
    id BIGINT PRIMARY KEY,
    type VARCHAR(50),
    user_name VARCHAR(255),
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notification_schedule VARCHAR(20) DEFAULT '18:00', -- Legacy, can be removed later
    timezone_offset INTEGER DEFAULT 3,
    notification_hour INTEGER DEFAULT 18
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT REFERENCES chats(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id) ON DELETE CASCADE,
    store_id INTEGER REFERENCES stores(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_subscription_target CHECK (
        (category_id IS NOT NULL AND store_id IS NULL) OR 
        (category_id IS NULL AND store_id IS NOT NULL)
    ),
    UNIQUE (chat_id, category_id, store_id)
);

CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    message TEXT NOT NULL,
    target_segment VARCHAR(50) DEFAULT 'all', -- 'all', 'migrated', 'group_id'
    is_sent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP
);
