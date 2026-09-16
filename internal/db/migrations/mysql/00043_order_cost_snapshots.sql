-- +goose Up
CREATE TABLE order_cost_snapshots (
    order_id VARCHAR(255) PRIMARY KEY,
    cookie_id VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL,
    sku_id VARCHAR(255) NOT NULL,
    unit_cost_cents BIGINT NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    match_source VARCHAR(64) NOT NULL,
    captured_at BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_order_cost_snapshots_order FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    CONSTRAINT fk_order_cost_snapshots_sku FOREIGN KEY (cookie_id,item_id,sku_id) REFERENCES item_skus(cookie_id,item_id,sku_id),
    KEY idx_order_cost_snapshots_item (cookie_id,item_id,sku_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO order_cost_snapshots(order_id,cookie_id,item_id,sku_id,unit_cost_cents,quantity,match_source,captured_at)
SELECT o.order_id,o.cookie_id,o.item_id,s.sku_id,s.cost_cents,
       CAST(o.quantity AS UNSIGNED),
       'single_local_default',UNIX_TIMESTAMP()
FROM orders o
JOIN item_skus s ON s.cookie_id=o.cookie_id AND s.item_id=o.item_id AND s.sku_id='__default__' AND s.deleted_at IS NULL
WHERE o.deleted_at IS NULL AND o.order_status IN ('pending_ship','paid','2','shipped','3','received','completed','4','11')
  AND s.cost_cents IS NOT NULL AND o.quantity REGEXP '^[1-9][0-9]*$';

-- +goose Down
DROP TABLE order_cost_snapshots;
