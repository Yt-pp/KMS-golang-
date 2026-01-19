# KMS System Startup Script
# Usage: .\start-kms-en.ps1

param(
    [string]$MasterKeyPath = "master.key",
    [string]$GrpcAddr = ":50051",
    [string]$HttpAddr = ":8080",
    [switch]$SkipHttpServer
)

Write-Host "=== Starting KMS System ===" -ForegroundColor Cyan

# Check if master.key exists
if (-not (Test-Path $MasterKeyPath)) {
    Write-Host "`nWarning: master.key not found, generating..." -ForegroundColor Yellow
    
    # Try OpenSSL first (if available)
    $opensslAvailable = Get-Command openssl -ErrorAction SilentlyContinue
    if ($opensslAvailable) {
        openssl rand -hex 32 | Out-File -FilePath $MasterKeyPath -Encoding ASCII -NoNewline
        Write-Host "[OK] Generated master.key using OpenSSL" -ForegroundColor Green
    }
    else {
        # Use Go to generate
        Write-Host "OpenSSL not available, using Go to generate key..." -ForegroundColor Yellow
        $key = go run -c "package main; import (\"crypto/rand\"; \"encoding/hex\"; \"os\"); func main() { b := make([]byte, 32); rand.Read(b); os.Stdout.WriteString(hex.EncodeToString(b)) }"
        $key | Out-File -FilePath $MasterKeyPath -Encoding ASCII -NoNewline
        Write-Host "[OK] Generated master.key using Go" -ForegroundColor Green
    }
}

# Set environment variables
$env:KMS_MASTER_KEY_PATH = $MasterKeyPath
$env:KMS_GRPC_ADDR = $GrpcAddr

Write-Host "`nEnvironment Variables:" -ForegroundColor Yellow
Write-Host "  KMS_MASTER_KEY_PATH = $env:KMS_MASTER_KEY_PATH" -ForegroundColor Gray
Write-Host "  KMS_GRPC_ADDR = $env:KMS_GRPC_ADDR" -ForegroundColor Gray

# Start gRPC service
Write-Host "`nStarting gRPC service..." -ForegroundColor Yellow
Write-Host "  Address: $GrpcAddr" -ForegroundColor Gray

$grpcJob = Start-Job -ScriptBlock {
    param($MasterKeyPath, $GrpcAddr)
    $env:KMS_MASTER_KEY_PATH = $MasterKeyPath
    $env:KMS_GRPC_ADDR = $GrpcAddr
    Set-Location $using:PWD
    go run ./cmd/kms-server 2>&1
} -ArgumentList $MasterKeyPath, $GrpcAddr

Start-Sleep -Seconds 2

# Check if gRPC service started successfully
$grpcOutput = Receive-Job $grpcJob
if ($grpcOutput -match "listening") {
    Write-Host "  [OK] gRPC service started" -ForegroundColor Green
}
else {
    Write-Host "  [ERROR] Failed to start gRPC service" -ForegroundColor Red
    Write-Host "  Output: $grpcOutput" -ForegroundColor Yellow
    Stop-Job $grpcJob
    Remove-Job $grpcJob
    exit 1
}

# Start HTTP REST API service (if not skipped)
if (-not $SkipHttpServer) {
    Write-Host "`nStarting HTTP REST API service..." -ForegroundColor Yellow
    
    $env:KMS_GRPC_ADDR = "127.0.0.1:50051"
    $env:KMS_HTTP_ADDR = $HttpAddr
    
    Write-Host "  gRPC Backend: 127.0.0.1:50051" -ForegroundColor Gray
    Write-Host "  HTTP Address: $HttpAddr" -ForegroundColor Gray
    
    $httpJob = Start-Job -ScriptBlock {
        param($GrpcAddr, $HttpAddr)
        $env:KMS_GRPC_ADDR = $GrpcAddr
        $env:KMS_HTTP_ADDR = $HttpAddr
        Set-Location $using:PWD
        go run ./cmd/kms-http-server 2>&1
    } -ArgumentList "127.0.0.1:50051", $HttpAddr
    
    Start-Sleep -Seconds 2
    
    # Check if HTTP service started successfully
    $httpOutput = Receive-Job $httpJob
    if ($httpOutput -match "listening") {
        Write-Host "  [OK] HTTP REST API service started" -ForegroundColor Green
    }
    else {
        Write-Host "  [ERROR] Failed to start HTTP REST API service" -ForegroundColor Red
        Write-Host "  Output: $httpOutput" -ForegroundColor Yellow
    }
    
    Write-Host "`nService Status:" -ForegroundColor Cyan
    Write-Host "  gRPC: $GrpcAddr" -ForegroundColor White
    Write-Host "  HTTP: $HttpAddr" -ForegroundColor White
    Write-Host "`nTest Command:" -ForegroundColor Cyan
    Write-Host "  .\test-system.ps1" -ForegroundColor White
    Write-Host "`nPress Ctrl+C to stop services" -ForegroundColor Yellow
    
    # Wait for user interrupt
    try {
        Wait-Job $grpcJob, $httpJob | Out-Null
    }
    catch {
        Write-Host "`nStopping services..." -ForegroundColor Yellow
        Stop-Job $grpcJob, $httpJob
        Remove-Job $grpcJob, $httpJob
    }
}
else {
    Write-Host "`nService Status:" -ForegroundColor Cyan
    Write-Host "  gRPC: $GrpcAddr" -ForegroundColor White
    Write-Host "  HTTP: Not started (using -SkipHttpServer)" -ForegroundColor Gray
    Write-Host "`nPress Ctrl+C to stop service" -ForegroundColor Yellow
    
    try {
        Wait-Job $grpcJob | Out-Null
    }
    catch {
        Write-Host "`nStopping service..." -ForegroundColor Yellow
        Stop-Job $grpcJob
        Remove-Job $grpcJob
    }
}

