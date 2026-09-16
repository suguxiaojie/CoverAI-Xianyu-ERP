-- +goose Up
-- +goose StatementBegin
ALTER TABLE chat_messages ADD COLUMN platform_message_id TEXT;
ALTER TABLE chat_messages ADD COLUMN recalled_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chat_messages ADD COLUMN recall_operator_type INTEGER NOT NULL DEFAULT -1;
ALTER TABLE chat_messages ADD COLUMN recall_operator_id TEXT NOT NULL DEFAULT '';
UPDATE chat_messages SET platform_message_id=message_key WHERE message_key LIKE '%.PNM';
CREATE UNIQUE INDEX uq_chat_messages_account_platform_id ON chat_messages(cookie_id,platform_message_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX uq_chat_messages_account_platform_id;
ALTER TABLE chat_messages DROP COLUMN recall_operator_id;
ALTER TABLE chat_messages DROP COLUMN recall_operator_type;
ALTER TABLE chat_messages DROP COLUMN recalled_at;
ALTER TABLE chat_messages DROP COLUMN platform_message_id;
-- +goose StatementEnd
