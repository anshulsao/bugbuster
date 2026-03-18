CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id BIGINT NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    external_ref VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO payments (order_id, amount, status, external_ref) VALUES
    (1, 159.98, 'completed', 'ext_stripe_ch_abc123'),
    (2, 39.99, 'completed', 'ext_stripe_ch_def456'),
    (4, 129.99, 'pending', NULL);
