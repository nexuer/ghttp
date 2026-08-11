# ghttp

English | [简体中文](./README_zh-CN.md)

A Go HTTP client designed for quick integration with REST APIs.

## Installation

```shell
go get github.com/nexuer/ghttp
```

## Quick Start

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

Use `Do` when the caller needs direct access to the response stream. The caller
should close the response body after use.

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

## Client Options

### Transport, TLS, and Proxy

`WithTransport(transport http.RoundTripper)` sets the HTTP transport. The
default is `http.DefaultTransport`.

`WithTLSConfig(cfg *tls.Config)` and
`WithProxy(f func(*http.Request) (*url.URL, error))` are applied only when the
selected transport is `*http.Transport`.

```go
client := ghttp.NewClient(
    ghttp.WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}),
    ghttp.WithProxy(ghttp.ProxyURL("127.0.0.1:7890")),
)
```

When using a custom `RoundTripper` implementation, configure TLS and proxy on
that transport itself.

### Timeout

`WithTimeout(timeout time.Duration)` sets the default end-to-end timeout. No
timeout is applied by default. A deadline already present on the context is
preserved.

For `Do`, a client-managed timeout is released when the response body reaches
EOF or is closed. The response body should still be closed when it is not read.

### Response Body Limit

`WithMaxResponseBodyBytes(n int64)` limits response bodies buffered while
decoding `Invoke` replies and configured non-2xx errors. The default value is
`0`, which applies no limit. Non-positive values disable the limit. Successful
response streams returned by `Do` are not limited.

When the limit is exceeded, the returned error can be detected with
`ghttp.IsResponseBodyTooLarge(err)`. Calls through the client return a
`*ghttp.Error` carrying the request and response status context.

### Endpoint and Headers

- `WithEndpoint(endpoint string)` sets the base endpoint.
- `WithUserAgent(userAgent string)` sets the default User-Agent.
- `WithContentType(contentType string)` sets the default Accept and
  Content-Type. The default is `application/json`.

### Proxy

`ProxyURL(address string)` converts common proxy addresses into an
`http.ProxyURL` function.

```go
ghttp.WithProxy(ghttp.ProxyURL(":7890"))
ghttp.WithProxy(http.ProxyFromEnvironment)
```

### Rate Limiter

`WithLimiter(l Limiter)` installs a blocking outbound request limiter.

```go
type Limiter interface {
    Wait(ctx context.Context) error
}
```

The limiter runs immediately before the request is sent. Local preparation
failures do not consume limiter capacity, and limiter wait is part of the
client timeout.

### Non-2xx Errors

`WithNot2xxError(f func() error)` creates the value used to decode non-2xx
response bodies.

### Debugging

`WithDebug(true)` enables the default curl-style debugger. `WithDebugger` takes
effect only when `WithDebug(true)` is also enabled.

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

Body limit values have the same semantics:

| Value | Behavior |
| ---: | --- |
| `0` | Use the default 64 KiB limit |
| `> 0` | Capture at most this many bytes |
| `< 0` | Disable logging for that body direction |

- Request previews use `GetBody`; streaming, multipart, and binary bodies are
  not consumed.
- Response bytes are logged as the caller reads them. A `Do` response that is
  never read has no body output.
- Headers are not redacted.
- Trace output includes DNS, TCP, TLS, connection reuse, request write, TTFB,
  and time to response headers. Missing phases are shown as `-`.

## Invocation Options

Both `Invoke` and `Do` accept request-specific options:

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

Built-in options include:

- `Query(value)`
- `ContentType(contentType)`
- `BasicAuth(username, password)`
- `BearerToken(token)`
- `Before(hooks...)`
- `After(hooks...)`

`ContentType` selects the request codec for `Invoke` and sets both
`Content-Type` and `Accept`. With `Do`, it only changes the headers; the caller
owns the existing body. Structured options are applied before `Before` hooks.
Repeated Query or authentication options use the last non-empty value.

See [query encoding](./query/README.md) for query-tag and nested-value rules.

## Encoding

Codecs are selected from the Content-Type subtype. For example,
`application/json` and `application/vnd.api+json` both resolve to JSON.

Custom codecs should be registered before concurrent use:

```go
type codec struct{}

func (codec) Name() string { return "sonic-json" }
func (codec) Marshal(v any) ([]byte, error) { return sonic.Marshal(v) }
func (codec) Unmarshal(data []byte, v any) error { return sonic.Unmarshal(data, v) }

func init() {
    ghttp.RegisterCodec("application/json", codec{})
}
```

Lookup helpers:

- `CodecForContentType(contentType string)`
- `CodecForRequest(request, headerName...)`
- `CodecForResponse(response, headerName...)`

## Errors

`*ghttp.Error` is the structured client error type. It supports `errors.Is` and
`errors.As` through `Unwrap`.

```go
httpErr, ok := ghttp.FromError(err)
if ok {
    fmt.Println(httpErr.StatusCode)
    fmt.Println(httpErr.Request)
    fmt.Println(httpErr.Err)
}

statusCode, ok := ghttp.StatusCode(err)
```

`IsTimeout(err)` recognizes context deadline errors and wrapped `net.Error`
timeouts.
