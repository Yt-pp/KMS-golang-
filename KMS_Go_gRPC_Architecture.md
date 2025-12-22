## Go + gRPC + KMS – How It Manages Your Sensitive Data

### 1. From `.proto` to Go code and network bytes (encoding flow)

- **Step 1 – Define schema in `proto/kms.proto`**
  - You describe services and messages in a language‑neutral way:
    - `service KMS { rpc Encrypt(EncryptRequest) returns (EncryptResponse); }`
    - `message EncryptRequest { bytes plaintext = 1; string key_id = 2; }`

- **Step 2 – Generate Go types and gRPC stubs**
  - Run:
    - `protoc --go_out=. --go-grpc_out=. proto/kms.proto`
  - This creates:
    - `kms.pb.go`: Go structs like `EncryptRequest`, `EncryptResponse`
    - `kms_grpc.pb.go`: interfaces/clients like `KMSServer`, `KMSClient`

- **Step 3 – Server: you implement business logic only**
  - In `internal/server/server.go`, you implement:
    - `func (s *KMSServer) Encrypt(ctx context.Context, req *kmsproto.EncryptRequest) (*kmsproto.EncryptResponse, error)`
  - Inside, you work with normal Go structs:
    - `req.GetPlaintext()` (Go `[]byte`)
    - return `&kmsproto.EncryptResponse{Ciphertext: ct, Nonce: nonce}`
  - **gRPC + Protobuf automatically:**
    - receives network bytes
    - decodes Protobuf → fills `EncryptRequest`
    - encodes `EncryptResponse` → sends as Protobuf bytes over HTTP/2

- **Step 4 – Client: also just uses Go structs**
  - ETL or test client does:
    - `client := kmsproto.NewKMSClient(conn)`
    - `resp, err := client.Encrypt(ctx, &kmsproto.EncryptRequest{Plaintext: data})`
  - gRPC client encodes the struct to Protobuf and sends it; when a response arrives, it is decoded back into a Go struct for you.

> **Key idea:** you only think in terms of Go structs and proto definitions.  
> The binary encoding/decoding is fully handled by the generated code and gRPC runtime.

---

### 2. How auth works now (JWT) and how it relates to OAuth2

#### 2.1 Current model: internal JWT, HS256

- **Login flow**
  - RPC: `kms.Auth/Login` (implemented in `internal/server/auth_server.go`)
  - Input: username + password (default `demo` / `demo123`, or from `KMS_DEMO_USER` / `KMS_DEMO_PASS`)
  - On success:
    - calls `auth.IssueToken(jwtCfg, username, tokenTTL)`
    - returns a **JWT (HS256)** in `LoginResponse.token`

- **Protection of KMS methods**
  - `internal/auth/jwt.go` defines `UnaryServerInterceptor(jwtCfg)`:
    - runs **before** any RPC handler
    - skips auth for `/Auth/Login` so clients can obtain a token
    - for all other RPCs:
      - reads `authorization: Bearer <token>` from gRPC metadata
      - verifies HS256 signature with `jwtCfg.Secret` (`KMS_JWT_SECRET`)
      - optionally validates `aud` (`KMS_JWT_AUD`) and `iss` (`KMS_JWT_ISS`)
      - if token is invalid/missing → returns `Unauthenticated`
      - if valid → allows the call to `Encrypt` / `Decrypt`

> This is a **self‑contained JWT auth** setup: the KMS server itself issues and validates tokens.  
> It is suitable for demos or internal systems, but it is **not** a full OAuth2/OIDC provider.

#### 2.2 Relation to “real” OAuth2 / OIDC

- In a full OAuth2/OIDC architecture:
  - An external Identity Provider (IdP) (e.g. Azure AD, Auth0, Keycloak) issues access tokens.
  - Your KMS acts as a **resource server**:
    - it does **not** issue tokens
    - it only **validates** tokens from the IdP

- To evolve this project towards OAuth2/OIDC:
  - Keep the existing gRPC services (`KMS`, `Auth` or drop `Auth` later).
  - Replace `auth.IssueToken` + HS256 validation with:
    - JWT validation against an IdP’s JWKS (public keys) using RS256/ES256
    - validation of `aud` (KMS identifier), `iss` (IdP URL), `exp`, `scope/roles`
  - The rest of the KMS code (crypto, gRPC methods, ETL usage) can stay the same.

> So, the current system already has the **shape** of an OAuth2‑protected resource server.  
> Swapping in a real IdP mainly affects the JWT issuing/validation parts in `internal/auth`.

---

### 3. Mental model: how Go + gRPC form your KMS

- **Go** gives you:
  - a fast, simple runtime to implement the KMS logic and crypto
  - easy deployment as a single binary (`kms-server`)

- **gRPC + Protobuf** give you:
  - a strongly‑typed, language‑neutral API to expose `Encrypt`/`Decrypt`
  - efficient binary encoding for high‑volume ETL traffic
  - interceptors for auth, logging, rate limiting, etc.

- **JWT (today) / OAuth2 (future)** give you:
  - a way to control **who** can call `Encrypt`/`Decrypt`
  - a clean upgrade path from demo tokens → enterprise SSO / IdP

In short, your KMS Server is:
- a **Go service** that owns the encryption keys and algorithms
- exposed via **gRPC** so ETL / other services can call it efficiently
- protected by **JWT today**, and ready to be upgraded to full OAuth2/OIDC later.


