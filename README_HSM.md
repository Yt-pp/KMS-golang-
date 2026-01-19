# HSM 整合快速參考

## 目前支援的 HSM 類型

1. **PKCS#11** - 硬體 HSM (Thales, SafeNet, SoftHSM)
2. **AWS KMS** - 雲端 HSM
3. **Azure Key Vault** - 雲端 HSM

## 快速開始

### PKCS#11 HSM

```bash
set KMS_HSM_TYPE=pkcs11
set KMS_PKCS11_LIB=C:\path\to\pkcs11.dll
set KMS_PKCS11_SLOT=0
set KMS_PKCS11_PIN=1234
set KMS_PKCS11_KEY_LABEL=kms-master-key

go run ./cmd/kms-server
```

### AWS KMS

```bash
set KMS_HSM_TYPE=aws
set KMS_AWS_KEY_ID=arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012
set KMS_AWS_REGION=us-east-1
set AWS_ACCESS_KEY_ID=your_key
set AWS_SECRET_ACCESS_KEY=your_secret

go run ./cmd/kms-server
```

### Azure Key Vault

```bash
set KMS_HSM_TYPE=azure
set KMS_AZURE_VAULT_URL=https://myvault.vault.azure.net/
set KMS_AZURE_KEY_NAME=kms-master-key
set AZURE_CLIENT_ID=your_client_id
set AZURE_CLIENT_SECRET=your_secret
set AZURE_TENANT_ID=your_tenant_id

go run ./cmd/kms-server
```

## 詳細文件

請參考 [HSM 整合完整指南](docs/HSM_INTEGRATION.md)

