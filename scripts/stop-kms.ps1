# Stop KMS Services
# Usage: .\scripts\stop-kms.ps1

Write-Host "=== Stopping KMS Services ===" -ForegroundColor Cyan

# Stop processes on port 50051 (gRPC)
$grpcConnections = Get-NetTCPConnection -LocalPort 50051 -ErrorAction SilentlyContinue
if ($grpcConnections) {
    foreach ($conn in $grpcConnections) {
        $pid = $conn.OwningProcess
        $process = Get-Process -Id $pid -ErrorAction SilentlyContinue
        if ($process) {
            Write-Host "Stopping gRPC service (PID: $pid, Name: $($process.ProcessName))..." -ForegroundColor Yellow
            Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
            Write-Host "  [OK] Stopped" -ForegroundColor Green
        }
    }
}
else {
    Write-Host "[OK] No process found on port 50051 (gRPC)" -ForegroundColor Green
}

# Stop processes on port 8080 (HTTP)
$httpConnections = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($httpConnections) {
    foreach ($conn in $httpConnections) {
        $pid = $conn.OwningProcess
        $process = Get-Process -Id $pid -ErrorAction SilentlyContinue
        if ($process) {
            Write-Host "Stopping HTTP service (PID: $pid, Name: $($process.ProcessName))..." -ForegroundColor Yellow
            Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
            Write-Host "  [OK] Stopped" -ForegroundColor Green
        }
    }
}
else {
    Write-Host "[OK] No process found on port 8080 (HTTP)" -ForegroundColor Green
}

Write-Host "`n=== Done ===" -ForegroundColor Cyan

