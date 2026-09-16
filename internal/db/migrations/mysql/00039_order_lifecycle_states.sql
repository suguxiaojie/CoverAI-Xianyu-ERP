-- +goose Up
ALTER TABLE orders ADD COLUMN received_at VARCHAR(64) DEFAULT '';
ALTER TABLE orders ADD COLUMN refunded_at VARCHAR(64) DEFAULT '';
ALTER TABLE orders ADD COLUMN cancelled_at VARCHAR(64) DEFAULT '';

-- +goose Down
ALTER TABLE orders DROP COLUMN cancelled_at;
ALTER TABLE orders DROP COLUMN refunded_at;
ALTER TABLE orders DROP COLUMN received_at;
