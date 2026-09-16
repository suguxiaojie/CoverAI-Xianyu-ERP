-- +goose Up
-- 已下架或缺少 SKU 的历史订单使用独立不可变成本覆盖，不伪造当前商品目录。
CREATE TABLE historical_order_cost_overrides (
    order_id TEXT PRIMARY KEY,
    cookie_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    unit_cost_cents BIGINT NOT NULL CHECK (unit_cost_cents >= 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    match_source TEXT NOT NULL DEFAULT 'manual_historical_cost',
    captured_at BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_historical_order_cost_order FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE
);
CREATE INDEX idx_historical_order_cost_overrides_item ON historical_order_cost_overrides(cookie_id, item_id);

-- +goose Down
DROP TABLE IF EXISTS historical_order_cost_overrides;
