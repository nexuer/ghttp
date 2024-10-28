# ghttp
A Go HTTP client designed for quick integration with REST APIs.

## Installation
```shell
go get github.com/nexuer/ghttp
```
## Usage

### Options
> Note: For rate limiting and retry mechanisms, please use external libraries.
> This keeps ghttp focused on making HTTP calls, maintaining simplicity and clarity in its design.

#### Configure the HTTP RoundTripper

`WithTransport(trans http.RoundTripper)`
```go
// Example: Configure proxy and client certificates
ghttp.WithTransport(&http.Transport{
    Proxy: ghttp.ProxyURL(":7890"), // or http.ProxyFromEnvironment
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: true,
    },
}),
```
#### Set Default Timeout

`WithTimeout(d time.Duration)`
```go
// Example: Set a specific timeout
ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
defer cancel()
_, err := client.Invoke(ctx, http.MethodGet, "/api/v4/projects", nil, nil)
```
#### Set Default User-Agent

`WithUserAgent(userAgent string)`

#### Set Default Endpoint

`WithEndpoint(endpoint string)`

#### Set Default Content-Type
`WithContentType(contentType string)`

#### Configure Proxy
> Default: http.ProxyFromEnvironment, can use `ghttp.ProxyURL(url)`

`WithProxy(f func(*http.Request) (*url.URL, error))`

#### Bind Struct for Non-2xx Status Codes
`WithNot2xxError(f func() error)`

#### Enable Debugging
`WithDebug(open bool)`

### Invocation Methods

- `Invoke(ctx context.Context, method, path string, args any, reply any, opts ...CallOption) (*http.Response, error)`
- `Do(req *http.Request, opts ...CallOption) (*http.Response, error)`

`CallOption` is an interface that allows customization through method implementation:

```go
type CallOption interface {
    Before(request *http.Request) error
    After(response *http.Response) error
}
```
#### `ghttp.CallOptions`
Implements the `CallOption` interface, providing functionality for:
```go
// CallOptions default call options
type CallOptions struct {
	// Set query parameters
	Query any

	// Basic auth
	Username string
	Password string

	// Bearer token
	BearerToken string

	// hooks
	BeforeHook func(request *http.Request) error
	AfterHook  func(response *http.Response) error
}
```
### Binding 
#### Request Query
[usage](./query/README.md)

#### Encoding
> Automatically loads the corresponding Codec instance based on content-type. Subtype extraction occurs (e.g., both application/json and application/vnd.api+json are treated as json).

Custom `Codec`
Override default JSON serialization using `sonic`:
```go
package main

import (
    "github.com/bytedance/sonic"
    "github.com/nexuer/ghttp"
)

type codec struct{}

func (codec) Name() string {
    return "sonic-json"
}

func (codec) Marshal(v interface{}) ([]byte, error) {
    return sonic.Marshal(v)
}

func (codec) Unmarshal(data []byte, v interface{}) error {
    return sonic.Unmarshal(data, v)
}

func main() {
    ghttp.RegisterCodec("application/json", codec{})
}
```

### Debugging
Enable debugging with `WithDebug`, output example:
```text
--------------------------------------------
Trace                         Value                          
--------------------------------------------
DNSDuration                   3.955292ms                    
ConnectDuration               102.718541ms                  
TLSHandshakeDuration          98.159333ms                   
RequestDuration               138.834µs                     
WaitResponseDuration          307.559875ms                  
TotalDuration                 412.40375ms                   

* Host gitlab.com:443 was resolved.
* IPv4: 198.18.7.159
*   Trying 198.18.7.159:443...
* Connected to gitlab.com (198.18.7.159) port 443
* SSL connection using TLS 1.3 / TLS_AES_128_GCM_SHA256
* ALPN: server accepted h2
* using HTTP/1.1
> POST /oauth/token HTTP/1.1
> User-Agent: sdk/gitlab-v0.0.1
> Accept: application/json
> Content-Type: application/json
> Beforehook: BeforeHook
> Authorization: Basic Z2l0bGFiOnBhc3N3b3Jk
>

{
    "client_id": "app",
    "grant_type": "password"
}

> HTTP/2.0 401 Unauthorized
... (remaining output truncated for brevity)
```
