-- +goose Up
-- FAQ 相似问法只保存用户确认的检索别名，不保存聊天或调试问题日志明文。
CREATE TABLE knowledge_faq_aliases (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    knowledge_base_id BIGINT NOT NULL,
    faq_id BIGINT NOT NULL,
    alias VARCHAR(500) NOT NULL,
    normalized_alias VARCHAR(500) NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'manual',
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_knowledge_faq_aliases_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    CONSTRAINT fk_knowledge_faq_aliases_faq FOREIGN KEY (faq_id) REFERENCES knowledge_faqs(id) ON DELETE CASCADE,
    UNIQUE KEY uk_knowledge_faq_aliases_base_normalized (knowledge_base_id, normalized_alias)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX idx_knowledge_faq_aliases_faq_enabled ON knowledge_faq_aliases(faq_id, enabled, id);

-- +goose Down
DROP TABLE IF EXISTS knowledge_faq_aliases;
