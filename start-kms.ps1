# KMS System Startup Script (Auto-Debug Mode)
# Usage: .\start-kms.ps1

param(
    [string]$MasterKeyPath = "master.key",
    [string]$GrpcAddr = ":50051",
    [string]$HttpAddr = ":8080",
    [switch]$SkipHttpServer
)

Write-Host "=== Starting KMS System ===" -ForegroundColor Cyan

# --- 1. SETUP PORTS ---
$grpcPort = if ($GrpcAddr -match ":(\d+)") { $Matches[1] } else { "50051" }
$httpPort = if ($HttpAddr -match ":(\d+)") { $Matches[1] } else { "8080" }

# Kill anything currently using these ports (Cleanup old messes)
Get-NetTCPConnection -LocalPort $grpcPort -ErrorAction SilentlyContinue | ForEach-Object { 
    Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue 
}
if (-not $SkipHttpServer) {
    Get-NetTCPConnection -LocalPort $httpPort -ErrorAction SilentlyContinue | ForEach-Object { 
        Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue 
    }
}

# --- 2. GENERATE KEY IF MISSING ---
if (-not (Test-Path $MasterKeyPath)) {
    Write-Host "Generating master.key..." -ForegroundColor Yellow
    $key = go run -c "package main; import (\"crypto/rand\"; \"encoding/hex\"; \"os\"); func main() { b := make([]byte, 32); rand.Read(b); os.Stdout.WriteString(hex.EncodeToString(b)) }"
    $key | Out-File -FilePath $MasterKeyPath -Encoding ASCII -NoNewline
}

# --- 3. START GRPC SERVER ---
Write-Host "Starting gRPC service ($GrpcAddr)..." -ForegroundColor Yellow

# Pass all variables to the background job
$grpcJob = Start-Job -ScriptBlock {
    param($MasterKeyPath, $GrpcAddr)
    $env:KMS_MASTER_KEY_PATH = $MasterKeyPath
    $env:KMS_GRPC_ADDR = $GrpcAddr
    
    # Import HSM variables from main session
    $env:KMS_PKCS11_LIB = $using:env:KMS_PKCS11_LIB
    $env:KMS_PKCS11_SLOT = $using:env:KMS_PKCS11_SLOT
    $env:KMS_PKCS11_KEY_LABEL = $using:env:KMS_PKCS11_KEY_LABEL
    $env:KMS_JWT_SECRET = $using:env:KMS_JWT_SECRET

    Set-Location $using:PWD
    
    # Run server and capture panic output
    go run -tags pkcs11 ./cmd/kms-server 2>&1
} -ArgumentList $MasterKeyPath, $GrpcAddr

# --- 4. START HTTP SERVER ---
if (-not $SkipHttpServer) {
    Write-Host "Starting HTTP service ($HttpAddr)..." -ForegroundColor Yellow
    $grpcConnectAddr = if ($GrpcAddr -match "^:") { "127.0.0.1$GrpcAddr" } else { $GrpcAddr }
    
    $httpJob = Start-Job -ScriptBlock {
        param($GrpcConnectAddr, $HttpAddr)
        $env:KMS_GRPC_ADDR = $GrpcConnectAddr
        $env:KMS_HTTP_ADDR = $HttpAddr
        Set-Location $using:PWD
        go run ./cmd/kms-http-server 2>&1
    } -ArgumentList $grpcConnectAddr, $HttpAddr
}

# --- 5. WAIT AND MONITOR FOR CRASHES ---
Write-Host "`n[OK] System is running." -ForegroundColor Green
Write-Host "Run your test script now: .\test-system.ps1" -ForegroundColor Cyan
Write-Host "Monitoring for crashes... (Press Ctrl+C to stop)`n" -ForegroundColor Gray

try {
    while ($true) {
        # Check gRPC Job
        $gState = Get-Job -Id $grpcJob.Id
        if ($gState.State -ne 'Running') {
            Write-Host "`n[CRASH DETECTED] gRPC Server died!" -ForegroundColor Red
            Write-Host "--- ERROR LOG START ---" -ForegroundColor Yellow
            Receive-Job -Id $grpcJob.Id -Keep
            Write-Host "--- ERROR LOG END ---" -ForegroundColor Yellow
            break
        }

        # Check HTTP Job
        if (-not $SkipHttpServer) {
            $hState = Get-Job -Id $httpJob.Id
            if ($hState.State -ne 'Running') {
                Write-Host "`n[CRASH DETECTED] HTTP Server died!" -ForegroundColor Red
                Write-Host "--- ERROR LOG START ---" -ForegroundColor Yellow
                Receive-Job -Id $httpJob.Id -Keep
                Write-Host "--- ERROR LOG END ---" -ForegroundColor Yellow
                break
            }
        }
        Start-Sleep -Milliseconds 500
    }
}
finally {
    Write-Host "`nStopping all services..." -ForegroundColor Yellow
    Stop-Job $grpcJob -ErrorAction SilentlyContinue
    if ($httpJob) { Stop-Job $httpJob -ErrorAction SilentlyContinue }
    Remove-Job * -ErrorAction SilentlyContinue
}