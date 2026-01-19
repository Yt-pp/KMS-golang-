# 完整系統運行指南

本指南將帶您從零開始運行整個 KMS 系統，包括測試和驗證。

## 📋 前置需求

### 1. 安裝 Go
```bash
# 檢查 Go 版本（需要 1.22+）
go version
```

### 2. 安裝依賴
```bash
# 下載所有依賴
go mod download
go mod tidy
```

### 3. 生成 gRPC 代碼（如果需要）
```bash
# 如果 proto 文件有修改，需要重新生成
protoc --go_out=. --go-grpc_out=. proto/kms.proto
```

## 🚀 步驟 1: 準備主金鑰

### 選項 A: 使用檔案金鑰（開發/測試）

```bash
# Windows PowerShell
openssl rand -hex 32 > master.key

# 或使用 Go
go run -c "package main; import (\"crypto/rand\"; \"encoding/hex\"; \"os\"); func main() { b := make([]byte, 32); rand.Read(b); os.Stdout.WriteString(hex.EncodeToString(b)) }" > master.key

# 驗證金鑰
cat master.key
# 應該看到 64 個十六進位字元（例如：7b6f3c...）
```

### 選項 B: 使用 HSM（生產環境）

參考 [HSM 整合指南](HSM_INTEGRATION.md) 設定 HSM。

## 🚀 步驟 2: 啟動 KMS gRPC 服務

### 開啟第一個終端（Terminal 1）

```powershell
# 設定環境變數
$env:KMS_MASTER_KEY_PATH="master.key"
$env:KMS_GRPC_ADDR=":50051"

# 啟動 gRPC 服務
go run ./cmd/kms-server
```

**預期輸出**：
```
KMS server: JWT auth disabled (KMS_JWT_SECRET not set)
KMS gRPC server listening on :50051
```

**保持這個終端運行！**

## 🚀 步驟 3: 啟動 HTTP REST API 服務

### 開啟第二個終端（Terminal 2）

```powershell
# 設定環境變數
$env:KMS_GRPC_ADDR="127.0.0.1:50051"
$env:KMS_HTTP_ADDR=":8080"

# 啟動 HTTP REST API 服務
go run ./cmd/kms-http-server
```

**預期輸出**：
```
KMS HTTP server listening on :8080 (gRPC backend: 127.0.0.1:50051)
```

**保持這個終端運行！**

## 🧪 步驟 4: 測試系統

### 測試 1: 健康檢查

```powershell
# 開啟第三個終端（Terminal 3）
Invoke-RestMethod -Uri "http://localhost:8080/health"
```

**預期輸出**：
```json
{
  "status": "ok"
}
```

### 測試 2: 單筆加密

```powershell
# 準備請求
$body = @{
    plaintext = "4111111111111111"
} | ConvertTo-Json

# 發送請求
$response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt" `
    -Method Post `
    -Body $body `
    -ContentType "application/json"

# 顯示結果
$response | ConvertTo-Json -Depth 10
```

**預期輸出**：
```json
{
  "ciphertext": "base64_encoded_ciphertext...",
  "nonce": "base64_encoded_nonce..."
}
```

### 測試 3: 批次加密（高效能）

```powershell
# 準備批次請求
$body = @{
    items = @(
        @{ plaintext = "4111111111111111" },
        @{ plaintext = "4222222222222222" },
        @{ plaintext = "4333333333333333" }
    )
} | ConvertTo-Json

# 發送請求
$response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt/batch" `
    -Method Post `
    -Body $body `
    -ContentType "application/json"

# 顯示結果
$response | ConvertTo-Json -Depth 10
```

**預期輸出**：
```json
{
  "results": [
    {
      "ciphertext": "...",
      "nonce": "..."
    },
    {
      "ciphertext": "...",
      "nonce": "..."
    },
    {
      "ciphertext": "...",
      "nonce": "..."
    }
  ],
  "errors": []
}
```

### 測試 4: 解密

```powershell
# 使用之前加密得到的結果
$decryptBody = @{
    ciphertext = $response.results[0].ciphertext
    nonce = $response.results[0].nonce
} | ConvertTo-Json

# 發送解密請求
$decryptResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/decrypt" `
    -Method Post `
    -Body $decryptBody `
    -ContentType "application/json"

# 顯示結果
$decryptResponse | ConvertTo-Json
```

**預期輸出**：
```json
{
  "plaintext": "4111111111111111"
}
```

### 測試 5: 使用測試客戶端（gRPC）

```powershell
# 開啟第四個終端（Terminal 4）

# 先登入獲取 Token（如果需要 JWT）
$env:KMS_GRPC_ADDR="127.0.0.1:50051"
go run ./cmd/test-client login admin password

# 測試加密
go run ./cmd/test-client encrypt "4111111111111111"
```

## 📊 步驟 5: 效能測試

### 批次加密效能測試

```powershell
# 建立測試腳本
$testScript = @"
`$items = @()
for (`$i = 1; `$i -le 100; `$i++) {
    `$items += @{ plaintext = "4111111111111111" }
}

`$body = @{
    items = `$items
} | ConvertTo-Json

`$startTime = Get-Date
`$response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt/batch" `
    -Method Post `
    -Body `$body `
    -ContentType "application/json"
`$endTime = Get-Date

`$duration = (`$endTime - `$startTime).TotalSeconds
Write-Host "處理 100 筆資料耗時: `$duration 秒"
Write-Host "吞吐量: $([math]::Round(100 / `$duration, 2)) req/s"
"@

# 執行測試
Invoke-Expression $testScript
```

## 🔧 步驟 6: SSIS 整合測試

### 6.1 執行 SSIS 部署腳本

```powershell
# 執行部署腳本
.\scripts\deploy-ssis.ps1 `
    -KMSHttpUrl "http://localhost:8080" `
    -BatchSize 100 `
    -TestConnection
```

### 6.2 在 SSIS 中測試

1. **開啟 Visual Studio / SQL Server Data Tools**
2. **建立新的 Integration Services Project**
3. **新增 Data Flow Task**
4. **設定 Script Component**：
   - 參考 `docs/SSIS_INTEGRATION.md`
   - 使用 `docs/SSIS_SCRIPT_EXAMPLE.cs` 的程式碼
5. **執行測試**

## 🐛 故障排除

### 問題 1: 無法連接到 gRPC 服務

**症狀**：
```
failed to connect to gRPC server at 127.0.0.1:50051
```

**解決**：
1. 確認 gRPC 服務正在運行（Terminal 1）
2. 檢查 `KMS_GRPC_ADDR` 環境變數是否正確
3. 檢查防火牆設定

### 問題 2: 無法讀取 master.key

**症狀**：
```
failed to load master key from master.key
```

**解決**：
1. 確認 `master.key` 檔案存在
2. 確認檔案格式正確（64 個十六進位字元）
3. 檢查檔案權限

### 問題 3: HTTP API 返回錯誤

**症狀**：
```
500 Internal Server Error
```

**解決**：
1. 檢查 gRPC 服務是否正常運行
2. 檢查 HTTP Server 日誌
3. 確認請求格式正確（JSON）

### 問題 4: 批次加密失敗

**症狀**：
```
batch size cannot exceed 1000 items
```

**解決**：
- 減少批次大小（建議 100-200）

## 📝 完整測試腳本

建立 `test-system.ps1`：

```powershell
# KMS 系統完整測試腳本

Write-Host "=== KMS 系統測試 ===" -ForegroundColor Cyan

# 測試 1: 健康檢查
Write-Host "`n1. 測試健康檢查..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8080/health"
    Write-Host "✓ 健康檢查通過: $($health.status)" -ForegroundColor Green
}
catch {
    Write-Host "✗ 健康檢查失敗: $_" -ForegroundColor Red
    exit 1
}

# 測試 2: 單筆加密
Write-Host "`n2. 測試單筆加密..." -ForegroundColor Yellow
try {
    $body = @{ plaintext = "4111111111111111" } | ConvertTo-Json
    $encrypt = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt" `
        -Method Post -Body $body -ContentType "application/json"
    Write-Host "✓ 單筆加密成功" -ForegroundColor Green
    Write-Host "  Ciphertext: $($encrypt.ciphertext.Substring(0, 20))..." -ForegroundColor Gray
}
catch {
    Write-Host "✗ 單筆加密失敗: $_" -ForegroundColor Red
    exit 1
}

# 測試 3: 批次加密
Write-Host "`n3. 測試批次加密..." -ForegroundColor Yellow
try {
    $items = @()
    for ($i = 1; $i -le 10; $i++) {
        $items += @{ plaintext = "4111111111111111" }
    }
    $body = @{ items = $items } | ConvertTo-Json
    
    $startTime = Get-Date
    $batch = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt/batch" `
        -Method Post -Body $body -ContentType "application/json"
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    
    Write-Host "✓ 批次加密成功" -ForegroundColor Green
    Write-Host "  處理 10 筆資料，耗時: $duration 秒" -ForegroundColor Gray
    Write-Host "  吞吐量: $([math]::Round(10 / $duration, 2)) req/s" -ForegroundColor Gray
}
catch {
    Write-Host "✗ 批次加密失敗: $_" -ForegroundColor Red
    exit 1
}

# 測試 4: 解密
Write-Host "`n4. 測試解密..." -ForegroundColor Yellow
try {
    $decryptBody = @{
        ciphertext = $encrypt.ciphertext
        nonce = $encrypt.nonce
    } | ConvertTo-Json
    
    $decrypt = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/decrypt" `
        -Method Post -Body $decryptBody -ContentType "application/json"
    
    if ($decrypt.plaintext -eq "4111111111111111") {
        Write-Host "✓ 解密成功，結果正確" -ForegroundColor Green
    }
    else {
        Write-Host "✗ 解密結果不正確: $($decrypt.plaintext)" -ForegroundColor Red
        exit 1
    }
}
catch {
    Write-Host "✗ 解密失敗: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== 所有測試通過 ===" -ForegroundColor Green
```

執行測試：
```powershell
.\test-system.ps1
```

## 🎯 下一步

1. ✅ **基本測試完成** - 系統運行正常
2. 📝 **整合到 SSIS** - 參考 `docs/SSIS_INTEGRATION.md`
3. 🔒 **設定 HSM** - 參考 `docs/HSM_INTEGRATION.md`（生產環境）
4. ⚡ **效能優化** - 參考 `docs/PERFORMANCE_OPTIMIZATION.md`

## 📚 相關文件

- [SSIS 整合指南](SSIS_INTEGRATION.md)
- [HSM 整合指南](HSM_INTEGRATION.md)
- [效能優化指南](PERFORMANCE_OPTIMIZATION.md)
- [完整整合總結](COMPLETE_INTEGRATION_SUMMARY.md)

