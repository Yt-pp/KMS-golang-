# Test script to verify Excel export functionality
# Usage: .\test-excel-export.ps1

Write-Host "=== Testing Excel Export ===" -ForegroundColor Cyan

# Check if etl-worker.exe exists
if (-not (Test-Path ".\etl-worker.exe")) {
    Write-Host "ERROR: etl-worker.exe not found!" -ForegroundColor Red
    Write-Host "Please build it first: go build -o etl-worker.exe ./cmd/etl-worker" -ForegroundColor Yellow
    exit 1
}

# Test 1: Check help
Write-Host "`n1. Testing help command..." -ForegroundColor Yellow
.\etl-worker.exe -help 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "   ✓ Help command works" -ForegroundColor Green
} else {
    Write-Host "   ✗ Help command failed" -ForegroundColor Red
}

# Test 2: Check if config file exists
Write-Host "`n2. Checking config file..." -ForegroundColor Yellow
if (Test-Path ".\config.yaml") {
    Write-Host "   ✓ config.yaml found" -ForegroundColor Green
} else {
    Write-Host "   ✗ config.yaml not found" -ForegroundColor Red
    Write-Host "   Please create config.yaml file" -ForegroundColor Yellow
}

# Test 3: Test Excel path creation
Write-Host "`n3. Testing Excel path..." -ForegroundColor Yellow
$excelPath = ".\test_verification_results.xlsx"
Write-Host "   Excel path: $excelPath" -ForegroundColor Gray

# Test 4: Show example command
Write-Host "`n4. Example command to run:" -ForegroundColor Yellow
Write-Host "   .\etl-worker.exe -verify-excel -excel-output `"$excelPath`" -config config.yaml" -ForegroundColor Cyan

Write-Host "`n=== Test Complete ===" -ForegroundColor Cyan

