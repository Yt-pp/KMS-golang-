# Verify ETL Worker Results - Decrypt and Verify Data
# Usage: .\scripts\verify-etl-results.ps1 [-Driver sqlserver|mysql] [-DSN connection_string] [-KMSAddr 127.0.0.1:50051]

param(
    [string]$Driver = "mysql",
    [string]$DSN = "",
    [string]$KMSAddr = "127.0.0.1:50051"
)

Write-Host "=== ETL Results Verification ===" -ForegroundColor Cyan
Write-Host ""

if ([string]::IsNullOrEmpty($DSN)) {
    if ($Driver -eq "sqlserver") {
        $DSN = "sqlserver://sa:YourPassword123@localhost:1433?database=kms_test_target&encrypt=disable"
    } else {
        $DSN = "root:1234@tcp(localhost:3306)/kms_test_target"
    }
}

Write-Host "Configuration:" -ForegroundColor Yellow
Write-Host "  Driver: $Driver" -ForegroundColor Gray
Write-Host "  DSN: $DSN" -ForegroundColor Gray
Write-Host "  KMS Addr: $KMSAddr" -ForegroundColor Gray
Write-Host ""

# Create temporary Go verification script
$verifyScript = @"
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"
    
    kmsproto "kms/proto"
    
    _ "github.com/go-sql-driver/mysql"
    _ "github.com/microsoft/go-mssqldb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    if len(os.Args) < 4 {
        log.Fatal("Usage: go run verify-etl.go <driver> <dsn> <kms_addr>")
    }
    
    driver := os.Args[1]
    dsn := os.Args[2]
    kmsAddr := os.Args[3]
    
    db, err := sql.Open(driver, dsn)
    if err != nil {
        log.Fatalf("Failed to open DB: %v", err)
    }
    defer db.Close()
    
    // Test connection
    if err := db.Ping(); err != nil {
        log.Fatalf("Failed to ping DB: %v", err)
    }
    
    // Connect to KMS
    conn, err := grpc.Dial(kmsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("Failed to dial KMS: %v", err)
    }
    defer conn.Close()
    
    kmsClient := kmsproto.NewKMSClient(conn)
    
    // Query encrypted records
    rows, err := db.Query("SELECT source_id, pan_ciphertext, pan_nonce, cvv_ciphertext, cvv_nonce, other_data FROM encrypted_cards ORDER BY source_id")
    if err != nil {
        log.Fatalf("Failed to query: %v", err)
    }
    defer rows.Close()
    
    count := 0
    successCount := 0
    
    fmt.Println("Decrypting and verifying records...")
    fmt.Println("")
    
    for rows.Next() {
        count++
        var sourceID int
        var panCiphertext, panNonce, cvvCiphertext, cvvNonce []byte
        var otherData sql.NullString
        
        if err := rows.Scan(&sourceID, &panCiphertext, &panNonce, &cvvCiphertext, &cvvNonce, &otherData); err != nil {
            fmt.Printf("Record %d: Failed to scan row: %v\n", sourceID, err)
            continue
        }
        
        ctx := context.Background()
        
        // Decrypt PAN
        panResp, err := kmsClient.Decrypt(ctx, &kmsproto.DecryptRequest{
            Ciphertext: panCiphertext,
            Nonce: panNonce,
        })
        if err != nil {
            fmt.Printf("Record %d: Failed to decrypt PAN: %v\n", sourceID, err)
            continue
        }
        
        // Decrypt CVV
        cvvResp, err := kmsClient.Decrypt(ctx, &kmsproto.DecryptRequest{
            Ciphertext: cvvCiphertext,
            Nonce: cvvNonce,
        })
        if err != nil {
            fmt.Printf("Record %d: Failed to decrypt CVV: %v\n", sourceID, err)
            continue
        }
        
        pan := string(panResp.Plaintext)
        cvv := string(cvvResp.Plaintext)
        
        fmt.Printf("Record %d: PAN=%s, CVV=%s, Other=%s\n", sourceID, pan, cvv, otherData.String)
        successCount++
    }
    
    fmt.Println("")
    fmt.Printf("Total records: %d\n", count)
    fmt.Printf("Successfully decrypted: %d\n", successCount)
    
    if count == 0 {
        log.Fatal("No records found in target database!")
    }
    
    if successCount != count {
        log.Fatalf("Failed to decrypt %d records", count-successCount)
    }
    
    fmt.Println("All records verified successfully!")
}
"@

$verifyScript | Out-File -FilePath "verify-etl-temp.go" -Encoding UTF8

Write-Host "Running verification..." -ForegroundColor Yellow
$verifyOutput = & go run verify-etl-temp.go $Driver $DSN $KMSAddr 2>&1

if ($LASTEXITCODE -eq 0) {
    Write-Host "`n[OK] Verification successful" -ForegroundColor Green
    $verifyOutput | ForEach-Object { Write-Host $_ -ForegroundColor Gray }
} else {
    Write-Host "`n[ERROR] Verification failed" -ForegroundColor Red
    $verifyOutput | ForEach-Object { Write-Host $_ -ForegroundColor Red }
    Remove-Item -Path "verify-etl-temp.go" -ErrorAction SilentlyContinue
    exit 1
}

# Cleanup
Remove-Item -Path "verify-etl-temp.go" -ErrorAction SilentlyContinue

Write-Host "`n=== Verification Complete ===" -ForegroundColor Green

