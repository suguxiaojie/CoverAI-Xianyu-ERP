-- +goose Up
ALTER TABLE orders ADD COLUMN received_at TEXT DEFAULT '';
ALTER TABLE orders ADD COLUMN refunded_at TEXT DEFAULT '';
ALTER TABLE orders ADD COLUMN cancelled_at TEXT DEFAULT '';

-- +goose Down
ALTER TABLE orders DROP COLUMN cancelled_at;
ALTER TABLE orders DROP COLUMN refunded_at;
ALTER TABLE orders DROP COLUMN received_at;
