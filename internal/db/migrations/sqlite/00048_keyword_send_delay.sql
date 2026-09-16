-- +goose Up
ALTER TABLE keywords ADD COLUMN send_delay_seconds INTEGER NOT NULL DEFAULT 0;
CREATE TABLE keyword_reply_pending_tasks (cookie_id TEXT NOT NULL,chat_id TEXT NOT NULL,group_id TEXT NOT NULL,due_at INTEGER NOT NULL,status TEXT NOT NULL DEFAULT 'pending',updated_at INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(cookie_id,chat_id,group_id));

-- +goose Down
DROP TABLE IF EXISTS keyword_reply_pending_tasks;
ALTER TABLE keywords DROP COLUMN send_delay_seconds;
