-- +goose Up
-- 知识库第一阶段：只存储人工维护 FAQ／文档、确定性分块和脱敏检索日志。
CREATE TABLE knowledge_bases (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_knowledge_bases_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY uk_knowledge_bases_user_name (user_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX idx_knowledge_bases_user_status ON knowledge_bases(user_id, status, id);

CREATE TABLE knowledge_base_accounts (
    knowledge_base_id BIGINT NOT NULL,
    cookie_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (knowledge_base_id, cookie_id),
    CONSTRAINT fk_knowledge_base_accounts_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    CONSTRAINT fk_knowledge_base_accounts_cookie FOREIGN KEY (cookie_id) REFERENCES cookies(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE knowledge_base_items (
    knowledge_base_id BIGINT NOT NULL,
    cookie_id VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (knowledge_base_id, cookie_id, item_id),
    CONSTRAINT fk_knowledge_base_items_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    CONSTRAINT fk_knowledge_base_items_cookie FOREIGN KEY (cookie_id) REFERENCES cookies(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE knowledge_faqs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    knowledge_base_id BIGINT NOT NULL,
    question TEXT NOT NULL,
    answer LONGTEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    review_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
    requires_live_data TINYINT(1) NOT NULL DEFAULT 0,
    allow_auto_reply TINYINT(1) NOT NULL DEFAULT 0,
    enabled TINYINT(1) NOT NULL DEFAULT 0,
    effective_from BIGINT NOT NULL DEFAULT 0,
    effective_to BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_knowledge_faqs_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX idx_knowledge_faqs_base_gate ON knowledge_faqs(knowledge_base_id, enabled, review_status, status, id);

CREATE TABLE knowledge_documents (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    knowledge_base_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content LONGTEXT NOT NULL,
    content_type VARCHAR(32) NOT NULL DEFAULT 'text',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    review_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
    requires_live_data TINYINT(1) NOT NULL DEFAULT 0,
    allow_auto_reply TINYINT(1) NOT NULL DEFAULT 0,
    enabled TINYINT(1) NOT NULL DEFAULT 0,
    effective_from BIGINT NOT NULL DEFAULT 0,
    effective_to BIGINT NOT NULL DEFAULT 0,
    content_hash VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_knowledge_documents_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX idx_knowledge_documents_base_gate ON knowledge_documents(knowledge_base_id, enabled, review_status, status, id);

CREATE TABLE knowledge_chunks (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    knowledge_base_id BIGINT NOT NULL,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content LONGTEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_knowledge_chunks_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    UNIQUE KEY uk_knowledge_chunks_source (source_type, source_id, chunk_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX idx_knowledge_chunks_base_source ON knowledge_chunks(knowledge_base_id, source_type, source_id, enabled, chunk_index);

CREATE TABLE knowledge_retrieve_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    query_digest VARCHAR(64) NOT NULL,
    answerability VARCHAR(32) NOT NULL,
    evidence_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_knowledge_retrieve_logs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE INDEX idx_knowledge_retrieve_logs_user_created ON knowledge_retrieve_logs(user_id, id);

-- +goose Down
DROP TABLE IF EXISTS knowledge_retrieve_logs;
DROP TABLE IF EXISTS knowledge_chunks;
DROP TABLE IF EXISTS knowledge_documents;
DROP TABLE IF EXISTS knowledge_faqs;
DROP TABLE IF EXISTS knowledge_base_items;
DROP TABLE IF EXISTS knowledge_base_accounts;
DROP TABLE IF EXISTS knowledge_bases;
