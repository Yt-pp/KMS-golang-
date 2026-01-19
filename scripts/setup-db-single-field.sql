-- Database Schema for Single-Field Encryption Format
-- This stores encrypted_pan and encrypted_cvv as single base64-encoded strings
-- Format: base64(nonce + ciphertext) where nonce is 12 bytes for AES-GCM

-- SQL Server Schema
IF OBJECT_ID('encrypted_cards', 'U') IS NOT NULL
    DROP TABLE encrypted_cards;
GO

CREATE TABLE encrypted_cards (
    id INT IDENTITY(1,1) PRIMARY KEY,
    source_id INT NOT NULL,
    encrypted_pan NVARCHAR(MAX) NOT NULL,  -- Base64: nonce + ciphertext
    encrypted_cvv NVARCHAR(MAX) NOT NULL,  -- Base64: nonce + ciphertext
    other_data NVARCHAR(255),
    created_at DATETIME DEFAULT GETDATE()
);
GO

-- MySQL Schema
/*
CREATE TABLE encrypted_cards (
    id INT AUTO_INCREMENT PRIMARY KEY,
    source_id INT NOT NULL,
    encrypted_pan TEXT NOT NULL,  -- Base64: nonce + ciphertext
    encrypted_cvv TEXT NOT NULL,  -- Base64: nonce + ciphertext
    other_data VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
*/

-- PostgreSQL Schema
/*
CREATE TABLE encrypted_cards (
    id SERIAL PRIMARY KEY,
    source_id INT NOT NULL,
    encrypted_pan TEXT NOT NULL,  -- Base64: nonce + ciphertext
    encrypted_cvv TEXT NOT NULL,  -- Base64: nonce + ciphertext
    other_data VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
*/

SELECT 'Database schema for single-field encryption format created successfully.';
GO

