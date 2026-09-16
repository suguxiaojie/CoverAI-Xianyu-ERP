-- +goose Up
ALTER TABLE chat_sessions ADD COLUMN is_pinned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chat_sessions ADD COLUMN pinned_at INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_chat_sessions_account_pinned_recent
    ON chat_sessions(cookie_id,is_pinned DESC,pinned_at DESC,last_message_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_chat_sessions_account_pinned_recent;
ALTER TABLE chat_sessions DROP COLUMN pinned_at;
ALTER TABLE chat_sessions DROP COLUMN is_pinned;
