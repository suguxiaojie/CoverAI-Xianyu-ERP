-- +goose Up
ALTER TABLE keywords ADD COLUMN reply_interval_seconds BIGINT NOT NULL DEFAULT 1;
CREATE TABLE keyword_reply_records (cookie_id TEXT NOT NULL,chat_id TEXT NOT NULL,group_id TEXT NOT NULL,last_success_at BIGINT NOT NULL DEFAULT 0,pending_token TEXT NOT NULL DEFAULT '',lease_expires_at BIGINT NOT NULL DEFAULT 0,updated_at BIGINT NOT NULL DEFAULT 0,PRIMARY KEY(cookie_id,chat_id,group_id));

-- +goose Down
DROP TABLE IF EXISTS keyword_reply_records;
ALTER TABLE keywords DROP COLUMN reply_interval_seconds;
