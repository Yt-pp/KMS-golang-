-- Setup Source Database for ETL Worker Test (MySQL)
CREATE DATABASE IF NOT EXISTS kms_test_source;
USE kms_test_source;

DROP TABLE IF EXISTS cards_to_encrypt;

CREATE TABLE cards_to_encrypt (
    id INT AUTO_INCREMENT PRIMARY KEY,
    card_no VARCHAR(19) NOT NULL,
    cvv VARCHAR(4) NOT NULL,
    other_data VARCHAR(255)
);

-- Insert test data
INSERT INTO cards_to_encrypt (card_no, cvv, other_data) VALUES
    ('4111111111111111', '123', 'Test Card 1'),
    ('5500000000000004', '456', 'Test Card 2'),
    ('340000000000009', '789', 'Test Card 3'),
    ('6011000000000004', '012', 'Test Card 4'),
    ('5105105105105100', '345', 'Test Card 5');

SELECT CONCAT('Source database setup complete. Records inserted: ', ROW_COUNT());
