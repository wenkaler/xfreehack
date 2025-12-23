CREATE TABLE IF NOT EXISTS collection_logs (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL, -- 'success', 'failed'
    categories_count INT DEFAULT 0,
    stores_count INT DEFAULT 0,
    coupons_count INT DEFAULT 0,
    duration_ms INT DEFAULT 0,
    error_message TEXT
);
