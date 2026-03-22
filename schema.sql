CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sender_id INTEGER NOT NULL,
    receiver_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    meta TEXT,
    comment TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_sender
    FOREIGN KEY (sender_id) REFERENCES users(id),
    CONSTRAINT fk_receiver
    FOREIGN KEY (receiver_id) REFERENCES users(id),
    CONSTRAINT sender_receiver
    CHECK (sender_id <> receiver_id)
);

CREATE TABLE IF NOT EXISTS scans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER NOT NULL,
    courier_id INTEGER NOT NULL,
    photo BLOB,
    condition VARCHAR(255) NOT NULL,
    longitude DECIMAL,
    latitude DECIMAL,
    comment TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_order
    FOREIGN KEY (order_id) REFERENCES orders(id),
    CONSTRAINT fk_courier
    FOREIGN KEY (courier_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS contacts (
    owner_id INTEGER NOT NULL,
    contact_id INTEGER NOT NULL,
    PRIMARY KEY (owner_id, contact_id),
    FOREIGN KEY (owner_id) REFERENCES users(id),
    FOREIGN KEY (contact_id) REFERENCES users(id),
    CHECK (owner_id <> contact_id)
);

