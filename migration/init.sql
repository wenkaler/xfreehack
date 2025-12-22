CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(255) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS stores (
    id SERIAL PRIMARY KEY,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    url VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS coupons (
    id SERIAL PRIMARY KEY,
    store_id INTEGER REFERENCES stores(id) ON DELETE CASCADE,
    code VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    expiry_date BIGINT NOT NULL,
    link TEXT UNIQUE,
    is_exclusive BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chats (
    id BIGINT PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    user_name VARCHAR(100),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id BIGINT PRIMARY KEY,
    chat_id BIGINT REFERENCES chats(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS relation_chat_coupons (
    id SERIAL PRIMARY KEY,
    coupon_id INTEGER REFERENCES coupons(id) ON DELETE CASCADE,
    chat_id BIGINT REFERENCES chats(id) ON DELETE CASCADE,
    status BOOLEAN DEFAULT FALSE,
    UNIQUE(coupon_id, chat_id)
);

CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    message TEXT NOT NULL,
    sent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

-- Note: We are modifying an existing table, for init.sql we can just recreate or ALTER based on if it's new env.
-- Since this is init.sql for a container volume we assume it runs on clean state or we can use generic ALTER if exists
-- But postgres init.sql only runs on empty DB. 
-- However, for the user flow of 'refactoring', we will add ALTER here to be safe if they keep volume, 
-- though we usually reset volume.
ALTER TABLE chats ADD COLUMN IF NOT EXISTS notification_schedule VARCHAR(20) DEFAULT '18:00';

