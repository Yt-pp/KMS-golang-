# ETL Worker Test Script - Source DB to Target DB Encryption Transfer
# Usage: .\test-etl-worker.ps1 [-SourceDBDriver sqlserver|mysql] [-TargetDBDriver sqlserver|mysql]

param(
    [string]$SourceDBDriver = "",
    [string]$TargetDBDriver = "",
    [string]$SourceDBDSN = "",
    [string]$TargetDBDSN = "",
    [string]$KMSGrpcAddr = "127.0.0.1:50051",
    [switch]$SkipSetup,
    [switch]$SkipCleanup
)

Write-Host "=== ETL Worker Test - Source DB to Target DB Encryption Transfer ===" -ForegroundColor Cyan
Write-Host ""

# Default DSNs if not provided
if ([string]::IsNullOrEmpty($SourceDBDSN)) {
    if ($SourceDBDriver -eq "sqlserver") {
        $SourceDBDSN = "sqlserver://sa:YourPassword123@localhost:1433?database=kms_test_source&encrypt=disable"
    } else {
        $SourceDBDSN = "root:1234@tcp(localhost:3306)/kms_test_source"
    }
}

if ([string]::IsNullOrEmpty($TargetDBDSN)) {
    if ($TargetDBDriver -eq "sqlserver") {
        $TargetDBDSN = "sqlserver://sa:YourPassword123@localhost:1433?database=kms_test_target&encrypt=disable"
    } else {
        $TargetDBDSN = "root:1234@tcp(localhost:3306)/kms_test_target"
    }
}

Write-Host "Configuration:" -ForegroundColor Yellow
Write-Host "  Source DB Driver: $SourceDBDriver" -ForegroundColor Gray
Write-Host "  Target DB Driver: $TargetDBDriver" -ForegroundColor Gray
Write-Host "  KMS gRPC Addr: $KMSGrpcAddr" -ForegroundColor Gray
Write-Host ""

# Check if KMS server is running
Write-Host "1. Checking KMS gRPC server..." -ForegroundColor Yellow
try {
    $kmsTest = Test-NetConnection -ComputerName ($KMSGrpcAddr -replace ":.*", "") -Port ($KMSGrpcAddr -replace ".*:", "") -InformationLevel Quiet -WarningAction SilentlyContinue
    if (-not $kmsTest) {
        Write-Host "   [WARNING] Cannot connect to KMS server at $KMSGrpcAddr" -ForegroundColor Yellow
        Write-Host "   Please ensure KMS server is running: .\start-kms.ps1" -ForegroundColor Yellow
        Write-Host "   Continuing anyway..." -ForegroundColor Gray
    } else {
        Write-Host "   [OK] KMS server is reachable" -ForegroundColor Green
    }
} catch {
    Write-Host "   [WARNING] Could not verify KMS server connection" -ForegroundColor Yellow
}

# Create config.yaml for ETL worker
Write-Host "`n2. Creating config.yaml for ETL worker..." -ForegroundColor Yellow
$configContent = @"
kms:
  addr: "$KMSGrpcAddr"

auth:
  bearerToken: ""

sourceDB:
  driver: "$SourceDBDriver"
  dsn: "$SourceDBDSN"

destDB:
  driver: "$TargetDBDriver"
  dsn: "$TargetDBDSN"
"@

$configContent | Out-File -FilePath "config.yaml" -Encoding UTF8 -NoNewline
Write-Host "   [OK] Created config.yaml" -ForegroundColor Green

# Setup test databases
if (-not $SkipSetup) {
    Write-Host "`n3. Setting up test databases..." -ForegroundColor Yellow
    
    $setupScript = "scripts\setup-test-db-$SourceDBDriver.sql"
    if (Test-Path $setupScript) {
        Write-Host "   Running setup script: $setupScript" -ForegroundColor Gray
        # Note: User needs to run SQL scripts manually or via sqlcmd
        Write-Host "   [INFO] Please run the SQL setup scripts manually:" -ForegroundColor Yellow
        Write-Host "   - scripts\setup-test-db-$SourceDBDriver.sql (for source DB)" -ForegroundColor White
        Write-Host "   - scripts\setup-test-db-$TargetDBDriver.sql (for target DB)" -ForegroundColor White
    } else {
        Write-Host "   [INFO] Setup scripts not found. Creating them..." -ForegroundColor Yellow
        # Create setup scripts
        if ($SourceDBDriver -eq "sqlserver") {
            $sourceSetup = @"
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
"@
            $sourceSetup | Out-File -FilePath "scripts\setup-test-db-sqlserver-source.sql" -Encoding UTF8
            
            $targetSetup = @"
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
"@
            $targetSetup | Out-File -FilePath "scripts\setup-test-db-sqlserver-target.sql" -Encoding UTF8
        } else {
            $sourceSetup = @"
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
"@
            $sourceSetup | Out-File -FilePath "scripts\setup-test-db-mysql-source.sql" -Encoding UTF8
            
            $targetSetup = @"
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
"@
            $targetSetup | Out-File -FilePath "scripts\setup-test-db-mysql-target.sql" -Encoding UTF8
        }
        Write-Host "   [OK] Created setup scripts in scripts\ directory" -ForegroundColor Green
        Write-Host "   [INFO] Please run the SQL setup scripts manually:" -ForegroundColor Yellow
        Write-Host "   - scripts\setup-test-db-$SourceDBDriver-source.sql" -ForegroundColor White
        Write-Host "   - scripts\setup-test-db-$TargetDBDriver-target.sql" -ForegroundColor White
    }
} else {
    Write-Host "`n3. Skipping database setup (using -SkipSetup)" -ForegroundColor Gray
}

# Run ETL worker
Write-Host "`n4. Running ETL worker..." -ForegroundColor Yellow
Write-Host "   Command: go run ./cmd/etl-worker" -ForegroundColor Gray

$etlStartTime = Get-Date
$etlOutput = & go run ./cmd/etl-worker 2>&1
$etlEndTime = Get-Date
$etlDuration = ($etlEndTime - $etlStartTime).TotalSeconds

if ($LASTEXITCODE -eq 0) {
    Write-Host "   [OK] ETL worker completed successfully" -ForegroundColor Green
    Write-Host "   Duration: $([math]::Round($etlDuration, 2)) seconds" -ForegroundColor Gray
    if ($etlOutput) {
        Write-Host "   Output:" -ForegroundColor Gray
        $etlOutput | ForEach-Object { Write-Host "     $_" -ForegroundColor Gray }
    }
} else {
    Write-Host "   [ERROR] ETL worker failed" -ForegroundColor Red
    if ($etlOutput) {
        Write-Host "   Output:" -ForegroundColor Red
        $etlOutput | ForEach-Object { Write-Host "     $_" -ForegroundColor Red }
    }
    exit 1
}

# Verify results
Write-Host "`n5. Verifying encrypted data in target database..." -ForegroundColor Yellow

Write-Host "   Running verification script..." -ForegroundColor Gray
$verifyOutput = & powershell -ExecutionPolicy Bypass -File "scripts\verify-etl-results.ps1" -Driver $TargetDBDriver -DSN $TargetDBDSN -KMSAddr $KMSGrpcAddr 2>&1

if ($LASTEXITCODE -eq 0) {
    Write-Host "   [OK] Verification successful" -ForegroundColor Green
    $verifyOutput | ForEach-Object { Write-Host "     $_" -ForegroundColor Gray }
} else {
    Write-Host "   [ERROR] Verification failed" -ForegroundColor Red
    $verifyOutput | ForEach-Object { Write-Host "     $_" -ForegroundColor Red }
    exit 1
}

# Cleanup
if (-not $SkipCleanup) {
    Write-Host "`n6. Cleanup..." -ForegroundColor Yellow
    # No temporary files to clean up (verification script handles its own cleanup)
    Write-Host "   [OK] Cleanup complete" -ForegroundColor Green
} else {
    Write-Host "`n6. Skipping cleanup (using -SkipCleanup)" -ForegroundColor Gray
}

Write-Host "`n=== ETL Worker Test Completed Successfully ===" -ForegroundColor Green
Write-Host "`nSummary:" -ForegroundColor Cyan
Write-Host "  - Source DB: $SourceDBDriver" -ForegroundColor White
Write-Host "  - Target DB: $TargetDBDriver" -ForegroundColor White
Write-Host "  - ETL Duration: $([math]::Round($etlDuration, 2)) seconds" -ForegroundColor White
Write-Host "  - All records encrypted and transferred successfully" -ForegroundColor White
Write-Host "`nNext Steps:" -ForegroundColor Cyan
Write-Host "1. Review encrypted data in target database" -ForegroundColor White
Write-Host "2. Integrate with SSIS - See docs/SSIS_INTEGRATION.md" -ForegroundColor White
Write-Host "3. Setup HSM (Production) - See docs/HSM_INTEGRATION.md" -ForegroundColor White

