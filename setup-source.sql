-- Source DB Setup
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY, 
    full_name TEXT, 
    credit_card TEXT, 
    email TEXT
);
DELETE FROM users;
INSERT INTO users (full_name, credit_card, email) VALUES 
('John Doe', '1234-5678-9012-3456', 'john@example.com'),
('Jane Smith', '9876-5432-1098-7654', 'jane@test.com');
