-- +goose Up
-- 系统关键词订单事件幂等：卖家实时卡片与后续买家通知只能成功发送一次。
CREATE TABLE keyword_event_reply_records (
    cookie_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    claim_token TEXT NOT NULL DEFAULT '',
    lease_expires_at INTEGER NOT NULL DEFAULT 0,
    last_success_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (cookie_id, order_id, group_id, event_type)
);
CREATE INDEX idx_keyword_event_reply_status_lease ON keyword_event_reply_records(status, lease_expires_at, updated_at);

-- +goose Down
DROP TABLE IF EXISTS keyword_event_reply_records;
