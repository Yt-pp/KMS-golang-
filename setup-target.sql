-- Target DB Setup
CREATE TABLE IF NOT EXISTS encrypted_users (
    id INTEGER PRIMARY KEY, 
    full_name_enc TEXT, 
    credit_card_enc TEXT, 
    email_enc TEXT
);
DELETE FROM encrypted_users;
