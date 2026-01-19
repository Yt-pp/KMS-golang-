# KMS System Test Script (Authenticated)
# Usage: .\test-system.ps1

param(
    [string]$KMSHttpUrl = "http://localhost:8080",
    [string]$GrpcAddr = "127.0.0.1:50051" # Needed for login
)

Write-Host "=== KMS System Test ===" -ForegroundColor Cyan
Write-Host "KMS URL: $KMSHttpUrl`n" -ForegroundColor Gray

# --- STEP 0: LOGIN & GET TOKEN ---
Write-Host "0. Logging in to get Token..." -ForegroundColor Yellow

# We use the 'grpcurl' trick or the client tool to get a token quickly.
# Or, if your HTTP server exposes a login endpoint, use that.
# Assuming your HTTP server just proxies encryption, we need a valid token.
# Let's generate one using the SECRET we know is set in the environment.

# NOTE: In a real scenario, you'd call a Login API.
# Here, we will assume you have the token in $env:KMS_BEARER_TOKEN or we ask for it.

$token = $env:KMS_BEARER_TOKEN

if (-not $token) {
    # Try to auto-login using the Go client tool if available
    Write-Host "   No token in environment. Attempting to login via test-client..." -ForegroundColor Gray
    try {
        # This runs the login command and parses the output for the token
        $loginOutput = go run ./cmd/test-client login 2>&1
        if ($loginOutput -match "Token: (ey[a-zA-Z0-9\._\-]+)") {
            $token = $matches[1]
            Write-Host "   [OK] Auto-login successful!" -ForegroundColor Green
        } else {
            throw "Could not parse token from login output."
        }
    } catch {
        Write-Host "   [ERROR] Login failed. Please set `$env:KMS_BEARER_TOKEN manually." -ForegroundColor Red
        Write-Host "   Run: go run ./cmd/test-client login" -ForegroundColor Yellow
        exit 1
    }
}

# Create the Authorization Header
$headers = @{
    "Authorization" = "Bearer $token"
}

# ---------------------------------

# Test 1: Health Check (Usually public, but good to test)
Write-Host "`n1. Testing health check..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "$KMSHttpUrl/health" -ErrorAction Stop
    Write-Host "   [OK] Health check passed: $($health.status)" -ForegroundColor Green
}
catch {
    Write-Host "   [ERROR] Health check failed: $_" -ForegroundColor Red
    exit 1
}

# Test 2: Single Encryption
Write-Host "`n2. Testing single encryption..." -ForegroundColor Yellow
try {
    $body = @{ plaintext = "4111111111111111" } | ConvertTo-Json
    $encrypt = Invoke-RestMethod -Uri "$KMSHttpUrl/api/v1/encrypt" `
        -Method Post -Body $body -ContentType "application/json" -Headers $headers -ErrorAction Stop
    Write-Host "   [OK] Single encryption successful" -ForegroundColor Green
    Write-Host "   Ciphertext: $($encrypt.ciphertext.Substring(0, [Math]::Min(20, $encrypt.ciphertext.Length)))..." -ForegroundColor Gray
    Write-Host "   Nonce: $($encrypt.nonce.Substring(0, [Math]::Min(20, $encrypt.nonce.Length)))..." -ForegroundColor Gray
}
catch {
    Write-Host "   [ERROR] Single encryption failed: $_" -ForegroundColor Red
    exit 1
}

# Test 3: Batch Encryption
Write-Host "`n3. Testing batch encryption (10 items)..." -ForegroundColor Yellow
try {
    $items = @()
    for ($i = 1; $i -le 10; $i++) {
        $items += @{ plaintext = "4111111111111111" }
    }
    $body = @{ items = $items } | ConvertTo-Json
    
    $startTime = Get-Date
    $batch = Invoke-RestMethod -Uri "$KMSHttpUrl/api/v1/encrypt/batch" `
        -Method Post -Body $body -ContentType "application/json" -Headers $headers -ErrorAction Stop
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    
    Write-Host "   [OK] Batch encryption successful" -ForegroundColor Green
    Write-Host "   Processed 10 items in $([math]::Round($duration, 3)) seconds" -ForegroundColor Gray
}
catch {
    Write-Host "   [ERROR] Batch encryption failed: $_" -ForegroundColor Red
    exit 1
}

# Test 4: Decryption
Write-Host "`n4. Testing decryption..." -ForegroundColor Yellow
try {
    $decryptBody = @{
        ciphertext = $encrypt.ciphertext
        nonce = $encrypt.nonce
    } | ConvertTo-Json
    
    $decrypt = Invoke-RestMethod -Uri "$KMSHttpUrl/api/v1/decrypt" `
        -Method Post -Body $decryptBody -ContentType "application/json" -Headers $headers -ErrorAction Stop
    
    if ($decrypt.plaintext -eq "4111111111111111") {
        Write-Host "   [OK] Decryption successful, result correct" -ForegroundColor Green
    }
    else {
        Write-Host "   [ERROR] Decryption result incorrect" -ForegroundColor Red
        exit 1
    }
}
catch {
    Write-Host "   [ERROR] Decryption failed: $_" -ForegroundColor Red
    exit 1
}

# Test 5: Performance Test (100 items)
Write-Host "`n5. Performance test (100 items)..." -ForegroundColor Yellow
try {
    $items = @()
    for ($i = 1; $i -le 100; $i++) {
        $items += @{ plaintext = "4111111111111111" }
    }
    $body = @{ items = $items } | ConvertTo-Json
    
    $startTime = Get-Date
    $batch = Invoke-RestMethod -Uri "$KMSHttpUrl/api/v1/encrypt/batch" `
        -Method Post -Body $body -ContentType "application/json" -Headers $headers -ErrorAction Stop
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    
    Write-Host "   [OK] Performance test completed" -ForegroundColor Green
    Write-Host "   Throughput: $([math]::Round(100 / $duration, 2)) req/s" -ForegroundColor Gray
}
catch {
    Write-Host "   [ERROR] Performance test failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== All Tests Passed ===" -ForegroundColor Green