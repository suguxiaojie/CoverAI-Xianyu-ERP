-- +goose Up
ALTER TABLE order_refresh_jobs ADD COLUMN sync_mode VARCHAR(32) NOT NULL DEFAULT 'incremental';

CREATE TABLE order_sync_cursors (
    cookie_id VARCHAR(255) PRIMARY KEY,
    high_water_created_at VARCHAR(64) NOT NULL,
    high_water_order_id VARCHAR(255) NOT NULL,
    last_incremental_sync_at BIGINT NOT NULL DEFAULT 0,
    last_full_sync_at BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_order_sync_cursors_cookie FOREIGN KEY (cookie_id) REFERENCES cookies(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS order_sync_cursors;
ALTER TABLE order_refresh_jobs DROP COLUMN sync_mode;
