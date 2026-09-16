-- +goose Up
ALTER TABLE chat_messages ADD COLUMN reply_to_platform_message_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_chat_messages_account_reply_target
    ON chat_messages(cookie_id,reply_to_platform_message_id);

-- +goose Down
DROP INDEX IF EXISTS idx_chat_messages_account_reply_target;
ALTER TABLE chat_messages DROP COLUMN reply_to_platform_message_id;
