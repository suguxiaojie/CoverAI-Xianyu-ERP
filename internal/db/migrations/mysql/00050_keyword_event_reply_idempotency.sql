-- +goose Up
-- 系统关键词订单事件幂等：卖家实时卡片与后续买家通知只能成功发送一次。
CREATE TABLE keyword_event_reply_records (
    -- 三个业务标识参与 utf8mb4 复合主键；191 字符保证最坏索引宽度低于 InnoDB 3072 字节上限。
    cookie_id VARCHAR(191) NOT NULL,
    order_id VARCHAR(191) NOT NULL,
    group_id VARCHAR(191) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    claim_token VARCHAR(255) NOT NULL DEFAULT '',
    lease_expires_at BIGINT NOT NULL DEFAULT 0,
    last_success_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (cookie_id, order_id, group_id, event_type),
    KEY idx_keyword_event_reply_status_lease (status, lease_expires_at, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS keyword_event_reply_records;
