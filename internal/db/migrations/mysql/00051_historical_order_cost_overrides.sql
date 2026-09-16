-- +goose Up
-- 已下架或缺少 SKU 的历史订单使用独立不可变成本覆盖，不伪造当前商品目录。
CREATE TABLE historical_order_cost_overrides (
    order_id VARCHAR(255) NOT NULL,
    cookie_id VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL,
    unit_cost_cents BIGINT NOT NULL,
    quantity INT NOT NULL,
    match_source VARCHAR(64) NOT NULL DEFAULT 'manual_historical_cost',
    captured_at BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (order_id),
    KEY idx_historical_order_cost_overrides_item (cookie_id, item_id),
    CONSTRAINT fk_historical_order_cost_order FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    CONSTRAINT chk_historical_order_cost_nonnegative CHECK (unit_cost_cents >= 0),
    CONSTRAINT chk_historical_order_cost_quantity CHECK (quantity > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS historical_order_cost_overrides;
