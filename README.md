## KMS (Go + gRPC) – POC

This project is a small KMS service and ETL worker for encrypting sensitive
fields (e.g. credit card number and CVV) coming from multiple databases and
storing the encrypted values in your own DWH database.

### Components

- **KMS gRPC service** (`cmd/kms-server`)
  - Loads a local master key (AES‑256) from `master.key`.
  - Exposes `Encrypt` and `Decrypt` RPCs.
- **ETL worker** (`cmd/etl-worker`)
  - Connects to source DB(s), reads card data.
  - Calls the KMS via gRPC to encrypt PAN and CVV.
  - Writes encrypted data into a target DB.

### Master key

Create a 32‑byte random key and store it as hex in `master.key`:

```bash
openssl rand -hex 32 > master.key
```

Keep this file secure and back it up appropriately.

### Generate gRPC code

You need `protoc` with the Go plugins installed. Then run:

```bash
protoc --go_out=. --go-grpc_out=. proto/kms.proto
```

This will generate Go code under `proto/` which is used by the server and ETL.

### Run the KMS server

```bash
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
go run ./cmd/kms-server
```

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


