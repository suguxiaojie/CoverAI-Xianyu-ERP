-- +goose Up
-- FAQ 相似问法只保存用户确认的检索别名，不保存聊天或调试问题日志明文。
CREATE TABLE knowledge_faq_aliases (
    id BIGSERIAL PRIMARY KEY,
    knowledge_base_id BIGINT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    faq_id BIGINT NOT NULL REFERENCES knowledge_faqs(id) ON DELETE CASCADE,
    alias TEXT NOT NULL,
    normalized_alias TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(knowledge_base_id, normalized_alias)
);
CREATE INDEX idx_knowledge_faq_aliases_faq_enabled ON knowledge_faq_aliases(faq_id, enabled, id);

-- +goose Down
DROP TABLE IF EXISTS knowledge_faq_aliases;
