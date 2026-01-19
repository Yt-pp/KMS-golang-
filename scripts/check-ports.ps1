# Check if KMS ports are available
# Usage: .\scripts\check-ports.ps1

Write-Host "=== Checking KMS Ports ===" -ForegroundColor Cyan

# Check port 50051 (gRPC)
$grpcPort = 50051
$grpcProcess = Get-NetTCPConnection -LocalPort $grpcPort -ErrorAction SilentlyContinue
if ($grpcProcess) {
    $pid = $grpcProcess.OwningProcess
    $process = Get-Process -Id $pid -ErrorAction SilentlyContinue
    Write-Host "`n[WARNING] Port $grpcPort (gRPC) is in use:" -ForegroundColor Yellow
    Write-Host "  Process ID: $pid" -ForegroundColor Gray
    if ($process) {
        Write-Host "  Process Name: $($process.ProcessName)" -ForegroundColor Gray
        Write-Host "  Command: $($process.Path)" -ForegroundColor Gray
    }
    Write-Host "`nTo stop the process:" -ForegroundColor Cyan
    Write-Host "  Stop-Process -Id $pid -Force" -ForegroundColor White
}
else {
    Write-Host "`n[OK] Port $grpcPort (gRPC) is available" -ForegroundColor Green
}

# Check port 8080 (HTTP)
$httpPort = 8080
$httpProcess = Get-NetTCPConnection -LocalPort $httpPort -ErrorAction SilentlyContinue
if ($httpProcess) {
    $pid = $httpProcess.OwningProcess
    $process = Get-Process -Id $pid -ErrorAction SilentlyContinue
    Write-Host "`n[WARNING] Port $httpPort (HTTP) is in use:" -ForegroundColor Yellow
    Write-Host "  Process ID: $pid" -ForegroundColor Gray
    if ($process) {
        Write-Host "  Process Name: $($process.ProcessName)" -ForegroundColor Gray
        Write-Host "  Command: $($process.Path)" -ForegroundColor Gray
    }
    Write-Host "`nTo stop the process:" -ForegroundColor Cyan
    Write-Host "  Stop-Process -Id $pid -Force" -ForegroundColor White
}
else {
    Write-Host "`n[OK] Port $httpPort (HTTP) is available" -ForegroundColor Green
}

Write-Host "`n=== Port Check Complete ===" -ForegroundColor Cyan

