# SSIS Integration Guide - KMS PAN Encryption

本指南說明如何在 Microsoft SSIS (SQL Server Integration Services) 中整合 KMS 服務來加密 PAN (Primary Account Number) 資料。

## 架構概述

```
SSIS Data Flow → C# Script Component → HTTP REST API → KMS gRPC Service → Encrypted PAN
```

## 前置需求

1. **KMS HTTP Server** 必須運行（包裝 gRPC 服務）
2. **SSIS** 專案（SQL Server Integration Services）
3. **.NET Framework 4.7.2+** 或 **.NET Core**（用於 C# Script Component）

## 步驟 1: 啟動 KMS HTTP Server

首先啟動 gRPC 服務，然後啟動 HTTP wrapper：

```bash
# Terminal 1: 啟動 gRPC 服務
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server

# Terminal 2: 啟動 HTTP REST API 服務
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_HTTP_ADDR=:8080
set KMS_BEARER_TOKEN=your_token_here  # 可選，如果啟用 JWT
go run ./cmd/kms-http-server
```

HTTP API 端點：
- `POST http://localhost:8080/api/v1/encrypt` - 單筆加密
- `POST http://localhost:8080/api/v1/encrypt/batch` - 批次加密（高效能）
- `POST http://localhost:8080/api/v1/decrypt` - 解密
- `GET http://localhost:8080/health` - 健康檢查

## 步驟 2: SSIS 設定

### 2.1 建立 SSIS Package

1. 在 Visual Studio 中建立新的 **Integration Services Project**
2. 新增 **Data Flow Task**
3. 在 Data Flow 中：
   - 新增 **OLE DB Source**（從來源資料庫讀取 PAN）
   - 新增 **Script Component**（轉換為 Transformation）
   - 新增 **OLE DB Destination**（寫入加密後的資料）

### 2.2 設定 Script Component

#### Input Columns
- `PAN` (String, DT_STR 或 DT_WSTR) - 要加密的卡號欄位
- 其他需要的欄位（如 ID、CVV 等）

#### Output Columns
- `EncryptedPAN` (String, DT_WSTR) - 加密後的密文（Base64）
- `PANNonce` (String, DT_WSTR) - Nonce（Base64）
- 其他原始欄位

## 步驟 3: C# Script Component 程式碼

### 方法 A: 單筆加密（簡單但較慢）

在 Script Component 的 `ScriptMain.cs` 中：

```csharp
using System;
using System.Data;
using Microsoft.SqlServer.Dts.Pipeline.Wrapper;
using Microsoft.SqlServer.Dts.Runtime.Wrapper;
using System.Net.Http;
using System.Text;
using System.Threading.Tasks;
using Newtonsoft.Json;

[Microsoft.SqlServer.Dts.Pipeline.SSISScriptComponentEntryPointAttribute]
public class ScriptMain : UserComponent
{
    private HttpClient httpClient;
    private string kmsApiUrl = "http://localhost:8080/api/v1/encrypt";
    private string bearerToken = ""; // 如果啟用 JWT，設定 token

    public override void PreExecute()
    {
        base.PreExecute();
        
        // 初始化 HTTP Client（重用連線以提升效能）
        httpClient = new HttpClient();
        httpClient.Timeout = TimeSpan.FromSeconds(30);
        
        if (!string.IsNullOrEmpty(bearerToken))
        {
            httpClient.DefaultRequestHeaders.Authorization = 
                new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", bearerToken);
        }
    }

    public override void PostExecute()
    {
        base.PostExecute();
        
        // 清理資源
        if (httpClient != null)
        {
            httpClient.Dispose();
        }
    }

    public override void Input0_ProcessInputRow(Input0Buffer Row)
    {
        try
        {
            string pan = Row.PAN;
            
            // 呼叫 KMS API 加密
            var encrypted = EncryptPAN(pan).Result;
            
            // 設定輸出欄位
            Row.EncryptedPAN = encrypted.Ciphertext;
            Row.PANNonce = encrypted.Nonce;
        }
        catch (Exception ex)
        {
            // 錯誤處理：記錄錯誤或設定預設值
            ComponentMetaData.FireError(0, "KMS Encryption", 
                $"Failed to encrypt PAN: {ex.Message}", "", 0, out bool pbCancel);
            
            // 可以選擇跳過該 row 或設定預設值
            Row.EncryptedPAN = "";
            Row.PANNonce = "";
        }
    }

    private async Task<EncryptResponse> EncryptPAN(string plaintext)
    {
        var request = new
        {
            plaintext = plaintext
        };

        var json = JsonConvert.SerializeObject(request);
        var content = new StringContent(json, Encoding.UTF8, "application/json");

        var response = await httpClient.PostAsync(kmsApiUrl, content);
        response.EnsureSuccessStatusCode();

        var responseJson = await response.Content.ReadAsStringAsync();
        return JsonConvert.DeserializeObject<EncryptResponse>(responseJson);
    }

    private class EncryptResponse
    {
        public string Ciphertext { get; set; }
        public string Nonce { get; set; }
    }
}
```

### 方法 B: 批次加密（高效能推薦）

對於大量資料，使用批次 API 可以大幅提升效能：

```csharp
using System;
using System.Collections.Generic;
using System.Data;
using Microsoft.SqlServer.Dts.Pipeline.Wrapper;
using Microsoft.SqlServer.Dts.Runtime.Wrapper;
using System.Net.Http;
using System.Text;
using System.Threading.Tasks;
using Newtonsoft.Json;
using System.Linq;

[Microsoft.SqlServer.Dts.Pipeline.SSISScriptComponentEntryPointAttribute]
public class ScriptMain : UserComponent
{
    private HttpClient httpClient;
    private string kmsBatchApiUrl = "http://localhost:8080/api/v1/encrypt/batch";
    private string bearerToken = "";
    private List<RowData> batchBuffer;
    private const int BATCH_SIZE = 100; // 每批處理 100 筆

    public override void PreExecute()
    {
        base.PreExecute();
        
        httpClient = new HttpClient();
        httpClient.Timeout = TimeSpan.FromSeconds(60);
        
        if (!string.IsNullOrEmpty(bearerToken))
        {
            httpClient.DefaultRequestHeaders.Authorization = 
                new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", bearerToken);
        }
        
        batchBuffer = new List<RowData>();
    }

    public override void PostExecute()
    {
        base.PostExecute();
        
        // 處理剩餘的批次
        if (batchBuffer.Count > 0)
        {
            ProcessBatch();
        }
        
        if (httpClient != null)
        {
            httpClient.Dispose();
        }
    }

    public override void Input0_ProcessInputRow(Input0Buffer Row)
    {
        // 將 row 加入批次緩衝區
        batchBuffer.Add(new RowData
        {
            Row = Row,
            PAN = Row.PAN
        });

        // 當批次達到大小時，處理批次
        if (batchBuffer.Count >= BATCH_SIZE)
        {
            ProcessBatch();
            batchBuffer.Clear();
        }
    }

    private void ProcessBatch()
    {
        try
        {
            // 準備批次請求
            var batchRequest = new
            {
                items = batchBuffer.Select(r => new { plaintext = r.PAN }).ToArray()
            };

            var json = JsonConvert.SerializeObject(batchRequest);
            var content = new StringContent(json, Encoding.UTF8, "application/json");

            // 同步呼叫（在 SSIS 中避免複雜的 async）
            var response = httpClient.PostAsync(kmsBatchApiUrl, content).Result;
            response.EnsureSuccessStatusCode();

            var responseJson = response.Content.ReadAsStringAsync().Result;
            var batchResponse = JsonConvert.DeserializeObject<BatchEncryptResponse>(responseJson);

            // 將結果寫回對應的 row
            for (int i = 0; i < batchBuffer.Count && i < batchResponse.Results.Count; i++)
            {
                var rowData = batchBuffer[i];
                var encrypted = batchResponse.Results[i];
                
                rowData.Row.EncryptedPAN = encrypted.Ciphertext;
                rowData.Row.PANNonce = encrypted.Nonce;
            }
        }
        catch (Exception ex)
        {
            ComponentMetaData.FireError(0, "KMS Batch Encryption", 
                $"Batch encryption failed: {ex.Message}", "", 0, out bool pbCancel);
            
            // 設定錯誤標記
            foreach (var rowData in batchBuffer)
            {
                rowData.Row.EncryptedPAN = "";
                rowData.Row.PANNonce = "";
            }
        }
    }

    private class RowData
    {
        public Input0Buffer Row { get; set; }
        public string PAN { get; set; }
    }

    private class BatchEncryptResponse
    {
        public List<EncryptResponse> Results { get; set; }
        public List<string> Errors { get; set; }
    }

    private class EncryptResponse
    {
        public string Ciphertext { get; set; }
        public string Nonce { get; set; }
    }
}
```

## 步驟 4: 效能優化建議

### 4.1 使用批次 API
- **單筆 API**: ~100-200 requests/sec
- **批次 API (100筆/批)**: ~1000-2000 requests/sec（10-20倍提升）

### 4.2 連線重用
- `HttpClient` 在 `PreExecute()` 中建立，在 `PostExecute()` 中釋放
- 避免每個 row 都建立新的 HTTP 連線

### 4.3 批次大小調整
- 根據網路延遲和記憶體調整 `BATCH_SIZE`
- 建議值：50-200 筆/批
- 太大可能導致記憶體問題，太小則效能不佳

### 4.4 錯誤處理
- 實作重試機制（exponential backoff）
- 記錄失敗的 row 以便後續處理
- 考慮使用 SSIS 的錯誤輸出（Error Output）

### 4.5 並行處理
如果 SSIS 支援，可以：
- 使用多個 Script Component 並行處理
- 在 KMS HTTP Server 端啟用多執行緒處理

## 步驟 5: 設定變數（可選）

在 SSIS Package 中建立變數，方便管理：

1. **KMS_API_URL**: `http://localhost:8080/api/v1/encrypt`
2. **KMS_BATCH_SIZE**: `100`
3. **KMS_BEARER_TOKEN**: （如果使用 JWT）

在 Script Component 中讀取變數：

```csharp
public override void PreExecute()
{
    base.PreExecute();
    
    // 從 SSIS 變數讀取
    kmsApiUrl = Variables.KMS_API_URL;
    bearerToken = Variables.KMS_BEARER_TOKEN;
    // ...
}
```

## 步驟 6: 測試

1. 使用少量測試資料驗證加密流程
2. 檢查加密後的資料格式（Base64）
3. 驗證可以正確解密
4. 監控效能指標（處理速度、錯誤率）

## 疑難排解

### 連線錯誤
- 確認 KMS HTTP Server 正在運行
- 檢查防火牆設定
- 驗證 URL 和 port 正確

### 效能問題
- 使用批次 API 而非單筆 API
- 增加批次大小
- 檢查網路延遲
- 考慮在 SSIS Server 和 KMS Server 之間使用高速網路

### 記憶體問題
- 減少批次大小
- 檢查 SSIS 記憶體限制設定

## 安全考量

1. **傳輸加密**: 生產環境建議使用 HTTPS (TLS)
2. **認證**: 啟用 JWT Bearer Token
3. **網路隔離**: KMS Server 應在受保護的網路中
4. **日誌**: 避免在日誌中記錄明文 PAN

## 範例：完整 SSIS Package 流程

```
1. OLE DB Source
   └─> SELECT id, pan, cvv FROM source_table

2. Script Component (Transformation)
   └─> 批次加密 PAN 和 CVV
   └─> 輸出: id, encrypted_pan, pan_nonce, encrypted_cvv, cvv_nonce

3. OLE DB Destination
   └─> INSERT INTO encrypted_table 
       (id, encrypted_pan, pan_nonce, encrypted_cvv, cvv_nonce)
```

## 參考資料

- [KMS gRPC API Documentation](../README_GRPC.md)
- [KMS Authentication Guide](../README_AUTH.md)
- [Microsoft SSIS Script Component](https://docs.microsoft.com/en-us/sql/integration-services/extending-packages-scripting/data-flow-script-component/)

