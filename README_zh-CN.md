# ghttp

[English](./README.md) | 简体中文

一个用于快速接入 REST API 的 Go HTTP 客户端。

## 安装

```shell
go get github.com/nexuer/ghttp
```

## 快速开始

```go
client := ghttp.NewClient(
    ghttp.WithEndpoint("https://gitlab.com"),
    ghttp.WithTimeout(10*time.Second),
)

var projects []Project
_, err := client.Invoke(
    context.Background(),
    http.MethodGet,
    "/api/v4/projects",
    nil,
    &projects,
    ghttp.Query(map[string]any{"membership": true}),
)
```

需要直接访问响应流时使用 `Do`。调用方使用完后应该关闭 Response Body。

```go
req, err := http.NewRequest(http.MethodGet, "https://example.com/data", nil)
if err != nil {
    return err
}
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()

body, err := io.ReadAll(resp.Body)
```

## 客户端配置

### Transport、TLS 与代理

`WithTransport(transport http.RoundTripper)` 用于设置 HTTP Transport，默认值为
`http.DefaultTransport`。

`WithTLSConfig(cfg *tls.Config)` 和
`WithProxy(f func(*http.Request) (*url.URL, error))` 只会应用到
`*http.Transport`。

```go
client := ghttp.NewClient(
    ghttp.WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}),
    ghttp.WithProxy(ghttp.ProxyURL("127.0.0.1:7890")),
)
```

使用自定义 `RoundTripper` 实现时，需要直接在该 Transport 上配置 TLS 和代理。

### 超时

`WithTimeout(timeout time.Duration)` 设置默认的端到端超时，默认不设置超时。调用方
Context 已经带有 deadline 时，不会覆盖它。

对于 `Do`，客户端设置的超时会在 Response Body 到达 EOF 或被关闭时释放。即使不
读取 Body，也应该将其关闭。

### Response Body 大小限制

`WithMaxResponseBodyBytes(n int64)` 限制 `Invoke` 响应和已配置的非 2xx 错误在
解码时缓冲的 Body 大小。默认值为 `0`，表示不限制；非正数都会禁用限制。`Do`
返回的成功响应流不会被限制。

超过限制时，可以通过 `ghttp.IsResponseBodyTooLarge(err)` 判断。通过 Client 调用时，
返回的 `*ghttp.Error` 仍会携带请求和响应状态信息。

### Endpoint 与 Header

- `WithEndpoint(endpoint string)` 设置基础地址。
- `WithUserAgent(userAgent string)` 设置默认 User-Agent。
- `WithContentType(contentType string)` 设置默认 Accept 和 Content-Type，默认值为
  `application/json`。

### 代理

`ProxyURL(address string)` 可以将常用代理地址转换为 `http.ProxyURL` 函数。

```go
ghttp.WithProxy(ghttp.ProxyURL(":7890"))
ghttp.WithProxy(http.ProxyFromEnvironment)
```

### 限流器

`WithLimiter(l Limiter)` 用于设置阻塞式出站请求限流器。

```go
type Limiter interface {
    Wait(ctx context.Context) error
}
```

限流器在请求实际发送前执行。本地准备失败不会消耗限流额度，等待时间属于客户端
超时预算。

### 非 2xx 错误

`WithNot2xxError(f func() error)` 创建用于解析非 2xx 响应 Body 的错误值。

### Debug

`WithDebug(true)` 启用默认的 curl 风格 Debug。`WithDebugger` 只有在同时启用
`WithDebug(true)` 时才会生效。

```go
client := ghttp.NewClient(
    ghttp.WithDebug(true),
    ghttp.WithDebugger(func() ghttp.Debugger {
        return &ghttp.Debug{
            Writer:            os.Stderr,
            Trace:             true,
            RequestBodyLimit:  64 << 10,
            ResponseBodyLimit: 64 << 10,
            TraceCallback: func(w io.Writer, info ghttp.TraceInfo) {
                _, _ = w.Write(info.Table())
            },
        }
    }),
)
```

请求和响应 Body limit 使用相同语义：

| 值 | 行为 |
| ---: | --- |
| `0` | 使用默认的 64 KiB |
| `> 0` | 最多捕获指定字节数 |
| `< 0` | 禁用对应方向的 Body 日志 |

- Request Body 预览使用 `GetBody`；流式、multipart 和二进制 Body 不会被消费。
- Response Body 会在调用方读取时输出；`Do` 的响应未被读取时不会输出 Body。
- Header 不会自动脱敏。
- Trace 包含 DNS、TCP、TLS、连接复用、请求写入、TTFB 和响应头耗时；未发生的阶段
  显示为 `-`。

## 请求级 Option

`Invoke` 和 `Do` 都可以接收请求级 Option：

```go
resp, err := client.Invoke(ctx, http.MethodPost, "/message", "hello", &reply,
    ghttp.ContentType("text/plain"),
    ghttp.Query(map[string]any{"verbose": true}),
    ghttp.BearerToken(token),
    ghttp.Before(func(req *http.Request) error {
        req.Header.Set("X-Request-ID", requestID)
        return nil
    }),
)
```

内置 Option 包括：

- `Query(value)`
- `ContentType(contentType)`
- `BasicAuth(username, password)`
- `BearerToken(token)`
- `Before(hooks...)`
- `After(hooks...)`

`ContentType` 在 `Invoke` 中选择请求 Codec，并同时设置 `Content-Type` 和 `Accept`。
在 `Do` 中只修改 Header，已有 Body 由调用方负责。结构化 Option 会在 `Before` hook
之前应用。重复设置 Query 或认证 Option 时，最后一个非空值生效。

Query tag 和嵌套值规则请参阅[查询参数编码](./query/README_zh-CN.md)。

## 编解码

Codec 根据 Content-Type subtype 选择。例如 `application/json` 和
`application/vnd.api+json` 都会解析为 JSON。

自定义 Codec 应该在开始并发使用前注册：

```go
type codec struct{}

func (codec) Name() string { return "sonic-json" }
func (codec) Marshal(v any) ([]byte, error) { return sonic.Marshal(v) }
func (codec) Unmarshal(data []byte, v any) error { return sonic.Unmarshal(data, v) }

func init() {
    ghttp.RegisterCodec("application/json", codec{})
}
```

查询辅助函数：

- `CodecForContentType(contentType string)`
- `CodecForRequest(request, headerName...)`
- `CodecForResponse(response, headerName...)`

## 错误处理

`*ghttp.Error` 是结构化客户端错误类型，通过 `Unwrap` 支持 `errors.Is` 和
`errors.As`。

```go
httpErr, ok := ghttp.FromError(err)
if ok {
    fmt.Println(httpErr.StatusCode)
    fmt.Println(httpErr.Request)
    fmt.Println(httpErr.Err)
}

statusCode, ok := ghttp.StatusCode(err)
```

`IsTimeout(err)` 可以识别 Context deadline 错误以及经过包装的 `net.Error` 超时。
