-- Setup Source Database for ETL Worker Test
USE master;
GO

IF NOT EXISTS (SELECT * FROM sys.databases WHERE name = 'kms_test_source')
BEGIN
    CREATE DATABASE kms_test_source;
END
GO

USE kms_test_source;
GO

IF OBJECT_ID('cards_to_encrypt', 'U') IS NOT NULL
    DROP TABLE cards_to_encrypt;
GO

CREATE TABLE cards_to_encrypt (
    id INT IDENTITY(1,1) PRIMARY KEY,
    card_no NVARCHAR(19) NOT NULL,
    cvv NVARCHAR(4) NOT NULL,
    other_data NVARCHAR(255)
);
GO

-- Insert test data
INSERT INTO cards_to_encrypt (card_no, cvv, other_data) VALUES
    ('4111111111111111', '123', 'Test Card 1'),
    ('5500000000000004', '456', 'Test Card 2'),
    ('340000000000009', '789', 'Test Card 3'),
    ('6011000000000004', '012', 'Test Card 4'),
    ('5105105105105100', '345', 'Test Card 5');
GO

SELECT 'Source database setup complete. Records inserted: ' + CAST(@@ROWCOUNT AS NVARCHAR(10));
GO
