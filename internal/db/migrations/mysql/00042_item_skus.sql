-- +goose Up
CREATE TABLE item_skus (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    cookie_id VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL,
    sku_id VARCHAR(255) NOT NULL,
    inventory_id VARCHAR(255) NOT NULL DEFAULT '',
    properties_json LONGTEXT NOT NULL,
    price_cents BIGINT NOT NULL DEFAULT 0,
    quantity INT NOT NULL DEFAULT 0,
    initial_quantity INT NOT NULL DEFAULT 0,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    sort_order INT NOT NULL DEFAULT 0,
    cost_cents BIGINT NULL,
    synced_at BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    CONSTRAINT fk_item_skus_item FOREIGN KEY (cookie_id, item_id) REFERENCES item_info(cookie_id, item_id) ON DELETE CASCADE,
    UNIQUE KEY uk_item_skus_item_sku (cookie_id, item_id, sku_id),
    KEY idx_item_skus_item_active (cookie_id, item_id, deleted_at, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE item_skus;
