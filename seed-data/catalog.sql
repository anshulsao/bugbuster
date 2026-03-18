CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    stock INT NOT NULL DEFAULT 0,
    category VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO products (name, description, price, stock, category) VALUES
    ('Wireless Bluetooth Headphones', 'Noise-cancelling over-ear headphones with 30hr battery life', 79.99, 150, 'electronics'),
    ('USB-C Hub 7-in-1', 'Multiport adapter with HDMI, USB-A, SD card reader', 34.99, 300, 'electronics'),
    ('Mechanical Keyboard', 'Cherry MX Brown switches, RGB backlit, full-size', 129.99, 85, 'electronics'),
    ('Clean Code', 'Robert C. Martin - A handbook of agile software craftsmanship', 39.99, 200, 'books'),
    ('Designing Data-Intensive Applications', 'Martin Kleppmann - The big ideas behind reliable systems', 49.99, 120, 'books'),
    ('The Pragmatic Programmer', 'David Thomas, Andrew Hunt - 20th Anniversary Edition', 44.99, 175, 'books'),
    ('Cotton Crew Neck T-Shirt', '100% organic cotton, available in 6 colors', 24.99, 500, 'clothing'),
    ('Slim Fit Chinos', 'Stretch cotton blend, business casual', 59.99, 250, 'clothing'),
    ('Portable SSD 1TB', 'NVMe external drive, USB 3.2, 1050 MB/s read', 89.99, 100, 'electronics'),
    ('Wool Blend Zip Hoodie', 'Merino wool blend, unisex, charcoal grey', 69.99, 180, 'clothing');
