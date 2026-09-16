-- +goose Up
CREATE TABLE item_skus (
    id BIGSERIAL PRIMARY KEY,
    cookie_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    sku_id TEXT NOT NULL,
    inventory_id TEXT NOT NULL DEFAULT '',
    properties_json TEXT NOT NULL DEFAULT '[]',
    price_cents BIGINT NOT NULL DEFAULT 0,
    quantity INTEGER NOT NULL DEFAULT 0,
    initial_quantity INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    cost_cents BIGINT NULL,
    synced_at BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    FOREIGN KEY (cookie_id, item_id) REFERENCES item_info(cookie_id, item_id) ON DELETE CASCADE,
    UNIQUE(cookie_id, item_id, sku_id)
);
CREATE INDEX idx_item_skus_item_active ON item_skus(cookie_id, item_id, deleted_at, sort_order);

-- +goose Down
DROP INDEX IF EXISTS idx_item_skus_item_active;
DROP TABLE item_skus;
