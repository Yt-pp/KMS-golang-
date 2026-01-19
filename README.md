## KMS (Go + gRPC) – POC

This project is a small KMS service and ETL worker for encrypting sensitive
fields (e.g. credit card number and CVV) coming from multiple databases and
storing the encrypted values in your own DWH database.

### 🚀 Quick Start

**5 minutes to get started!**

```powershell
# 1. Start services
.\start-kms.ps1

# 2. Test system (in new terminal)
.\test-system.ps1
```

See [Quick Start Guide](docs/QUICK_START.md) and [Complete Run Guide](docs/RUN_SYSTEM_GUIDE.md) for details.

### Components

- **KMS gRPC service** (`cmd/kms-server`)
  - Loads a local master key (AES‑256) from `master.key`.
  - Exposes `Encrypt` and `Decrypt` RPCs.
- **KMS HTTP REST API** (`cmd/kms-http-server`)
  - HTTP wrapper around gRPC service for easy integration.
  - Supports single and batch encryption endpoints.
  - **Perfect for SSIS integration** - see [SSIS Integration Guide](docs/SSIS_INTEGRATION.md)
- **ETL worker** (`cmd/etl-worker`)
  - Connects to source DB(s), reads card data.
  - Calls the KMS via gRPC to encrypt PAN and CVV.
  - Writes encrypted data into a target DB.

### Master key

**Option 1: File-based key (Development/Testing)**
Create a 32‑byte random key and store it as hex in `master.key`:

```bash
openssl rand -hex 32 > master.key
```

Keep this file secure and back it up appropriately.

**Option 2: HSM (Production)**
For production environments, use HSM for enhanced security:
- **PKCS#11**: Hardware HSM (Thales, SafeNet, SoftHSM)
- **AWS KMS**: Cloud HSM service
- **Azure Key Vault**: Cloud HSM service

See [HSM Integration Guide](docs/HSM_INTEGRATION.md) and [README_HSM.md](README_HSM.md) for details.

### Generate gRPC code

You need `protoc` with the Go plugins installed. Then run:

```bash
protoc --go_out=. --go-grpc_out=. proto/kms.proto
```

This will generate Go code under `proto/` which is used by the server and ETL.

### Run the KMS server

**Option 1: gRPC Server (for service-to-service communication)**
```bash
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server
```

**Option 2: HTTP REST API Server (for SSIS and HTTP clients)**
```bash
# Terminal 1: Start gRPC server
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server

# Terminal 2: Start HTTP REST API wrapper
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_HTTP_ADDR=:8080
set KMS_BEARER_TOKEN=your_token_here  # Optional, if JWT is enabled
go run ./cmd/kms-http-server
```

The HTTP server exposes REST endpoints:
- `POST /api/v1/encrypt` - Single encryption
- `POST /api/v1/encrypt/batch` - Batch encryption (high performance, **recommended for SSIS**)
- `POST /api/v1/decrypt` - Decryption
- `GET /health` - Health check

See [SSIS Integration Guide](docs/SSIS_INTEGRATION.md) for detailed SSIS setup instructions.

### Run the ETL worker

You can switch DBs via env vars. Supported drivers out of the box:
- `sqlserver` (default) using `github.com/microsoft/go-mssqldb`
- `mysql` using `github.com/go-sql-driver/mysql`

**Example – SQL Server**

```bash
set SRC_DB_DRIVER=sqlserver
set SRC_DB_DSN=sqlserver://user:password@localhost:1433?database=source_db&encrypt=disable
set DST_DB_DRIVER=sqlserver
set DST_DB_DSN=sqlserver://user:password@localhost:1433?database=kms_db&encrypt=disable
set KMS_GRPC_ADDR=127.0.0.1:50051
go run ./cmd/etl-worker
```

**Example – MySQL**

```bash
set SRC_DB_DRIVER=mysql
set SRC_DB_DSN=user:password@tcp(127.0.0.1:3306)/source_db
set DST_DB_DRIVER=mysql
set DST_DB_DSN=user:password@tcp(127.0.0.1:3306)/kms_db
set KMS_GRPC_ADDR=127.0.0.1:50051
go run ./cmd/etl-worker
```

Adapt the SQL queries in `cmd/etl-worker/main.go` to your three source systems
and your DWH schema.

---

## KMS (Go + gRPC) – 中文说明

本项目提供一个轻量的 KMS 服务和 ETL 工作者，用于从多个数据源读取敏感字段（如银行卡号、CVV），通过 gRPC 调用 KMS 加密后，写入到自有 DWH 数据库。

### 组件
- **KMS gRPC 服务**（`cmd/kms-server`）
  - 从本地 `master.key` 读取 AES‑256 主密钥。
  - 提供 `Encrypt` / `Decrypt` RPC。
- **ETL 工作者**（`cmd/etl-worker`）
  - 连接源库读取卡数据。
  - 调用 KMS 加密 PAN 和 CVV。
  - 将密文写入目标库。

### 主密钥
生成 32 字节随机密钥（十六进制存储）：

```bash
openssl rand -hex 32 > master.key
```

请妥善保护并备份此文件。

### 生成 gRPC 代码

确保安装了 `protoc` 和 Go 插件，运行：

```bash
protoc --go_out=. --go-grpc_out=. proto/kms.proto
```

生成的代码在 `proto/` 目录中，供服务和 ETL 使用。

### 运行 KMS 服务

```bash
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server
```

### 运行 ETL 工作者（可切换 DB 驱动）

已内置驱动：
- `sqlserver`（默认，使用 `github.com/microsoft/go-mssqldb`）
- `mysql`（使用 `github.com/go-sql-driver/mysql`）

**SQL Server 示例**

```bash
set SRC_DB_DRIVER=sqlserver
set SRC_DB_DSN=sqlserver://user:password@localhost:1433?database=source_db&encrypt=disable
set DST_DB_DRIVER=sqlserver
set DST_DB_DSN=sqlserver://user:password@localhost:1433?database=kms_db&encrypt=disable
set KMS_GRPC_ADDR=127.0.0.1:50051
go run ./cmd/etl-worker
```

**MySQL 示例**

```bash
set SRC_DB_DRIVER=mysql
set SRC_DB_DSN=user:password@tcp(127.0.0.1:3306)/source_db
set DST_DB_DRIVER=mysql
set DST_DB_DSN=user:password@tcp(127.0.0.1:3306)/kms_db
set KMS_GRPC_ADDR=127.0.0.1:50051
go run ./cmd/etl-worker
```

请根据三套源系统与 DWH 的表结构，修改 `cmd/etl-worker/main.go` 中的查询和写入 SQL。

---

## HSM 整合

本项目支持多种 HSM（硬件安全模块）后端：

- **PKCS#11**: 硬件 HSM（Thales, SafeNet, SoftHSM）
- **AWS KMS**: 云端 HSM 服务
- **Azure Key Vault**: 云端 HSM 服务

### 快速开始

```bash
# PKCS#11 HSM
set KMS_HSM_TYPE=pkcs11
set KMS_PKCS11_LIB=C:\path\to\pkcs11.dll
set KMS_PKCS11_SLOT=0
set KMS_PKCS11_PIN=1234

# AWS KMS
set KMS_HSM_TYPE=aws
set KMS_AWS_KEY_ID=arn:aws:kms:...

# Azure Key Vault
set KMS_HSM_TYPE=azure
set KMS_AZURE_VAULT_URL=https://myvault.vault.azure.net/

go run ./cmd/kms-server
```

详细说明请参考：
- [HSM 整合完整指南](docs/HSM_INTEGRATION.md)
- [HSM 快速参考](README_HSM.md)

---

## SSIS 整合指南

本项目支持与 Microsoft SSIS (SQL Server Integration Services) 整合，用于在 ETL 流程中加密 PAN 数据。

### 快速开始

1. **启动 HTTP REST API 服务**（见上方说明）
2. **在 SSIS 中使用 C# Script Component** 调用 REST API
3. **使用批次 API** 以获得最佳性能（10-20倍提升）

详细说明请参考：
- [SSIS 整合完整指南](docs/SSIS_INTEGRATION.md)
- [SSIS C# Script 示例代码](docs/SSIS_SCRIPT_EXAMPLE.cs)
- [性能优化指南](docs/PERFORMANCE_OPTIMIZATION.md)

### 性能对比

- **单笔 API**: ~100-200 req/s
- **批次 API (100笔/批)**: ~1000-2000 req/s ⚡ **推荐用于 SSIS ETL**


