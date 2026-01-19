# ETL Worker Test Guide

This guide explains how to test the ETL worker that transfers and encrypts data from a source database to a target database.

## Overview

The ETL worker performs the following steps:
1. Connects to source database and reads card data (PAN, CVV)
2. Calls KMS gRPC service to encrypt PAN and CVV
3. Writes encrypted data (ciphertext + nonce) to target database

## Prerequisites

1. **KMS Server Running**: The gRPC server must be running
   ```powershell
   .\start-kms.ps1
   ```

2. **Database Setup**: You need both source and target databases configured

## Quick Start

### Step 1: Setup Test Databases

Run the SQL setup scripts for your database type:

**For SQL Server:**
```powershell
# Run in SQL Server Management Studio or via sqlcmd
sqlcmd -S localhost -U sa -P YourPassword123 -i scripts\setup-test-db-sqlserver-source.sql
sqlcmd -S localhost -U sa -P YourPassword123 -i scripts\setup-test-db-sqlserver-target.sql
```

**For MySQL:**
```powershell
mysql -u root -p < scripts\setup-test-db-mysql-source.sql
mysql -u root -p < scripts\setup-test-db-mysql-target.sql
```

### Step 2: Configure ETL Worker

The test script will automatically create a `config.yaml` file, or you can create it manually:

```yaml
kms:
  addr: "127.0.0.1:50051"

auth:
  bearerToken: ""

sourceDB:
  driver: "sqlserver"  # or "mysql"
  dsn: "sqlserver://sa:YourPassword123@localhost:1433?database=kms_test_source&encrypt=disable"

destDB:
  driver: "sqlserver"  # or "mysql"
  dsn: "sqlserver://sa:YourPassword123@localhost:1433?database=kms_test_target&encrypt=disable"
```

### Step 3: Run Test

```powershell
.\test-etl-worker.ps1
```

Or with custom parameters:

```powershell
.\test-etl-worker.ps1 `
    -SourceDBDriver "sqlserver" `
    -TargetDBDriver "sqlserver" `
    -SourceDBDSN "sqlserver://sa:Password@localhost:1433?database=source_db&encrypt=disable" `
    -TargetDBDSN "sqlserver://sa:Password@localhost:1433?database=target_db&encrypt=disable"
```

## Test Process

The test script performs the following steps:

1. **Check KMS Server**: Verifies KMS gRPC server is reachable
2. **Create Config**: Generates `config.yaml` for ETL worker
3. **Setup Databases**: Creates test tables and inserts sample data
4. **Run ETL Worker**: Executes `go run ./cmd/etl-worker`
5. **Verify Results**: Decrypts data from target DB to verify correctness
6. **Cleanup**: Removes temporary files

## Verification

After the ETL worker runs, you can manually verify the results:

```powershell
.\scripts\verify-etl-results.ps1 `
    -Driver "sqlserver" `
    -DSN "sqlserver://sa:Password@localhost:1433?database=kms_test_target&encrypt=disable" `
    -KMSAddr "127.0.0.1:50051"
```

## Test Data

The setup scripts create 5 test records:
- Card 1: PAN=4111111111111111, CVV=123
- Card 2: PAN=5500000000000004, CVV=456
- Card 3: PAN=340000000000009, CVV=789
- Card 4: PAN=6011000000000004, CVV=012
- Card 5: PAN=5105105105105100, CVV=345

## Troubleshooting

### ETL Worker Fails to Connect to Source DB

- Check database connection string in `config.yaml`
- Verify database server is running
- Check credentials and permissions

### ETL Worker Fails to Connect to KMS

- Ensure KMS gRPC server is running: `.\start-kms.ps1`
- Check KMS address in `config.yaml` matches the running server
- Verify firewall/network settings

### Verification Fails

- Ensure KMS server is still running (same master key)
- Check that encrypted data exists in target database
- Verify database connection string for verification script

### No Records Found

- Check if source database has data in `cards_to_encrypt` table
- Verify ETL worker completed without errors
- Check target database `encrypted_cards` table

## Database Schema

### Source Database Table: `cards_to_encrypt`

**SQL Server:**
```sql
CREATE TABLE cards_to_encrypt (
    id INT IDENTITY(1,1) PRIMARY KEY,
    card_no NVARCHAR(19) NOT NULL,
    cvv NVARCHAR(4) NOT NULL,
    other_data NVARCHAR(255)
);
```

**MySQL:**
```sql
CREATE TABLE cards_to_encrypt (
    id INT AUTO_INCREMENT PRIMARY KEY,
    card_no VARCHAR(19) NOT NULL,
    cvv VARCHAR(4) NOT NULL,
    other_data VARCHAR(255)
);
```

### Target Database Table: `encrypted_cards`

**SQL Server:**
```sql
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
```

**MySQL:**
```sql
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
```

## Next Steps

After successful testing:

1. **Integrate with SSIS**: See `docs/SSIS_INTEGRATION.md`
2. **Setup HSM**: See `docs/HSM_INTEGRATION.md` for production HSM integration
3. **Performance Optimization**: See `docs/PERFORMANCE_OPTIMIZATION.md`
4. **Customize ETL Worker**: Modify `cmd/etl-worker/main.go` for your specific schema

## Related Documentation

- [SSIS Integration Guide](docs/SSIS_INTEGRATION.md)
- [HSM Integration Guide](docs/HSM_INTEGRATION.md)
- [Performance Optimization](docs/PERFORMANCE_OPTIMIZATION.md)
- [Main README](README.md)

