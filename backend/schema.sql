-- Table 1: Supported Locations (The Dictionary)
CREATE TABLE locations (
    id SERIAL PRIMARY KEY,
    city VARCHAR(100) NOT NULL,
    barangay VARCHAR(100) NOT NULL,
    -- Prevent duplicate entries for the same barangay
    UNIQUE (city, barangay) 
);

-- Table 2: The Subscribers (Web Push Credentials)
CREATE TABLE subscribers (
    id SERIAL PRIMARY KEY,
    push_subscription JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table 3: The Junction Table (Mapping Users to Locations)
CREATE TABLE user_subscriptions (
    subscriber_id INT REFERENCES subscribers(id) ON DELETE CASCADE,
    location_id INT REFERENCES locations(id) ON DELETE CASCADE,
    -- A user shouldn't be able to subscribe to the same barangay twice
    PRIMARY KEY (subscriber_id, location_id)
);

INSERT INTO locations (city, barangay) VALUES
    ('Valenzuela', 'Karuhatan'),
    ('Valenzuela', 'Malanday'),
    ('Valenzuela', 'Marulas'),
    ('Valenzuela', 'Gen. T. de Leon');