# Build script for KMS project executables

Write-Host "Building KMS executables..." -ForegroundColor Green

# Check if Go is installed
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

Write-Host "Go version: $(go version)" -ForegroundColor Cyan

# Build etl-worker.exe
Write-Host "`nBuilding etl-worker.exe..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o etl-worker.exe ./cmd/etl-worker
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ etl-worker.exe built successfully" -ForegroundColor Green
} else {
    Write-Host "✗ Failed to build etl-worker.exe" -ForegroundColor Red
    exit 1
}

# Build kms-server.exe
Write-Host "`nBuilding kms-server.exe..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o kms-server.exe ./cmd/kms-server
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ kms-server.exe built successfully" -ForegroundColor Green
} else {
    Write-Host "✗ Failed to build kms-server.exe" -ForegroundColor Red
}

# Build kms-http-server.exe
Write-Host "`nBuilding kms-http-server.exe..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o kms-http-server.exe ./cmd/kms-http-server
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ kms-http-server.exe built successfully" -ForegroundColor Green
} else {
    Write-Host "✗ Failed to build kms-http-server.exe" -ForegroundColor Red
}

Write-Host "`n=== Build Complete ===" -ForegroundColor Green
Write-Host "Executables:" -ForegroundColor Cyan
Get-ChildItem -Filter "*.exe" | ForEach-Object {
    $size = [math]::Round($_.Length / 1MB, 2)
    Write-Host "  - $($_.Name) ($size MB)" -ForegroundColor White
}
