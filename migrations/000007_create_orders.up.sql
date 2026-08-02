CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'pending_payment' CHECK (status IN ('pending_payment','paid','pending_pharmacist_review','processing','shipped','cancelled')),
    total_cents INTEGER NOT NULL DEFAULT 0,
    shipping_address_line1 TEXT NOT NULL DEFAULT '',
    shipping_address_city TEXT NOT NULL DEFAULT '',
    shipping_address_state TEXT NOT NULL DEFAULT '',
    shipping_address_phone TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);

CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    variant_id BIGINT NOT NULL,
    product_name_snapshot TEXT NOT NULL,
    price_cents_snapshot INTEGER NOT NULL,
    quantity INTEGER NOT NULL
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
