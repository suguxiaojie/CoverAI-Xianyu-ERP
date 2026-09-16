-- +goose Up
CREATE TABLE order_shipment_proofs (
    order_id VARCHAR(255) PRIMARY KEY,
    cookie_id VARCHAR(255) NOT NULL,
    trade_text TEXT NOT NULL,
    image_urls_json TEXT NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'erp',
    submitted_at BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_order_shipment_proofs_order FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    CONSTRAINT fk_order_shipment_proofs_cookie FOREIGN KEY (cookie_id) REFERENCES cookies(id) ON DELETE CASCADE,
    INDEX idx_order_shipment_proofs_account (cookie_id, submitted_at)
);

-- +goose Down
DROP TABLE IF EXISTS order_shipment_proofs;
