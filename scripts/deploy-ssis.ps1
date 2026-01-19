# SSIS KMS Integration Deployment Script
# This script helps deploy and configure KMS for SSIS integration

param(
    [string]$KMSHttpUrl = "http://localhost:8080",
    [string]$KMSToken = "",
    [int]$BatchSize = 100,
    [switch]$TestConnection
)

Write-Host "=== KMS SSIS Integration Deployment ===" -ForegroundColor Cyan

# Test KMS connection
if ($TestConnection) {
    Write-Host "`nTesting KMS connection..." -ForegroundColor Yellow
    try {
        $healthUrl = "$KMSHttpUrl/health"
        $response = Invoke-RestMethod -Uri $healthUrl -Method Get -ErrorAction Stop
        Write-Host "✓ KMS is reachable at $KMSHttpUrl" -ForegroundColor Green
        Write-Host "  Status: $($response.status)" -ForegroundColor Green
    }
    catch {
        Write-Host "✗ Failed to connect to KMS: $_" -ForegroundColor Red
        Write-Host "  Please ensure KMS HTTP server is running" -ForegroundColor Yellow
        exit 1
    }
}

# Create SSIS configuration file
Write-Host "`nCreating SSIS configuration..." -ForegroundColor Yellow

$configContent = @"
# KMS SSIS Configuration
# Copy these values to your SSIS Script Component

KMS_API_URL=$KMSHttpUrl/api/v1/encrypt/batch
KMS_BATCH_SIZE=$BatchSize
KMS_BEARER_TOKEN=$KMSToken
KMS_USE_BATCH_MODE=true
"@

$configPath = "ssis-kms-config.txt"
$configContent | Out-File -FilePath $configPath -Encoding UTF8
Write-Host "✓ Configuration saved to: $configPath" -ForegroundColor Green

# Create PowerShell script for SSIS variables
Write-Host "`nCreating SSIS variable script..." -ForegroundColor Yellow

$varScript = @"
# SSIS Package Variables Setup Script
# Run this in SSIS Package to set variables

# KMS API Configuration
`$KMSApiUrl = "$KMSHttpUrl/api/v1/encrypt/batch"
`$KMSBatchSize = $BatchSize
`$KMSBearerToken = "$KMSToken"
`$KMSUseBatchMode = `$true

Write-Host "SSIS Variables configured:" -ForegroundColor Cyan
Write-Host "  KMS_API_URL = `$KMSApiUrl"
Write-Host "  KMS_BATCH_SIZE = `$KMSBatchSize"
Write-Host "  KMS_BEARER_TOKEN = [hidden]"
Write-Host "  KMS_USE_BATCH_MODE = `$KMSUseBatchMode"
"@

$varScriptPath = "ssis-variables.ps1"
$varScript | Out-File -FilePath $varScriptPath -Encoding UTF8
Write-Host "✓ Variable script saved to: $varScriptPath" -ForegroundColor Green

# Test encryption endpoint
Write-Host "`nTesting encryption endpoint..." -ForegroundColor Yellow
try {
    $testPayload = @{
        items = @(
            @{ plaintext = "4111111111111111" }
        )
    } | ConvertTo-Json

    $headers = @{
        "Content-Type" = "application/json"
    }
    
    if ($KMSToken) {
        $headers["Authorization"] = "Bearer $KMSToken"
    }

    $encryptUrl = "$KMSHttpUrl/api/v1/encrypt/batch"
    $response = Invoke-RestMethod -Uri $encryptUrl -Method Post -Body $testPayload -Headers $headers -ErrorAction Stop
    
    Write-Host "✓ Encryption test successful!" -ForegroundColor Green
    Write-Host "  Ciphertext length: $($response.results[0].ciphertext.Length)" -ForegroundColor Gray
    Write-Host "  Nonce length: $($response.results[0].nonce.Length)" -ForegroundColor Gray
}
catch {
    Write-Host "✗ Encryption test failed: $_" -ForegroundColor Red
    Write-Host "  Check KMS server logs for details" -ForegroundColor Yellow
}

Write-Host "`n=== Deployment Complete ===" -ForegroundColor Cyan
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Copy the C# code from docs/SSIS_SCRIPT_EXAMPLE.cs" -ForegroundColor White
Write-Host "2. Update the Script Component in your SSIS package" -ForegroundColor White
Write-Host "3. Set the variables using the configuration above" -ForegroundColor White
Write-Host "4. Test with a small dataset first" -ForegroundColor White

