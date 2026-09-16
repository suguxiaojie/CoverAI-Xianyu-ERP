-- +goose Up
ALTER TABLE order_refresh_jobs ADD COLUMN sync_mode TEXT NOT NULL DEFAULT 'incremental';

CREATE TABLE order_sync_cursors (
    cookie_id TEXT PRIMARY KEY REFERENCES cookies(id) ON DELETE CASCADE,
    high_water_created_at TEXT NOT NULL,
    high_water_order_id TEXT NOT NULL,
    last_incremental_sync_at BIGINT NOT NULL DEFAULT 0,
    last_full_sync_at BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS order_sync_cursors;
ALTER TABLE order_refresh_jobs DROP COLUMN sync_mode;
