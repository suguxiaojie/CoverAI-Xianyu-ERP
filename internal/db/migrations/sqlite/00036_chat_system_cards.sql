-- +goose Up
ALTER TABLE chat_messages ADD COLUMN platform_content_type INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chat_messages ADD COLUMN system_card_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_event TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_title TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_description TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_order_id TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_item_id TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_messages ADD COLUMN system_card_action TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_chat_messages_system_order ON chat_messages(cookie_id,system_card_order_id);

-- +goose Down
DROP INDEX idx_chat_messages_system_order;
ALTER TABLE chat_messages DROP COLUMN system_card_action;
ALTER TABLE chat_messages DROP COLUMN system_card_item_id;
ALTER TABLE chat_messages DROP COLUMN system_card_order_id;
ALTER TABLE chat_messages DROP COLUMN system_card_description;
ALTER TABLE chat_messages DROP COLUMN system_card_title;
ALTER TABLE chat_messages DROP COLUMN system_card_event;
ALTER TABLE chat_messages DROP COLUMN system_card_kind;
ALTER TABLE chat_messages DROP COLUMN platform_content_type;
