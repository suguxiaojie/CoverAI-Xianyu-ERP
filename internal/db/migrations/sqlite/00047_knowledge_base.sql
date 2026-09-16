-- +goose Up
-- 知识库第一阶段：只存储人工维护 FAQ／文档、确定性分块和脱敏检索日志。
CREATE TABLE knowledge_bases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, name)
);
CREATE INDEX idx_knowledge_bases_user_status ON knowledge_bases(user_id, status, id);

CREATE TABLE knowledge_base_accounts (
    knowledge_base_id INTEGER NOT NULL,
    cookie_id TEXT NOT NULL,
    PRIMARY KEY (knowledge_base_id, cookie_id),
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    FOREIGN KEY (cookie_id) REFERENCES cookies(id) ON DELETE CASCADE
);

CREATE TABLE knowledge_base_items (
    knowledge_base_id INTEGER NOT NULL,
    cookie_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    PRIMARY KEY (knowledge_base_id, cookie_id, item_id),
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    FOREIGN KEY (cookie_id) REFERENCES cookies(id) ON DELETE CASCADE
);

CREATE TABLE knowledge_faqs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_base_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    review_status TEXT NOT NULL DEFAULT 'pending',
    risk_level TEXT NOT NULL DEFAULT 'low',
    requires_live_data INTEGER NOT NULL DEFAULT 0,
    allow_auto_reply INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 0,
    effective_from INTEGER NOT NULL DEFAULT 0,
    effective_to INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE
);
CREATE INDEX idx_knowledge_faqs_base_gate ON knowledge_faqs(knowledge_base_id, enabled, review_status, status, id);

CREATE TABLE knowledge_documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_base_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'text',
    status TEXT NOT NULL DEFAULT 'draft',
    review_status TEXT NOT NULL DEFAULT 'pending',
    risk_level TEXT NOT NULL DEFAULT 'low',
    requires_live_data INTEGER NOT NULL DEFAULT 0,
    allow_auto_reply INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 0,
    effective_from INTEGER NOT NULL DEFAULT 0,
    effective_to INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE
);
CREATE INDEX idx_knowledge_documents_base_gate ON knowledge_documents(knowledge_base_id, enabled, review_status, status, id);

CREATE TABLE knowledge_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_base_id INTEGER NOT NULL,
    source_type TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    UNIQUE(source_type, source_id, chunk_index)
);
CREATE INDEX idx_knowledge_chunks_base_source ON knowledge_chunks(knowledge_base_id, source_type, source_id, enabled, chunk_index);

CREATE TABLE knowledge_retrieve_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    query_digest TEXT NOT NULL,
    answerability TEXT NOT NULL,
    evidence_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_knowledge_retrieve_logs_user_created ON knowledge_retrieve_logs(user_id, id);

-- +goose Down
DROP TABLE IF EXISTS knowledge_retrieve_logs;
DROP TABLE IF EXISTS knowledge_chunks;
DROP TABLE IF EXISTS knowledge_documents;
DROP TABLE IF EXISTS knowledge_faqs;
DROP TABLE IF EXISTS knowledge_base_items;
DROP TABLE IF EXISTS knowledge_base_accounts;
DROP TABLE IF EXISTS knowledge_bases;
