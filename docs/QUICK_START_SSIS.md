# SSIS 快速開始指南

## 5 分鐘快速設定

### 步驟 1: 啟動 KMS 服務

```bash
# Terminal 1: 啟動 gRPC 服務
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server

# Terminal 2: 啟動 HTTP REST API
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_HTTP_ADDR=:8080
go run ./cmd/kms-http-server
```

### 步驟 2: 測試 API

```bash
# 測試健康檢查
curl http://localhost:8080/health

# 測試單筆加密
curl -X POST http://localhost:8080/api/v1/encrypt ^
  -H "Content-Type: application/json" ^
  -d "{\"plaintext\":\"4111111111111111\"}"

# 測試批次加密（高效能）
curl -X POST http://localhost:8080/api/v1/encrypt/batch ^
  -H "Content-Type: application/json" ^
  -d "{\"items\":[{\"plaintext\":\"4111111111111111\"},{\"plaintext\":\"4222222222222222\"}]}"
```

### 步驟 3: SSIS 設定

1. 在 SSIS Data Flow 中新增 **Script Component**
2. 設定為 **Transformation**
3. 輸入欄位：`PAN` (String)
4. 輸出欄位：`EncryptedPAN` (String), `PANNonce` (String)
5. 複製 [SSIS_SCRIPT_EXAMPLE.cs](./SSIS_SCRIPT_EXAMPLE.cs) 的程式碼
6. 修改 `kmsBatchApiUrl` 為你的 KMS 服務地址
7. 設定 `useBatchMode = true` 以獲得最佳效能

### 步驟 4: 執行測試

執行 SSIS Package，檢查：
- ✅ 資料正確加密
- ✅ 處理速度符合預期（批次模式應比單筆快 10-20倍）
- ✅ 錯誤處理正常

## API 端點說明

### 單筆加密
```http
POST /api/v1/encrypt
Content-Type: application/json

{
  "plaintext": "4111111111111111"
}

Response:
{
  "ciphertext": "base64_encoded_ciphertext",
  "nonce": "base64_encoded_nonce"
}
```

### 批次加密（推薦）
```http
POST /api/v1/encrypt/batch
Content-Type: application/json

{
  "items": [
    {"plaintext": "4111111111111111"},
    {"plaintext": "4222222222222222"}
  ]
}

Response:
{
  "results": [
    {"ciphertext": "...", "nonce": "..."},
    {"ciphertext": "...", "nonce": "..."}
  ],
  "errors": []
}
```

## 效能建議

| 場景 | API 類型 | 批次大小 | 預期效能 |
|------|---------|---------|---------|
| 少量資料 (<1000筆) | 單筆或批次 | 50-100 | 100-200 req/s |
| 中等資料 (1000-10000筆) | **批次** | 100-200 | **1000-2000 req/s** |
| 大量資料 (>10000筆) | **批次** | 200-500 | **2000+ req/s** |

## 常見問題

**Q: 為什麼使用 HTTP 而不是直接使用 gRPC？**
A: SSIS C# Script Component 更容易整合 HTTP REST API。gRPC 需要額外的 .NET 套件和設定。

**Q: 批次 API 真的比較快嗎？**
A: 是的！批次 API 可以達到 10-20倍的效能提升，因為：
- 減少 HTTP 連線開銷
- 並行處理多筆加密
- 減少網路往返次數

**Q: 如何處理錯誤？**
A: SSIS Script Component 範例中已包含錯誤處理。批次 API 會返回錯誤陣列，可以個別處理失敗的項目。

**Q: 需要認證嗎？**
A: 可選。如果 KMS Server 啟用了 JWT，設定 `bearerToken` 變數即可。

## 下一步

- 📖 閱讀 [完整 SSIS 整合指南](./SSIS_INTEGRATION.md)
- ⚡ 查看 [效能優化指南](./PERFORMANCE_OPTIMIZATION.md)
- 💻 參考 [完整 C# 範例程式碼](./SSIS_SCRIPT_EXAMPLE.cs)

