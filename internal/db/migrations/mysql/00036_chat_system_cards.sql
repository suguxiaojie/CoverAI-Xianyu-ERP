-- +goose Up
ALTER TABLE chat_messages ADD COLUMN platform_content_type INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chat_messages ADD COLUMN system_card_kind VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_event VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_title VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_description VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_order_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_item_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_action VARCHAR(64) NOT NULL DEFAULT '';
CREATE INDEX idx_chat_messages_system_order ON chat_messages(cookie_id,system_card_order_id);

-- +goose Down
DROP INDEX idx_chat_messages_system_order ON chat_messages;
ALTER TABLE chat_messages DROP COLUMN system_card_action;
ALTER TABLE chat_messages DROP COLUMN system_card_item_id;
ALTER TABLE chat_messages DROP COLUMN system_card_order_id;
ALTER TABLE chat_messages DROP COLUMN system_card_description;
ALTER TABLE chat_messages DROP COLUMN system_card_title;
ALTER TABLE chat_messages DROP COLUMN system_card_event;
ALTER TABLE chat_messages DROP COLUMN system_card_kind;
ALTER TABLE chat_messages DROP COLUMN platform_content_type;
