package ghttp

import (
	"fmt"
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
	subContentType := contentType[left+1 : right]
	left = strings.Index(subContentType, "+")
	if left >= 0 {
		return subContentType[left+1:]
	}
	return subContentType
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
