# HSM (Hardware Security Module) 整合指南

本文件說明如何將 KMS 與各種 HSM 解決方案整合，以提供更高層級的安全性。

## 支援的 HSM 類型

### 1. PKCS#11 HSM
- **適用於**: Thales Luna, SafeNet, SoftHSM 等
- **標準**: PKCS#11 介面

### 2. AWS KMS
- **適用於**: AWS 雲端環境
- **特點**: 託管服務，無需管理硬體

### 3. Azure Key Vault
- **適用於**: Azure 雲端環境
- **特點**: 託管服務，整合 Azure 身份驗證

## 配置方式

### 方法 1: 環境變數配置（推薦）

KMS Server 會自動根據環境變數選擇 HSM 後端：

```bash
# 設定 HSM 類型
set KMS_HSM_TYPE=pkcs11  # 或 aws, azure
```

### 方法 2: 程式碼配置

在 `cmd/kms-server/main.go` 中使用 `kmslib.NewManager()` 會自動偵測環境變數。

## PKCS#11 HSM 配置

### 前置需求

1. 安裝 PKCS#11 庫（例如 SoftHSM）:
   ```bash
   # Windows
   # 下載 SoftHSM2 並安裝
   
   # Linux
   sudo apt-get install softhsm2
   ```

2. 初始化 HSM slot:
   ```bash
   softhsm2-util --init-token --slot 0 --label "KMS Token" --pin 1234 --so-pin 1234
   ```

3. 建立 AES 金鑰:
   ```bash
   # 使用 pkcs11-tool 或其他工具建立 AES-256 金鑰
   pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
     --login --pin 1234 \
     --keygen --key-type aes:32 \
     --label kms-master-key \
     --id 1
   ```

### 環境變數設定

```bash
# PKCS#11 配置
set KMS_HSM_TYPE=pkcs11
set KMS_PKCS11_LIB=C:\SoftHSM2\lib\softhsm2-x64.dll
set KMS_PKCS11_SLOT=0
set KMS_PKCS11_PIN=1234
set KMS_PKCS11_KEY_LABEL=kms-master-key
set KMS_KEY_ID=default

# 啟動 KMS Server
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server
```

### 範例：SoftHSM (Linux)

```bash
# 1. 初始化 token
softhsm2-util --init-token --slot 0 --label "KMS" --pin 1234 --so-pin 1234

# 2. 建立金鑰（使用 OpenSSL 和 pkcs11-tool）
# 注意：實際操作可能需要使用 HSM 廠商提供的工具

# 3. 設定環境變數
export KMS_HSM_TYPE=pkcs11
export KMS_PKCS11_LIB=/usr/lib/softhsm/libsofthsm2.so
export KMS_PKCS11_SLOT=1357216551
export KMS_PKCS11_PIN=1234
export KMS_PKCS11_KEY_LABEL=kms-master-key

# 4. 啟動服務
go run ./cmd/kms-server
```

## AWS KMS 配置

### 前置需求

1. AWS 帳號和 IAM 權限
2. AWS CLI 配置或環境變數

### 建立 KMS Key

```bash
# 使用 AWS CLI 建立 KMS key
aws kms create-key --description "KMS Master Key" --key-spec AES_256

# 記下 Key ID 或 ARN
```

### 環境變數設定

```bash
# AWS KMS 配置
set KMS_HSM_TYPE=aws
set KMS_AWS_KEY_ID=arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012
set KMS_AWS_REGION=us-east-1

# AWS 認證（選擇一種方式）
# 方式 1: AWS CLI 配置
aws configure

# 方式 2: 環境變數
set AWS_ACCESS_KEY_ID=your_access_key
set AWS_SECRET_ACCESS_KEY=your_secret_key

# 方式 3: IAM Role (EC2/ECS/Lambda)

# 啟動 KMS Server
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server
```

### IAM 權限需求

KMS Server 需要以下 IAM 權限：

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "kms:GenerateDataKey",
        "kms:Decrypt",
        "kms:DescribeKey"
      ],
      "Resource": "arn:aws:kms:*:*:key/*"
    }
  ]
}
```

## Azure Key Vault 配置

### 前置需求

1. Azure 訂閱
2. Azure Key Vault 資源
3. Azure CLI 或環境變數認證

### 建立 Key Vault 和 Key

```bash
# 使用 Azure CLI
az keyvault create --name my-kms-vault --resource-group my-resource-group --location eastus

# 建立 AES key
az keyvault key create --vault-name my-kms-vault --name kms-master-key --kty oct --size 256
```

### 環境變數設定

```bash
# Azure Key Vault 配置
set KMS_HSM_TYPE=azure
set KMS_AZURE_VAULT_URL=https://my-kms-vault.vault.azure.net/
set KMS_AZURE_KEY_NAME=kms-master-key

# Azure 認證（選擇一種方式）
# 方式 1: Azure CLI 登入
az login

# 方式 2: 環境變數
set AZURE_CLIENT_ID=your_client_id
set AZURE_CLIENT_SECRET=your_client_secret
set AZURE_TENANT_ID=your_tenant_id

# 方式 3: Managed Identity (適用於 Azure VM/App Service)

# 啟動 KMS Server
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server
```

### 權限需求

Key Vault 需要以下權限：
- `get` - 讀取 key
- `encrypt` - 加密（如果使用直接加密）
- `decrypt` - 解密（如果使用直接解密）
- `wrapKey` - 包裝 key（envelope encryption）
- `unwrapKey` - 解包 key（envelope encryption）

## 運作方式

### Envelope Encryption（信封加密）

對於不支援直接加密大量資料的 HSM（如 AWS KMS），系統使用 **Envelope Encryption**：

1. **加密流程**:
   - 從 HSM 取得 Data Key（由 HSM 加密）
   - 使用 Data Key 在應用層加密資料
   - 儲存加密資料和加密的 Data Key

2. **解密流程**:
   - 從 HSM 解密 Data Key
   - 使用 Data Key 在應用層解密資料

### 直接加密（如果 HSM 支援）

某些 HSM（如 PKCS#11）可能支援直接加密，系統會優先嘗試直接加密。

## 安全性考量

### 優點

1. **金鑰保護**: 主金鑰永遠不會離開 HSM
2. **合規性**: 符合 PCI-DSS、FIPS 140-2 等標準
3. **審計**: HSM 提供完整的操作日誌
4. **高可用性**: 雲端 HSM 提供自動備份和災難恢復

### 注意事項

1. **效能**: HSM 操作可能比軟體加密慢
2. **成本**: 硬體 HSM 和雲端 HSM 都有成本
3. **可用性**: 需要確保 HSM 的高可用性
4. **備份**: 確保 HSM 金鑰有適當的備份策略

## 測試 HSM 整合

### 測試腳本

```bash
# 1. 啟動 KMS Server（使用 HSM）
set KMS_HSM_TYPE=pkcs11
# ... 其他 HSM 環境變數
go run ./cmd/kms-server

# 2. 測試加密（另一個終端）
go run ./cmd/test-client encrypt "4111111111111111"

# 3. 檢查日誌確認使用 HSM
```

### 驗證 HSM 使用

檢查 KMS Server 啟動日誌：
```
KMS server: Using HSM backend (type=pkcs11)
```

## 故障排除

### PKCS#11 常見問題

**問題**: `failed to load PKCS#11 library`
- **解決**: 確認庫路徑正確，檢查檔案權限

**問題**: `failed to login to PKCS#11`
- **解決**: 確認 PIN 正確，檢查 token 狀態

**問題**: `key not found in HSM`
- **解決**: 確認 key label 正確，使用 `pkcs11-tool --list-objects` 檢查

### AWS KMS 常見問題

**問題**: `AccessDenied`
- **解決**: 檢查 IAM 權限和認證配置

**問題**: `InvalidKeyId`
- **解決**: 確認 Key ID 或 ARN 格式正確

### Azure Key Vault 常見問題

**問題**: `Unauthorized`
- **解決**: 檢查 Azure 認證和 Key Vault 權限

**問題**: `KeyNotFound`
- **解決**: 確認 Key Vault URL 和 Key 名稱正確

## 遷移指南

### 從檔案金鑰遷移到 HSM

1. **備份現有金鑰**:
   ```bash
   # 確保 master.key 有備份
   cp master.key master.key.backup
   ```

2. **將金鑰匯入 HSM**:
   - PKCS#11: 使用 `pkcs11-tool` 匯入
   - AWS KMS: 使用 `aws kms import-key-material`
   - Azure: 使用 Azure Portal 或 CLI

3. **更新環境變數**:
   ```bash
   set KMS_HSM_TYPE=pkcs11  # 或其他類型
   # ... 設定 HSM 特定變數
   ```

4. **測試**:
   - 使用測試資料驗證加密/解密
   - 確認現有加密資料可以正確解密

5. **切換**:
   - 停止舊服務
   - 啟動使用 HSM 的新服務
   - 監控錯誤日誌

## 參考資料

- [PKCS#11 標準](https://en.wikipedia.org/wiki/PKCS_11)
- [AWS KMS 文件](https://docs.aws.amazon.com/kms/)
- [Azure Key Vault 文件](https://docs.microsoft.com/azure/key-vault/)
- [SoftHSM2 文件](https://www.opendnssec.org/softhsm/)

