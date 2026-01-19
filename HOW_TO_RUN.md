# 如何運行系統 - 簡單說明

## 🎯 最簡單的方式（3 步驟）

### 步驟 1: 啟動服務

開啟 PowerShell，執行：

```powershell
.\start-kms.ps1
```

這會自動：
- ✅ 檢查並生成 `master.key`（如果不存在）
- ✅ 啟動 gRPC 服務（Port 50051）
- ✅ 啟動 HTTP REST API 服務（Port 8080）

**保持這個終端運行！**

### 步驟 2: 測試系統

開啟**新的 PowerShell 終端**，執行：

```powershell
.\test-system.ps1
```

這會自動測試：
- ✅ 健康檢查
- ✅ 單筆加密
- ✅ 批次加密（10 筆）
- ✅ 解密
- ✅ 效能測試（100 筆）

### 步驟 3: 查看結果

測試完成後，您會看到：
```
=== 所有測試通過 ===
```

## 📝 手動測試（可選）

如果想手動測試 API：

```powershell
# 健康檢查
Invoke-RestMethod -Uri "http://localhost:8080/health"

# 單筆加密
$body = @{ plaintext = "4111111111111111" } | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt" `
    -Method Post -Body $body -ContentType "application/json"

# 批次加密（高效能）
$items = @(
    @{ plaintext = "4111111111111111" },
    @{ plaintext = "4222222222222222" }
)
$body = @{ items = $items } | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt/batch" `
    -Method Post -Body $body -ContentType "application/json"
```

## 🛑 停止服務

在運行 `start-kms.ps1` 的終端中按 `Ctrl+C` 停止服務。

## 📚 更多資訊

- **快速開始**: [docs/QUICK_START.md](docs/QUICK_START.md)
- **完整指南**: [docs/RUN_SYSTEM_GUIDE.md](docs/RUN_SYSTEM_GUIDE.md)
- **SSIS 整合**: [docs/SSIS_INTEGRATION.md](docs/SSIS_INTEGRATION.md)
- **HSM 設定**: [docs/HSM_INTEGRATION.md](docs/HSM_INTEGRATION.md)

## ❓ 遇到問題？

1. **無法啟動服務**
   - 確認 Go 已安裝：`go version`
   - 確認依賴已下載：`go mod tidy`

2. **測試失敗**
   - 確認服務正在運行（檢查終端 1）
   - 確認 Port 8080 未被占用

3. **需要幫助**
   - 查看 [完整運行指南](docs/RUN_SYSTEM_GUIDE.md) 的故障排除章節

