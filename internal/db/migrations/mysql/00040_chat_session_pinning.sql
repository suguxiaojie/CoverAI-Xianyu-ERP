-- +goose Up
ALTER TABLE chat_sessions
    ADD COLUMN is_pinned TINYINT(1) NOT NULL DEFAULT 0,
    ADD COLUMN pinned_at BIGINT NOT NULL DEFAULT 0;
CREATE INDEX idx_chat_sessions_account_pinned_recent
    ON chat_sessions(cookie_id,is_pinned,pinned_at,last_message_at);

-- +goose Down
ALTER TABLE chat_sessions DROP INDEX idx_chat_sessions_account_pinned_recent;
ALTER TABLE chat_sessions
    DROP COLUMN pinned_at,
    DROP COLUMN is_pinned;
