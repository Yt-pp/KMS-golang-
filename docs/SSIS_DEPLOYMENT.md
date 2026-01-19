# SSIS 部署完整指南

本指南提供完整的 SSIS 整合部署步驟，包括自動化腳本和配置。

## 快速部署

### 步驟 1: 執行部署腳本

```powershell
# 基本部署（測試連線）
.\scripts\deploy-ssis.ps1 -TestConnection

# 完整配置
.\scripts\deploy-ssis.ps1 `
  -KMSHttpUrl "http://your-kms-server:8080" `
  -KMSToken "your_bearer_token" `
  -BatchSize 100 `
  -TestConnection
```

### 步驟 2: 在 SSIS 中設定

1. **開啟 SSIS Package**
2. **新增 Script Component** (Transformation)
3. **設定 Input Columns**: `PAN` (String)
4. **設定 Output Columns**: 
   - `EncryptedPAN` (String)
   - `PANNonce` (String)
5. **貼上程式碼**: 從 `docs/SSIS_SCRIPT_EXAMPLE.cs` 複製
6. **更新設定**: 修改 API URL 和批次大小

## 詳細步驟

### 1. 準備 KMS 服務

#### 啟動 gRPC 服務
```bash
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server
```

#### 啟動 HTTP REST API
```bash
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_HTTP_ADDR=:8080
set KMS_BEARER_TOKEN=your_token_here  # 可選
go run ./cmd/kms-http-server
```

### 2. 測試連線

使用部署腳本測試：
```powershell
.\scripts\deploy-ssis.ps1 -TestConnection -KMSHttpUrl "http://localhost:8080"
```

或手動測試：
```powershell
# 健康檢查
Invoke-RestMethod -Uri "http://localhost:8080/health"

# 測試加密
$body = @{
    items = @(
        @{ plaintext = "4111111111111111" }
    )
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/encrypt/batch" `
  -Method Post `
  -Body $body `
  -ContentType "application/json"
```

### 3. SSIS Package 設定

#### 3.1 建立變數（可選）

在 SSIS Package 中建立以下變數：

| 變數名稱 | 類型 | 值 |
|---------|------|-----|
| `KMS_API_URL` | String | `http://localhost:8080/api/v1/encrypt/batch` |
| `KMS_BATCH_SIZE` | Int32 | `100` |
| `KMS_BEARER_TOKEN` | String | `your_token` (可選) |

#### 3.2 Script Component 設定

1. **新增 Script Component**
   - 在 Data Flow 中拖放 Script Component
   - 選擇 **Transformation**

2. **設定 Input**
   - 選擇要加密的欄位（例如 `PAN`）
   - 類型：String (DT_STR 或 DT_WSTR)

3. **設定 Output**
   - `EncryptedPAN` (String, DT_WSTR)
   - `PANNonce` (String, DT_WSTR)

4. **新增參考**
   - 在 Script Component 的 References 中新增：
     - `System.Net.Http`
     - `Newtonsoft.Json` (需要透過 NuGet 安裝)

5. **貼上程式碼**
   - 開啟 Script Editor
   - 複製 `docs/SSIS_SCRIPT_EXAMPLE.cs` 的內容
   - 修改以下設定：
     ```csharp
     private string kmsBatchApiUrl = "http://localhost:8080/api/v1/encrypt/batch";
     private int batchSize = 100;
     private bool useBatchMode = true;
     ```

### 4. 測試執行

1. **小量測試**
   - 使用少量測試資料（10-100 筆）
   - 驗證加密結果正確
   - 檢查錯誤處理

2. **效能測試**
   - 使用實際資料量
   - 監控處理速度
   - 調整批次大小以優化效能

3. **錯誤測試**
   - 模擬網路錯誤
   - 測試無效的 PAN 格式
   - 驗證錯誤處理邏輯

## 生產環境部署

### 1. 安全配置

#### 使用 HTTPS
```csharp
private string kmsBatchApiUrl = "https://kms-server.company.com/api/v1/encrypt/batch";
```

#### 使用 JWT 認證
```csharp
private string bearerToken = Variables.KMS_BEARER_TOKEN; // 從 SSIS 變數讀取
```

#### 安全儲存 Token
- 使用 SSIS Package Configuration
- 使用 Windows Credential Store
- 使用環境變數（不建議，但可用於測試）

### 2. 效能優化

#### 批次大小調整
根據資料量和網路延遲調整：
```csharp
// 小批次：低延遲
private int batchSize = 50;

// 中批次：平衡（推薦）
private int batchSize = 100;

// 大批次：高吞吐量
private int batchSize = 200;
```

#### 並行處理
- 使用多個 Data Flow Task
- 設定 `MaxConcurrentExecutables`

### 3. 監控和日誌

#### SSIS 日誌
- 啟用 SSIS 日誌記錄
- 記錄加密錯誤和效能指標

#### 應用層監控
- 記錄處理時間
- 記錄錯誤率
- 監控 KMS API 回應時間

### 4. 錯誤處理策略

#### 重試機制
```csharp
private async Task<EncryptResponse> EncryptPANWithRetry(string plaintext, int maxRetries = 3)
{
    Exception lastException = null;
    
    for (int attempt = 0; attempt < maxRetries; attempt++)
    {
        try
        {
            return await EncryptPAN(plaintext);
        }
        catch (Exception ex)
        {
            lastException = ex;
            if (attempt < maxRetries - 1)
            {
                int delayMs = (int)Math.Pow(2, attempt) * 100; // Exponential backoff
                System.Threading.Thread.Sleep(delayMs);
            }
        }
    }
    
    throw lastException;
}
```

#### 錯誤輸出
- 使用 SSIS Error Output
- 記錄失敗的 row 以便後續處理
- 發送告警通知

## 故障排除

### 常見問題

#### 1. 連線錯誤
**症狀**: `Failed to connect to KMS`
**解決**:
- 檢查 KMS HTTP Server 是否運行
- 檢查防火牆設定
- 驗證 URL 和 port 正確

#### 2. 認證錯誤
**症狀**: `401 Unauthorized`
**解決**:
- 檢查 Bearer Token 是否正確
- 確認 Token 未過期
- 驗證 JWT 設定

#### 3. 效能問題
**症狀**: 處理速度慢
**解決**:
- 使用批次 API 而非單筆 API
- 增加批次大小
- 檢查網路延遲
- 檢查 KMS Server 資源使用

#### 4. 記憶體問題
**症狀**: 記憶體不足錯誤
**解決**:
- 減少批次大小
- 檢查 SSIS 記憶體限制
- 優化 Script Component 程式碼

## 自動化部署

### PowerShell 部署腳本

```powershell
# 完整自動化部署
$config = @{
    KMSHttpUrl = "http://kms-server:8080"
    KMSToken = "your_token"
    BatchSize = 100
}

.\scripts\deploy-ssis.ps1 @config -TestConnection
```

### SSIS Package 配置

使用 SSIS Package Configuration 儲存設定：
- XML 配置檔案
- SQL Server 資料庫
- 環境變數
- 登錄表

## 參考資料

- [SSIS 整合指南](./SSIS_INTEGRATION.md)
- [SSIS C# Script 範例](./SSIS_SCRIPT_EXAMPLE.cs)
- [效能優化指南](./PERFORMANCE_OPTIMIZATION.md)
- [HSM 整合指南](./HSM_INTEGRATION.md)

