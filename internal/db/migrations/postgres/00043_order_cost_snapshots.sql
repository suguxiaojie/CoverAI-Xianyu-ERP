-- +goose Up
CREATE TABLE order_cost_snapshots (
    order_id TEXT PRIMARY KEY REFERENCES orders(order_id) ON DELETE CASCADE,
    cookie_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    sku_id TEXT NOT NULL,
    unit_cost_cents BIGINT NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    match_source TEXT NOT NULL,
    captured_at BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cookie_id,item_id,sku_id) REFERENCES item_skus(cookie_id,item_id,sku_id)
);
CREATE INDEX idx_order_cost_snapshots_item ON order_cost_snapshots(cookie_id,item_id,sku_id);

INSERT INTO order_cost_snapshots(order_id,cookie_id,item_id,sku_id,unit_cost_cents,quantity,match_source,captured_at)
SELECT o.order_id,o.cookie_id,o.item_id,s.sku_id,s.cost_cents,
       CAST(o.quantity AS INTEGER),
       'single_local_default',EXTRACT(EPOCH FROM CURRENT_TIMESTAMP)::BIGINT
FROM orders o
JOIN item_skus s ON s.cookie_id=o.cookie_id AND s.item_id=o.item_id AND s.sku_id='__default__' AND s.deleted_at IS NULL
WHERE o.deleted_at IS NULL AND o.order_status IN ('pending_ship','paid','2','shipped','3','received','completed','4','11')
  AND s.cost_cents IS NOT NULL AND o.quantity ~ '^[1-9][0-9]*$'
ON CONFLICT (order_id) DO NOTHING;

-- +goose Down
DROP INDEX IF EXISTS idx_order_cost_snapshots_item;
DROP TABLE order_cost_snapshots;
