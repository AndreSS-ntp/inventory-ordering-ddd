CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE stock_items (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku               VARCHAR(64) NOT NULL UNIQUE,
    total_quantity    INTEGER NOT NULL CHECK (total_quantity >= 0),
    reserved_quantity INTEGER NOT NULL DEFAULT 0 CHECK (reserved_quantity >= 0),
    version           INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT reserved_lte_total CHECK (reserved_quantity <= total_quantity)
);

INSERT INTO stock_items (sku, total_quantity, reserved_quantity, version) VALUES
    ('SKU-001', 100, 0, 0),
    ('SKU-002', 10, 0, 0),
    ('SKU-003', 0, 0, 0);
