CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE orders (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku            VARCHAR(64) NOT NULL,
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    status         VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'reserved', 'failed')),
    failure_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_sku ON orders (sku);
