# 快速開始指南

5 分鐘內啟動並測試整個 KMS 系統！

## 🚀 方法 1: 使用自動化腳本（推薦）

### 步驟 1: 啟動服務

```powershell
# 啟動 gRPC 和 HTTP 服務
.\start-kms.ps1
```

腳本會自動：
- ✅ 檢查並生成 `master.key`（如果不存在）
- ✅ 啟動 gRPC 服務
- ✅ 啟動 HTTP REST API 服務

### 步驟 2: 測試系統

開啟**新的終端**，執行測試：

```powershell
# 執行完整測試
.\test-system.ps1
```

## 🚀 方法 2: 手動啟動

### 步驟 1: 準備主金鑰

```powershell
# 生成主金鑰
openssl rand -hex 32 > master.key

# 或使用 Go
go run -c "package main; import (\"crypto/rand\"; \"encoding/hex\"; \"os\"); func main() { b := make([]byte, 32); rand.Read(b); os.Stdout.WriteString(hex.EncodeToString(b)) }" > master.key
```

### 步驟 2: 啟動 gRPC 服務

**終端 1**：
```powershell
$env:KMS_MASTER_KEY_PATH="master.key"
$env:KMS_GRPC_ADDR=":50051"
go run ./cmd/kms-server
```

### 步驟 3: 啟動 HTTP REST API

**終端 2**：
```powershell
$env:KMS_GRPC_ADDR="127.0.0.1:50051"
$env:KMS_HTTP_ADDR=":8080"
go run ./cmd/kms-http-server
```

### 步驟 4: 測試

**終端 3**：
```powershell
# 健康檢查
Invoke-RestMethod -Uri "http://localhost:8080/health"

# 單筆加密
$body = @{ plaintext = "4111111111111111" } | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt" `
    -Method Post -Body $body -ContentType "application/json"

# 批次加密
$items = @(
    @{ plaintext = "4111111111111111" },
    @{ plaintext = "4222222222222222" }
)
$body = @{ items = $items } | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt/batch" `
    -Method Post -Body $body -ContentType "application/json"
```

## ✅ 驗證清單

- [ ] gRPC 服務運行中（終端 1）
- [ ] HTTP REST API 運行中（終端 2）
- [ ] 健康檢查返回 `{"status":"ok"}`
- [ ] 單筆加密成功
- [ ] 批次加密成功
- [ ] 解密成功

## 🎯 下一步

1. **整合到 SSIS** - 參考 [SSIS 整合指南](SSIS_INTEGRATION.md)
2. **設定 HSM** - 參考 [HSM 整合指南](HSM_INTEGRATION.md)（生產環境）
3. **效能優化** - 參考 [效能優化指南](PERFORMANCE_OPTIMIZATION.md)

## 🐛 遇到問題？

參考 [完整運行指南](RUN_SYSTEM_GUIDE.md) 的故障排除章節。

