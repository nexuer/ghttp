package ghttp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nexuer/ghttp/query"
)

func subContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	left := strings.Index(contentType, "/")
	if left == -1 {
		return ""
	}
	right := strings.Index(contentType, ";")
	if right == -1 {
		right = len(contentType)
	}
	if right < left {
		return ""
	}
	sct := contentType[left+1 : right]
	left = strings.Index(sct, "+")
	if left >= 0 {
		return sct[left+1:]
	}
	return sct
}

func ProxyURL(address string) func(*http.Request) (*url.URL, error) {
	// :7890 or /proxy
	if strings.HasPrefix(address, ":") || strings.HasPrefix(address, "/") {
		address = fmt.Sprintf("http://127.0.0.1%s", address)
	}
	// 127.0.0.1:7890
	if !strings.HasPrefix(address, "https://") && !strings.HasPrefix(address, "http://") {
		address = fmt.Sprintf("http://%s", address)
	}

	proxy, err := url.Parse(address)
	if err != nil {
		return func(request *http.Request) (*url.URL, error) {
			return nil, err
		}
	}

	return http.ProxyURL(proxy)
}

func ForceHttps(endpoint string) string {
	index := strings.Index(endpoint, "://")
	if index >= 0 {
		endpoint = endpoint[index+3:]
	}
	return fmt.Sprintf("https://%s", endpoint)
}

func Not2xxCode(code int) bool {
	return code < 200 || code > 299
}

func joinPath(endpoint, path string) string {
	if endpoint == "" {
		return path
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	var fullPath string
	if strings.HasPrefix(path, endpoint) {
		fullPath = path
	} else {
		fullPath = fmt.Sprintf("%s/%s", strings.TrimRight(endpoint, "/"), strings.TrimLeft(path, "/"))
	}

	if !strings.HasPrefix(fullPath, "http://") && !strings.HasPrefix(fullPath, "https://") {
		return "http://" + fullPath
	}

	return fullPath
}

func SetQuery(req *http.Request, q any) error {
	if q == nil {
		return nil
	}
	values, err := query.Values(q)
	if err != nil {
		return err
	}
	queryStr := values.Encode()
	if queryStr == "" {
		return nil
	}

	if req.URL.RawQuery == "" {
		req.URL.RawQuery = queryStr
	} else {
		req.URL.RawQuery += "&" + queryStr
	}
	return nil
}

func BindResponseBody(response *http.Response, reply any) error {
	if reply == nil {
		return nil
	}

	if response.Body == nil || response.Body == http.NoBody {
		return fmt.Errorf("response: no body")
	}

	codec, _ := CodecForResponse(response)
	if codec == nil {
		return fmt.Errorf("response: unsupported content type: %s",
			response.Header.Get("Content-Type"))
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return codec.Unmarshal(body, reply)
}

func SetRequestBody(req *http.Request, body io.Reader) error {
	if body != nil {
		switch v := body.(type) {
		case *bytes.Buffer:
			req.ContentLength = int64(v.Len())
			buf := v.Bytes()
			req.GetBody = func() (io.ReadCloser, error) {
				r := bytes.NewReader(buf)
				return io.NopCloser(r), nil
			}
		case *bytes.Reader:
			req.ContentLength = int64(v.Len())
			snapshot := *v
			req.GetBody = func() (io.ReadCloser, error) {
				r := snapshot
				return io.NopCloser(&r), nil
			}
		case *strings.Reader:
			req.ContentLength = int64(v.Len())
			snapshot := *v
			req.GetBody = func() (io.ReadCloser, error) {
				r := snapshot
				return io.NopCloser(&r), nil
			}
		default:
			// This is where we'd set it to -1 (at least
			// if body != NoBody) to mean unknown, but
			// that broke people during the Go 1.8 testing
			// period. People depend on it being 0 I
			// guess. Maybe retry later. See Issue 18117.
		}
		// For client requests, Request.ContentLength of 0
		// means either actually 0, or unknown. The only way
		// to explicitly say that the ContentLength is zero is
		// to set the Body to nil. But turns out too much code
		// depends on NewRequest returning a non-nil Body,
		// so we use a well-known ReadCloser variable instead
		// and have the http package also treat that sentinel
		// variable to mean explicitly zero.
		if req.GetBody != nil && req.ContentLength == 0 {
			req.Body = http.NoBody
			req.GetBody = func() (io.ReadCloser, error) { return http.NoBody, nil }
		}
	}
	return nil
}
