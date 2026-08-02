CREATE TABLE product_drafts (
    id BIGSERIAL PRIMARY KEY,
    image_key TEXT NOT NULL DEFAULT '',
    suggested_fields JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending_review' CHECK (status IN ('pending_review', 'approved', 'rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by BIGINT REFERENCES users(id)
);

CREATE INDEX idx_product_drafts_status ON product_drafts(status);
