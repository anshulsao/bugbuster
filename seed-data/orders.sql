CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    product_id INT NOT NULL,
    quantity INT NOT NULL,
    total_price DECIMAL(10,2),
    status VARCHAR(50) DEFAULT 'pending',
    payment_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO orders (product_id, quantity, total_price, status, payment_id) VALUES
    (1, 2, 159.98, 'completed', 'pay_abc123'),
    (4, 1, 39.99, 'completed', 'pay_def456'),
    (7, 3, 74.97, 'pending', NULL),
    (3, 1, 129.99, 'processing', 'pay_ghi789'),
    (9, 1, 89.99, 'completed', 'pay_jkl012');
