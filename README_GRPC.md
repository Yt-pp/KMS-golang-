## How Go + gRPC Are Used in This KMS System

This document explains the roles of Go, gRPC, and supporting components in the
KMS/ETL setup, and how services interact.

### Architecture at a Glance
- **KMS Service (gRPC)**
  - Provides `Encrypt` / `Decrypt` via gRPC (`proto/kms.proto`).
  - Uses an AES-256 master key (local file for demo; can be HSM/Cloud KMS later).
  - Optional JWT auth interceptor protects RPCs.
- **Auth Service (gRPC)**
  - `Auth/Login` issues a JWT (HS256) after username/password check (demo only).
  - Token is validated by the KMS interceptor for subsequent calls.
- **ETL Worker**
  - Reads source DB rows, calls KMS `Encrypt`, writes ciphertext+nonce to DWH.
  - Sends `Authorization: Bearer <token>` if `KMS_BEARER_TOKEN` is set.
- **(Optional) Reporting/API Layer**
  - Can expose HTTP to users/tools; internally calls KMS gRPC for decrypt/masking.

### Flow: Encrypt (ETL)
1) ETL connects to source DB, selects rows to migrate.
2) For each sensitive field (PAN, CVV), ETL calls `kms.KMS/Encrypt` over gRPC.
3) KMS:
   - Validates JWT (if `KMS_JWT_SECRET` set) via unary interceptor.
   - Uses AES-GCM with random nonce to encrypt plaintext.
   - Returns `ciphertext` + `nonce`.
4) ETL stores `(ciphertext, nonce, source_id, other_data)` in `encrypted_cards`.

### Flow: Decrypt (Reporting or Spot Check)
1) A trusted service (or grpcurl) reads `ciphertext` + `nonce` from DWH.
2) Sends gRPC `kms.KMS/Decrypt` with token in `Authorization` header.
3) KMS validates JWT, decrypts via AES-GCM, returns plaintext.
4) Caller masks or uses plaintext as allowed (e.g., show `**** **** **** 1234`).

### Why gRPC Here?
- **Strongly typed contracts**: `proto/kms.proto` defines messages/services.
- **Performance**: Binary Protobuf over HTTP/2; good for service-to-service calls.
- **Interceptors**: Easy to attach auth/logging/retry on both server/client.
- **Codegen**: `protoc --go_out --go-grpc_out` generates Go stubs for client/server.

### Where the Code Lives
- `proto/kms.proto`: service definitions (`KMS`, `Auth`), messages, go_package option.
- `internal/kms/crypto.go`: AES-GCM manager loading master key from file.
- `internal/auth/jwt.go`: JWT validation interceptor + token issuance helper.
- `internal/server/server.go`: gRPC server bootstrap, registers KMS + Auth services.
- `internal/server/auth_server.go`: Implements `Auth/Login`.
- `cmd/kms-server/main.go`: Loads config, installs JWT interceptor, starts server.
- `cmd/etl-worker/main.go`: gRPC client; encrypts fields and writes to DWH.

### Config You Touch for gRPC
- **Server**
  - `KMS_GRPC_ADDR` (e.g., `:50051`)
  - `KMS_MASTER_KEY_PATH` (e.g., `master.key`)
  - `KMS_JWT_SECRET` (+ optional `KMS_JWT_AUD`, `KMS_JWT_ISS`)
  - Demo login creds: `KMS_DEMO_USER`, `KMS_DEMO_PASS` (default `demo` / `demo123`)
- **Client (ETL)**
  - `KMS_GRPC_ADDR`
  - `KMS_BEARER_TOKEN` (from `Auth/Login`)
  - DB driver/DSN envs (`SRC_DB_DRIVER/DSN`, `DST_DB_DRIVER/DSN`)

### Running Sequence (Quick)
1) `openssl rand -hex 32 > master.key`
2) Start KMS with JWT:
   ```bash
   set KMS_MASTER_KEY_PATH=master.key
   set KMS_GRPC_ADDR=:50051
   set KMS_JWT_SECRET=demo-secret
   set KMS_JWT_AUD=kms-demo
   set KMS_JWT_ISS=demo
   go run ./cmd/kms-server
   ```
3) Get token via gRPC login:
   **PowerShell (Windows):**
   ```powershell
   cmd /c 'echo {"username":"demo","password":"demo123"} | grpcurl -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login'
   ```
   **Bash/Linux/Mac:**
   ```bash
   grpcurl -plaintext -d '{"username":"demo","password":"demo123"}' \
     127.0.0.1:50051 kms.Auth/Login
   ```
4) Run ETL with token:
   ```bash
   set KMS_BEARER_TOKEN=<token>
   set SRC_DB_DRIVER=sqlserver
   set SRC_DB_DSN=sqlserver://user:password@localhost:1433?database=source_db&encrypt=disable
   set DST_DB_DRIVER=sqlserver
   set DST_DB_DSN=sqlserver://user:password@localhost:1433?database=kms_db&encrypt=disable
   set KMS_GRPC_ADDR=127.0.0.1:50051
   go run ./cmd/etl-worker
   ```

### Security Notes
- KMS should stay minimal: only encrypt/decrypt; no direct DB queries.
- Store master key in HSM/Cloud KMS in real environments; file key is for demo.
-,Enable TLS/mTLS for transport; today’s setup uses plaintext for demo simplicity.
- JWT secret must be strong; in production, prefer OIDC/JWKS instead of HS256 shared secret.

### Extending
- Add HTTP/REST gateway if you need browser or SAS tools: the gateway calls gRPC KMS internally.
- Add RBAC: issue role claims in JWT, enforce per-RPC policy in the interceptor.
- Add metrics/tracing: wrap gRPC interceptors with Prometheus/OTel.


