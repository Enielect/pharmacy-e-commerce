CREATE TABLE prescriptions (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    file_key TEXT NOT NULL,
    verified_by_user_id BIGINT REFERENCES users(id),
    verified_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX idx_prescriptions_order_id ON prescriptions(order_id);
