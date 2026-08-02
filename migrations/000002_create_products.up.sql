CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    brand TEXT NOT NULL DEFAULT '',
    category_id BIGINT NOT NULL REFERENCES categories(id),
    description TEXT NOT NULL DEFAULT '',
    active_ingredient TEXT NOT NULL DEFAULT '',
    nafdac_reg_number TEXT,
    requires_prescription BOOLEAN NOT NULL DEFAULT false,
    primary_image_key TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_slug ON products(slug);
