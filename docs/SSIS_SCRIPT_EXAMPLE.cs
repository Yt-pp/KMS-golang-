// SSIS Script Component - C# Code Example
// 完整可用的 SSIS Script Component 程式碼範例
// 
// 使用方式：
// 1. 在 SSIS Data Flow 中新增 Script Component
// 2. 設定為 Transformation
// 3. 設定 Input Columns: PAN (String)
// 4. 設定 Output Columns: EncryptedPAN (String), PANNonce (String)
// 5. 貼上此程式碼到 ScriptMain.cs
// 6. 在 References 中新增 System.Net.Http 和 Newtonsoft.Json

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
    // ========== 設定區域 ==========
    private string kmsApiUrl = "http://localhost:8080/api/v1/encrypt";
    private string kmsBatchApiUrl = "http://localhost:8080/api/v1/encrypt/batch";
    private string bearerToken = ""; // 如果啟用 JWT，在此設定 token
    private bool useBatchMode = true; // true = 批次模式（高效能）, false = 單筆模式
    private int batchSize = 100; // 批次大小（僅在批次模式下有效）
    
    // ========== 內部變數 ==========
    private HttpClient httpClient;
    private List<RowData> batchBuffer;

    // ========== SSIS 生命週期方法 ==========
    
    public override void PreExecute()
    {
        base.PreExecute();
        
        // 初始化 HTTP Client（重用連線以提升效能）
        httpClient = new HttpClient();
        httpClient.Timeout = TimeSpan.FromSeconds(60);
        
        // 設定 Authorization header（如果使用 JWT）
        if (!string.IsNullOrEmpty(bearerToken))
        {
            httpClient.DefaultRequestHeaders.Authorization = 
                new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", bearerToken);
        }
        
        // 初始化批次緩衝區（如果使用批次模式）
        if (useBatchMode)
        {
            batchBuffer = new List<RowData>();
        }
    }

    public override void PostExecute()
    {
        base.PostExecute();
        
        // 處理剩餘的批次資料
        if (useBatchMode && batchBuffer != null && batchBuffer.Count > 0)
        {
            ProcessBatch();
        }
        
        // 清理資源
        if (httpClient != null)
        {
            httpClient.Dispose();
        }
    }

    // ========== 主要處理邏輯 ==========
    
    public override void Input0_ProcessInputRow(Input0Buffer Row)
    {
        try
        {
            if (useBatchMode)
            {
                // 批次模式：將 row 加入緩衝區
                ProcessRowInBatch(Row);
            }
            else
            {
                // 單筆模式：立即處理
                ProcessRowSingle(Row);
            }
        }
        catch (Exception ex)
        {
            // 錯誤處理
            HandleError(Row, ex);
        }
    }

    // ========== 單筆加密模式 ==========
    
    private void ProcessRowSingle(Input0Buffer Row)
    {
        string pan = Row.PAN;
        
        if (string.IsNullOrWhiteSpace(pan))
        {
            Row.EncryptedPAN = "";
            Row.PANNonce = "";
            return;
        }
        
        // 同步呼叫（SSIS 不支援 async/await）
        var encrypted = EncryptPAN(pan).Result;
        
        Row.EncryptedPAN = encrypted.Ciphertext;
        Row.PANNonce = encrypted.Nonce;
    }

    // ========== 批次加密模式 ==========
    
    private void ProcessRowInBatch(Input0Buffer Row)
    {
        // 將 row 加入批次緩衝區
        batchBuffer.Add(new RowData
        {
            Row = Row,
            PAN = Row.PAN ?? ""
        });

        // 當批次達到大小時，處理批次
        if (batchBuffer.Count >= batchSize)
        {
            ProcessBatch();
            batchBuffer.Clear();
        }
    }

    private void ProcessBatch()
    {
        if (batchBuffer == null || batchBuffer.Count == 0)
        {
            return;
        }

        try
        {
            // 準備批次請求
            var batchRequest = new
            {
                items = batchBuffer.Select(r => new 
                { 
                    plaintext = r.PAN 
                }).ToArray()
            };

            var json = JsonConvert.SerializeObject(batchRequest);
            var content = new StringContent(json, Encoding.UTF8, "application/json");

            // 同步呼叫批次 API
            var response = httpClient.PostAsync(kmsBatchApiUrl, content).Result;
            
            if (!response.IsSuccessStatusCode)
            {
                var errorContent = response.Content.ReadAsStringAsync().Result;
                throw new Exception($"HTTP {response.StatusCode}: {errorContent}");
            }

            var responseJson = response.Content.ReadAsStringAsync().Result;
            var batchResponse = JsonConvert.DeserializeObject<BatchEncryptResponse>(responseJson);

            // 將結果寫回對應的 row
            int resultIndex = 0;
            foreach (var rowData in batchBuffer)
            {
                if (resultIndex < batchResponse.Results.Count)
                {
                    var encrypted = batchResponse.Results[resultIndex];
                    rowData.Row.EncryptedPAN = encrypted.Ciphertext;
                    rowData.Row.PANNonce = encrypted.Nonce;
                    resultIndex++;
                }
                else
                {
                    // 結果數量不足，可能是錯誤
                    rowData.Row.EncryptedPAN = "";
                    rowData.Row.PANNonce = "";
                }
            }

            // 記錄錯誤（如果有）
            if (batchResponse.Errors != null && batchResponse.Errors.Count > 0)
            {
                foreach (var error in batchResponse.Errors)
                {
                    ComponentMetaData.FireWarning(0, "KMS Batch Encryption", 
                        $"Batch encryption error: {error}", "", 0);
                }
            }
        }
        catch (Exception ex)
        {
            // 批次處理失敗，標記所有 row 為錯誤
            ComponentMetaData.FireError(0, "KMS Batch Encryption", 
                $"Batch encryption failed: {ex.Message}", "", 0, out bool pbCancel);
            
            foreach (var rowData in batchBuffer)
            {
                rowData.Row.EncryptedPAN = "";
                rowData.Row.PANNonce = "";
            }
        }
    }

    // ========== API 呼叫方法 ==========
    
    private async Task<EncryptResponse> EncryptPAN(string plaintext)
    {
        if (string.IsNullOrWhiteSpace(plaintext))
        {
            return new EncryptResponse { Ciphertext = "", Nonce = "" };
        }

        var request = new
        {
            plaintext = plaintext
        };

        var json = JsonConvert.SerializeObject(request);
        var content = new StringContent(json, Encoding.UTF8, "application/json");

        var response = await httpClient.PostAsync(kmsApiUrl, content);
        
        if (!response.IsSuccessStatusCode)
        {
            var errorContent = await response.Content.ReadAsStringAsync();
            throw new Exception($"HTTP {response.StatusCode}: {errorContent}");
        }

        var responseJson = await response.Content.ReadAsStringAsync();
        return JsonConvert.DeserializeObject<EncryptResponse>(responseJson);
    }

    // ========== 錯誤處理 ==========
    
    private void HandleError(Input0Buffer Row, Exception ex)
    {
        // 記錄錯誤
        ComponentMetaData.FireError(0, "KMS Encryption", 
            $"Failed to encrypt PAN: {ex.Message}", "", 0, out bool pbCancel);
        
        // 設定預設值（或可以選擇跳過該 row）
        Row.EncryptedPAN = "";
        Row.PANNonce = "";
    }

    // ========== 資料結構 ==========
    
    private class RowData
    {
        public Input0Buffer Row { get; set; }
        public string PAN { get; set; }
    }

    private class EncryptResponse
    {
        [JsonProperty("ciphertext")]
        public string Ciphertext { get; set; }
        
        [JsonProperty("nonce")]
        public string Nonce { get; set; }
    }

    private class BatchEncryptResponse
    {
        [JsonProperty("results")]
        public List<EncryptResponse> Results { get; set; }
        
        [JsonProperty("errors")]
        public List<string> Errors { get; set; }
    }
}

// ========== 使用 SSIS 變數的版本（進階） ==========
// 如果要在 SSIS Package 中使用變數，可以這樣修改：

/*
public override void PreExecute()
{
    base.PreExecute();
    
    // 從 SSIS 變數讀取設定
    kmsApiUrl = Variables.KMS_API_URL;
    kmsBatchApiUrl = Variables.KMS_BATCH_API_URL;
    bearerToken = Variables.KMS_BEARER_TOKEN;
    useBatchMode = Variables.KMS_USE_BATCH_MODE;
    batchSize = Variables.KMS_BATCH_SIZE;
    
    // ... 其餘初始化程式碼
}
*/

// ========== 重試機制範例（進階） ==========
/*
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
            
            // Exponential backoff
            if (attempt < maxRetries - 1)
            {
                int delayMs = (int)Math.Pow(2, attempt) * 100; // 100ms, 200ms, 400ms
                System.Threading.Thread.Sleep(delayMs);
            }
        }
    }
    
    throw lastException;
}
*/

