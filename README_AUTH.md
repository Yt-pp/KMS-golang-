## KMS (Go + gRPC) – JWT/OAuth2 Demo Guide

This guide shows how to run the KMS with JWT (HS256) auth and test calls from
the ETL worker (or any gRPC client). It adds security on top of the POC without
replacing the original README.

### What changed (code)
- Server: optional JWT auth interceptor. Enable by setting `KMS_JWT_SECRET`
  (with optional `KMS_JWT_AUD`, `KMS_JWT_ISS`).
- Client (ETL): can send `Authorization: Bearer <token>` when
  `KMS_BEARER_TOKEN` is set.
- No TLS is configured here—use a real cert for production.

### 1) Generate a demo JWT (HS256)

You can generate a token using any JWT tool OR use the built-in `Auth/Login`
gRPC method. For a quick manual token (no login), example (Node one-liner):

```bash
node -e "const jwt=require('jsonwebtoken'); console.log(jwt.sign({sub:'etl', aud:'kms-demo', iss:'demo'}, 'demo-secret', {expiresIn:'1h'}));"
```

Save the printed token as `DEMO_TOKEN`.

#### Or: use gRPC login to get a token

The KMS exposes an `Auth` service with `Login`:

- Default demo user: `demo`
- Default demo password: `demo123`

You can obtain a token via `grpcurl`:

**PowerShell (Windows):**

Due to PowerShell's handling of JSON strings, use one of these methods:

**Method 1: Using cmd.exe (Recommended)**
```powershell
cmd /c 'echo {"username":"demo","password":"demo123"} | grpcurl -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login'
```

**Method 2: Using a temporary file**
```powershell
'{"username":"demo","password":"demo123"}' | Out-File -FilePath request.json -Encoding utf8 -NoNewline
cmd /c 'type request.json | grpcurl -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login'
```

**Bash/Linux/Mac:**
```bash
grpcurl -plaintext \
  -d '{"username":"demo","password":"demo123"}' \
  127.0.0.1:50051 kms.Auth/Login
```

The response JSON will contain a `token` field; copy it as `DEMO_TOKEN`.

### 2) Start KMS server with JWT enabled

```bash
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
set KMS_JWT_SECRET=demo-secret
set KMS_JWT_AUD=kms-demo
set KMS_JWT_ISS=demo
go run ./cmd/kms-server
```

If `KMS_JWT_SECRET` is not set, auth is disabled (not recommended beyond local testing).

### 3) Run ETL worker with bearer token

```bash
set SRC_DB_DRIVER=sqlserver
set SRC_DB_DSN=sqlserver://user:password@localhost:1433?database=source_db&encrypt=disable
set DST_DB_DRIVER=sqlserver
set DST_DB_DSN=sqlserver://user:password@localhost:1433?database=kms_db&encrypt=disable
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_BEARER_TOKEN=DEMO_TOKEN   # put the token from step 1
go run ./cmd/etl-worker
```

For MySQL, switch drivers/DSNs as in the main README:

```bash
set SRC_DB_DRIVER=mysql
set SRC_DB_DSN=user:password@tcp(127.0.0.1:3306)/source_db
set DST_DB_DRIVER=mysql
set DST_DB_DSN=user:password@tcp(127.0.0.1:3306)/kms_db
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_BEARER_TOKEN=DEMO_TOKEN
go run ./cmd/etl-worker
```

### 4) gRPC curl test (optional)

If you have `grpcurl`, you can test Encrypt directly:

**PowerShell (Windows):**
```powershell
cmd /c 'echo {"plaintext":"cGF5bG9hZA=="} | grpcurl -plaintext -H "authorization: Bearer DEMO_TOKEN" -d @ 127.0.0.1:50051 kms.KMS/Encrypt'
```

**Bash/Linux/Mac:**
```bash
grpcurl -plaintext \
  -H "authorization: Bearer DEMO_TOKEN" \
  -d '{"plaintext":"cGF5bG9hZA=="}' \
  127.0.0.1:50051 kms.KMS/Encrypt
```

`cGF5bG9hZA==` is base64 for `payload`.

### 5) Notes for a more enterprise-ready setup
- Use TLS/mTLS for transport encryption and peer auth.
- Prefer an OAuth2/OIDC provider (Auth0, Azure AD, Okta) to mint tokens instead
  of a shared secret; plug its JWKS into validation.
- Rotate secrets/keys regularly; store secrets in a vault.
- Add structured audit logging (who, when, which operation, key_id).
- Apply rate limits and per-caller authorization (RBAC).

---

## KMS (Go + gRPC) – JWT/OAuth2 演示指南（中文）

本指南说明如何在 KMS 上启用 JWT（HS256）校验，并从 ETL 工作者或其他 gRPC 客户端带 Token 调用，实现在原 POC 之上增加一层访问控制。

### 代码里的变化
- 服务端：增加可选的 JWT 校验拦截器。设置环境变量 `KMS_JWT_SECRET`
 （可选 `KMS_JWT_AUD`、`KMS_JWT_ISS`）即可启用。
- 客户端（ETL）：当设置 `KMS_BEARER_TOKEN` 时，会自动在请求头加上
  `Authorization: Bearer <token>`。
- 当前示例未配置 TLS，线上环境建议启用 TLS/mTLS。

### 1）生成一个示例 JWT（HS256）

可以使用任意 JWT 工具，或者使用内置的 `Auth/Login` gRPC 登录接口。

下面是不用登录、直接生成 Token 的 Node 一行脚本示例：

```bash
node -e "const jwt=require('jsonwebtoken'); console.log(jwt.sign({sub:'etl', aud:'kms-demo', iss:'demo'}, 'demo-secret', {expiresIn:'1h'}));"
```

把打印出来的内容保存成 `DEMO_TOKEN`（字符串即可）。

#### 或者：先登录，再拿到 Token

KMS 暴露了一个 `Auth` 服务，提供 `Login` 方法：

- 默认演示用户名：`demo`
- 默认演示密码：`demo123`

可以用 `grpcurl` 登录获取 Token：

**PowerShell (Windows):**

由于 PowerShell 对 JSON 字符串的处理方式，推荐使用以下方法之一：

**方法 1：使用 cmd.exe（推荐）**
```powershell
cmd /c 'echo {"username":"demo","password":"demo123"} | grpcurl -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login'
```

**方法 2：使用临时文件**
```powershell
'{"username":"demo","password":"demo123"}' | Out-File -FilePath request.json -Encoding utf8 -NoNewline
cmd /c 'type request.json | grpcurl -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login'
```

**方法 3：使用变量和 cmd.exe**
```powershell
$json = '{"username":"demo","password":"demo123"}'
cmd /c "echo $json | grpcurl.exe -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login"
```

**Bash/Linux/Mac:**
```bash
grpcurl -plaintext \
  -d '{"username":"demo","password":"demo123"}' \
  127.0.0.1:50051 kms.Auth/Login
```

返回的 JSON 里会有一个 `token` 字段，把这个值复制出来作为 `DEMO_TOKEN`。

### 2）启动带 JWT 校验的 KMS 服务

```bash
set KMS_MASTER_KEY_PATH=master.key
set KMS_GRPC_ADDR=:50051
set KMS_JWT_SECRET=demo-secret
set KMS_JWT_AUD=kms-demo
set KMS_JWT_ISS=demo
go run ./cmd/kms-server
```

说明：
- `KMS_JWT_SECRET`：JWT 的签名密钥（这里用 HS256 对称密钥）。服务端用它校验 Token 是否被篡改。
- `KMS_JWT_AUD` / `KMS_JWT_ISS`：可选，用于校验 JWT 的受众（aud）和签发方（iss）。
- 如果 **没有设置** `KMS_JWT_SECRET`，则不会做 JWT 校验，相当于“任何客户端都能调用”（只适合本地测试）。

### 3）带 Token 运行 ETL（SQL Server 示例）

```bash
set SRC_DB_DRIVER=sqlserver
set SRC_DB_DSN=sqlserver://user:password@localhost:1433?database=source_db&encrypt=disable
set DST_DB_DRIVER=sqlserver
set DST_DB_DSN=sqlserver://user:password@localhost:1433?database=kms_db&encrypt=disable
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_BEARER_TOKEN=DEMO_TOKEN   # 第一步生成的 Token
go run ./cmd/etl-worker
```

### 4）MySQL 示例

```bash
set SRC_DB_DRIVER=mysql
set SRC_DB_DSN=user:password@tcp(127.0.0.1:3306)/source_db
set DST_DB_DRIVER=mysql
set DST_DB_DSN=user:password@tcp(127.0.0.1:3306)/kms_db
set KMS_GRPC_ADDR=127.0.0.1:50051
set KMS_BEARER_TOKEN=DEMO_TOKEN
go run ./cmd/etl-worker
```

### 5）使用 grpcurl 直接测试 Encrypt（可选）

**PowerShell (Windows):**
```powershell
cmd /c 'echo {"plaintext":"cGF5bG9hZA=="} | grpcurl -plaintext -H "authorization: Bearer DEMO_TOKEN" -d @ 127.0.0.1:50051 kms.KMS/Encrypt'
```

**Bash/Linux/Mac:**
```bash
grpcurl -plaintext \
  -H "authorization: Bearer DEMO_TOKEN" \
  -d '{"plaintext":"cGF5bG9hZA=="}' \
  127.0.0.1:50051 kms.KMS/Encrypt
```

其中 `cGF5bG9hZA==` 是字符串 `payload` 的 Base64。

### 6）`KMS_JWT_SECRET` 的作用总结
- 用于 **验证 JWT 签名**，确保 Token 是由可信方签发、没有被篡改。
- 没有正确的 Secret，即使伪造一个结构类似的 JWT，服务端也会校验失败并拒绝请求。
- 生产环境中：
  - Secret 应该足够复杂/随机，并存放在安全位置（Vault、Key Vault 等），不要写死在代码或 Git。
  - 建议进一步用 OAuth2/OIDC + 公钥验证（JWKS），由身份提供方签发 Token。

