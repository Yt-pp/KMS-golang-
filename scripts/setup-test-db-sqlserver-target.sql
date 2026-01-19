-- Setup Target Database for ETL Worker Test
USE master;
GO

IF NOT EXISTS (SELECT * FROM sys.databases WHERE name = 'kms_test_target')
BEGIN
    CREATE DATABASE kms_test_target;
END
GO

USE kms_test_target;
GO

IF OBJECT_ID('encrypted_cards', 'U') IS NOT NULL
    DROP TABLE encrypted_cards;
GO

CREATE TABLE encrypted_cards (
    id INT IDENTITY(1,1) PRIMARY KEY,
    source_id INT NOT NULL,
    pan_ciphertext VARBINARY(MAX) NOT NULL,
    pan_nonce VARBINARY(MAX) NOT NULL,
    cvv_ciphertext VARBINARY(MAX) NOT NULL,
    cvv_nonce VARBINARY(MAX) NOT NULL,
    other_data NVARCHAR(255),
    created_at DATETIME DEFAULT GETDATE()
);
GO

SELECT 'Target database setup complete.';
GO
