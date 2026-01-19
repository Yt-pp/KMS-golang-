-- Setup Target Database for ETL Worker Test (MySQL)
CREATE DATABASE IF NOT EXISTS kms_test_target;
USE kms_test_target;

DROP TABLE IF EXISTS encrypted_cards;

CREATE TABLE encrypted_cards (
    id INT AUTO_INCREMENT PRIMARY KEY,
    source_id INT NOT NULL,
    pan_ciphertext BLOB NOT NULL,
    pan_nonce BLOB NOT NULL,
    cvv_ciphertext BLOB NOT NULL,
    cvv_nonce BLOB NOT NULL,
    other_data VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

SELECT 'Target database setup complete.';
