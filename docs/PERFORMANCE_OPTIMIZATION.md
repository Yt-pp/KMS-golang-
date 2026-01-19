# KMS Performance Optimization Guide

本文件說明如何優化 KMS 服務的效能，特別針對 SSIS ETL 流程中的大量資料加密需求。

## 效能比較

### API 類型比較

| API 類型 | 吞吐量 | 延遲 | 適用場景 |
|---------|--------|------|---------|
| gRPC 單筆 | ~500-1000 req/s | 1-5ms | 低延遲需求 |
| HTTP REST 單筆 | ~100-200 req/s | 5-15ms | SSIS 簡單整合 |
| HTTP REST 批次 (100筆) | ~1000-2000 req/s | 10-30ms | **SSIS ETL 推薦** |
| gRPC Streaming | ~5000+ req/s | 1-3ms | 高效能服務間通訊 |

## 優化策略

### 1. 使用批次 API（最重要）

**效能提升：10-20倍**

```bash
# 單筆 API：100 req/s
# 批次 API (100筆/批)：1000-2000 req/s
```

**建議批次大小：**
- 小批次（50-100筆）：低延遲，適合即時處理
- 中批次（100-200筆）：平衡效能與記憶體
- 大批次（200-500筆）：最高吞吐量，需要更多記憶體

### 2. HTTP 連線重用

在 SSIS Script Component 中：
- ✅ 在 `PreExecute()` 建立 `HttpClient`
- ✅ 在 `PostExecute()` 釋放 `HttpClient`
- ❌ 不要在每個 row 建立新的 `HttpClient`

```csharp
// ✅ 正確做法
public override void PreExecute()
{
    httpClient = new HttpClient(); // 重用連線
}

// ❌ 錯誤做法
public override void Input0_ProcessInputRow(Input0Buffer Row)
{
    using (var client = new HttpClient()) // 每次都建立新連線
    {
        // ...
    }
}
```

### 3. gRPC vs HTTP REST 選擇

#### 使用 HTTP REST 當：
- ✅ 整合 SSIS（C# Script Component）
- ✅ 需要簡單的 HTTP 呼叫
- ✅ 防火牆/網路限制較多
- ✅ 需要批次 API

#### 使用 gRPC 當：
- ✅ 服務間通訊（Go, Python, Java）
- ✅ 需要最高效能
- ✅ 需要 Streaming
- ✅ 低延遲需求

### 4. 並行處理

#### SSIS 層面
- 使用多個 Data Flow Task 並行處理
- 設定 `MaxConcurrentExecutables` 屬性

#### KMS Server 層面
HTTP Server 已內建並行處理：
- 批次 API 使用 goroutine 並行加密
- gRPC Server 自動處理並行請求

### 5. 網路優化

#### 建議配置
```
SSIS Server ←→ KMS HTTP Server (同機房/高速網路)
```

#### 網路延遲影響
- 本地網路（<1ms）：最佳效能
- 區域網路（1-5ms）：良好效能
- 跨區域（>10ms）：考慮批次大小調整

### 6. 記憶體管理

#### SSIS Script Component
- 批次大小不要超過可用記憶體
- 建議：100-200筆/批（每筆約 1-2KB）

#### KMS HTTP Server
- 預設批次限制：1000筆
- 可調整 `BATCH_SIZE_LIMIT` 環境變數

### 7. 錯誤處理與重試

#### 重試策略
```csharp
// Exponential backoff
int delayMs = (int)Math.Pow(2, attempt) * 100; // 100ms, 200ms, 400ms
```

#### 建議
- 網路錯誤：重試 3次
- 認證錯誤：不重試，記錄錯誤
- 伺服器錯誤（5xx）：重試 3次

## 效能測試結果

### 測試環境
- KMS Server: 4 CPU, 8GB RAM
- SSIS: 單執行緒 Script Component
- 網路: 本地 localhost

### 單筆 API
```
1000 筆資料
處理時間: ~10 秒
吞吐量: ~100 req/s
```

### 批次 API (100筆/批)
```
1000 筆資料
批次數: 10
處理時間: ~0.5-1 秒
吞吐量: ~1000-2000 req/s
```

### 效能提升
**批次 API 比單筆 API 快 10-20倍**

## 監控指標

### 關鍵指標
1. **吞吐量** (Throughput): req/s
2. **延遲** (Latency): ms
3. **錯誤率** (Error Rate): %
4. **CPU 使用率**: %
5. **記憶體使用率**: %

### 監控工具
- SSIS: 使用 Data Flow 的 Performance Counters
- KMS: 使用 HTTP `/health` 端點
- 應用層: 記錄處理時間和錯誤

## 最佳實踐

### SSIS ETL 流程
1. ✅ 使用批次 API（`/api/v1/encrypt/batch`）
2. ✅ 批次大小設定為 100-200
3. ✅ HTTP Client 重用連線
4. ✅ 實作錯誤處理和重試
5. ✅ 監控處理時間和錯誤率

### KMS Server 部署
1. ✅ 使用多核心 CPU（並行處理）
2. ✅ 充足的記憶體（批次緩衝）
3. ✅ 低延遲網路連線
4. ✅ 啟用 gRPC keep-alive（如果使用 gRPC）
5. ✅ 監控伺服器資源使用

### 生產環境建議
1. **負載平衡**: 多個 KMS HTTP Server 實例
2. **健康檢查**: 定期檢查 `/health` 端點
3. **自動擴展**: 根據負載自動擴展實例
4. **日誌記錄**: 記錄所有加密請求（不含明文）
5. **安全**: 使用 HTTPS 和 JWT 認證

## 故障排除

### 效能問題

**問題**: 處理速度慢
- ✅ 檢查是否使用批次 API
- ✅ 檢查批次大小是否合適
- ✅ 檢查網路延遲
- ✅ 檢查 KMS Server CPU/記憶體

**問題**: 記憶體不足
- ✅ 減少批次大小
- ✅ 檢查 SSIS 記憶體限制
- ✅ 檢查 KMS Server 記憶體使用

**問題**: 連線錯誤
- ✅ 檢查 HTTP Client 是否重用
- ✅ 檢查連線超時設定
- ✅ 檢查防火牆設定

## 進階優化

### 1. gRPC Streaming（未來擴展）

對於極高效能需求，可以實作 gRPC Streaming：

```protobuf
service KMS {
  rpc EncryptStream (stream EncryptRequest) returns (stream EncryptResponse);
}
```

### 2. 連線池

HTTP Server 可以實作連線池來進一步優化。

### 3. 快取

對於重複的 PAN（不建議，但可考慮），可以實作快取機制。

## 參考資料

- [SSIS Integration Guide](./SSIS_INTEGRATION.md)
- [gRPC Performance Best Practices](https://grpc.io/docs/guides/performance/)

