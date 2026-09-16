-- +goose Up
ALTER TABLE keywords ADD COLUMN send_delay_seconds BIGINT NOT NULL DEFAULT 0;
CREATE TABLE keyword_reply_pending_tasks (cookie_id VARCHAR(255) NOT NULL,chat_id VARCHAR(255) NOT NULL,group_id VARCHAR(255) NOT NULL,due_at BIGINT NOT NULL,status VARCHAR(32) NOT NULL DEFAULT 'pending',updated_at BIGINT NOT NULL DEFAULT 0,PRIMARY KEY(cookie_id,chat_id,group_id));

-- +goose Down
DROP TABLE IF EXISTS keyword_reply_pending_tasks;
ALTER TABLE keywords DROP COLUMN send_delay_seconds;
