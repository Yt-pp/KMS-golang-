# PowerShell 中使用 grpcurl 的说明

## 为什么 PowerShell 中需要特殊处理？

### gRPC 服务器工作正常 ✅

你的 gRPC 服务器**完全正常工作**：
- ✅ gRPC 协议正常
- ✅ 反射 API 已启用
- ✅ 服务可以被发现：`kms.Auth`, `kms.KMS`
- ✅ 服务器可以正常处理请求

### 问题在于 PowerShell 的参数传递方式

**这不是 gRPC 的问题，而是 PowerShell 如何传递 JSON 字符串给命令行工具的问题。**

## 问题根源

### 1. PowerShell 的字符串处理方式

PowerShell 对单引号和双引号的处理与 Bash 不同：

```powershell
# PowerShell 中，单引号内的内容会被视为字面量
'd{"username":"demo"}'  # PowerShell 可能错误解析

# Bash 中，单引号正常工作
'{"username":"demo"}'   # Bash 正确解析
```

### 2. grpcurl 期望的输入格式

`grpcurl` 是一个命令行工具，它期望：
- 通过 `-d` 参数接收 JSON 字符串
- 或者通过 stdin (`-d @`) 接收 JSON

### 3. PowerShell 传递参数时的编码问题

当 PowerShell 将包含特殊字符（如 `{`, `}`, `:`）的字符串传递给外部程序时，可能会：
- 错误地转义字符
- 改变字符串的编码
- 导致 JSON 解析失败

## 解决方案

### 方法 1：使用 cmd.exe（推荐）✅

```powershell
cmd /c 'echo {"username":"demo","password":"demo123"} | grpcurl -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login'
```

**为什么有效？**
- `cmd.exe` 使用 Windows 的命令行解析器，能正确处理 JSON
- 管道 (`|`) 将 JSON 传递给 `grpcurl` 的 stdin (`-d @`)
- 避免了 PowerShell 的字符串处理问题

### 方法 2：使用临时文件

```powershell
'{"username":"demo","password":"demo123"}' | Out-File -FilePath request.json -Encoding utf8 -NoNewline
cmd /c 'type request.json | grpcurl -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login'
```

### 方法 3：使用 PowerShell 变量（需要转义）

```powershell
$json = '{"username":"demo","password":"demo123"}'
cmd /c "echo $json | grpcurl.exe -plaintext -d @ 127.0.0.1:50051 kms.Auth/Login"
```

## 验证 gRPC 服务器正常工作

你可以用这些命令验证服务器工作正常：

```powershell
# 列出所有服务（不需要 JSON，所以可以直接用）
grpcurl -plaintext 127.0.0.1:50051 list

# 描述服务
grpcurl -plaintext 127.0.0.1:50051 describe kms.Auth

# 描述方法
grpcurl -plaintext 127.0.0.1:50051 describe kms.Auth.Login
```

这些命令都能正常工作，证明：
- ✅ gRPC 服务器运行正常
- ✅ 反射 API 工作正常
- ✅ 网络连接正常

## 总结

| 组件 | 状态 | 说明 |
|------|------|------|
| gRPC 服务器 | ✅ 正常 | 完全正常工作 |
| gRPC 协议 | ✅ 正常 | 协议通信正常 |
| 反射 API | ✅ 正常 | 已启用并工作 |
| PowerShell | ⚠️ 需要特殊处理 | 字符串传递方式不同 |
| grpcurl 工具 | ✅ 正常 | 工具本身正常 |

**结论：** 你的 gRPC 服务器没有问题！问题只是 PowerShell 如何传递 JSON 参数给 `grpcurl`。使用 `cmd.exe` 可以完美解决这个问题。

