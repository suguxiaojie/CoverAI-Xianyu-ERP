-- +goose Up
CREATE TABLE order_shipment_proofs (
    order_id TEXT PRIMARY KEY,
    cookie_id TEXT NOT NULL,
    trade_text TEXT NOT NULL DEFAULT '',
    image_urls_json TEXT NOT NULL DEFAULT '[]',
    source TEXT NOT NULL DEFAULT 'erp',
    submitted_at INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (cookie_id) REFERENCES cookies(id) ON DELETE CASCADE
);
CREATE INDEX idx_order_shipment_proofs_account ON order_shipment_proofs(cookie_id, submitted_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_order_shipment_proofs_account;
DROP TABLE IF EXISTS order_shipment_proofs;
