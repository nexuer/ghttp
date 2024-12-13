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

// ProxyURL returns a function that sets a proxy URL for the given HTTP request.
//
// This function accepts an address as input and ensures that the address is properly formatted
// as a valid URL. If the address is a relative address (e.g., ":7890" or "/proxy"), it is
// converted to a full HTTP address (e.g., "http://127.0.0.1:7890"). If the address does not
// include a scheme (e.g., "http://"), the function prepends "http://" to the address.
// The returned function can be used to configure HTTP requests to route through the specified proxy.
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

// ForceHttps ensures that the provided endpoint is using the HTTPS protocol.
// It checks if the URL already contains a scheme (like "http://"), removes it,
// and then prepends "https://" to the endpoint to ensure the URL uses HTTPS.
//
// Example usage:
//
//	url := "http://example.com"
//	secureUrl := ForceHttps(url)
//	fmt.Println(secureUrl) // Output: "https://example.com"
func ForceHttps(endpoint string) string {
	index := strings.Index(endpoint, "://")
	if index >= 0 {
		endpoint = endpoint[index+3:]
	}
	return fmt.Sprintf("https://%s", endpoint)
}

func not2xxCode(code int) bool {
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

// SetQuery encodes the provided query parameters into a URL query string and appends them to
// the given HTTP request's URL.
// This function uses the `github.com/nexuer/ghttp/query` package to encode the query parameters.
// The query parameters are serialized into the URL query string format and appended to the
// existing URL of the HTTP request. If the request already contains a query string, the new
// parameters will be appended to it. If no query parameters are provided, no changes are made
// to the request.
//
// Example usage:
//
//	req, err := http.NewRequest("GET", "https://example.com/api", nil)
//	if err != nil {
//	    log.Fatal("Failed to create request:", err)
//	}
//
//	// Define query parameters as a struct
//	queryParams := struct {
//	    Name  string `query:"name"`
//	    Value int    `query:"value"`
//	}{
//	    Name:  "example",
//	    Value: 42,
//	}
//
//	err = SetQuery(req, queryParams)
//	if err != nil {
//	    log.Fatal("Failed to set query parameters:", err)
//	}
//
//	// The request URL will now include the query parameters encoded as `?name=example&value=42`
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

// BindResponseBody binds the body of an HTTP response to the given 'target' struct,
// automatically decoding the body based on the Content-Type header of the response.
//
// This function checks the Content-Type header and decodes the response body into the
// corresponding Go type. It assumes that the 'target' parameter is a pointer to the
// structure that should be populated with the response data.
//
// Example usage:
//
//	var userResponse User
//	err := BindResponseBody(response, &userResponse)
//	if err != nil {
//	    log.Fatal("Failed to bind response body:", err)
//	}
//	// The 'userResponse' struct will now be populated with the decoded response data.
func BindResponseBody(resp *http.Response, target any) error {
	if target == nil {
		return nil
	}

	if resp.Body == nil || resp.Body == http.NoBody {
		return fmt.Errorf("response: no body")
	}

	codec, _ := CodecForResponse(resp)
	if codec == nil {
		return fmt.Errorf("response: unsupported content type: %s",
			resp.Header.Get("Content-Type"))
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return codec.Unmarshal(body, target)
}

// EncodeRequestBody encodes the provided body content based on the Content-Type of the
// given HTTP request, and sets the encoded body in the request.
//
// The function will automatically detect the Content-Type header of the request and
// encode the body accordingly (e.g., JSON, XML, etc.). It then uses SetRequestBody to
// set the encoded body in the request.
//
// Example usage:
//
//	// Example structure to be encoded into the request body
//	type MyRequest struct {
//	    Name  string `json:"name"`
//	    Value int    `json:"value"`
//	}
//
//	req, err := http.NewRequest("POST", "https://example.com/api", nil)
//	if err != nil {
//	    log.Fatal("Failed to create request:", err)
//	}
//
//	body := MyRequest{Name: "example", Value: 42}
//	err = EncodeRequestBody(req, body)
//	if err != nil {
//	    log.Fatal("Failed to encode request body:", err)
//	}
//
//	// Now the request body is set to the JSON-encoded version of MyRequest
//	fmt.Println("Request prepared with body:", req.Body)
func EncodeRequestBody(req *http.Request, body any) error {
	if body == nil || req == nil {
		return nil
	}
	codec, _ := CodecForRequest(req)
	if codec == nil {
		return fmt.Errorf("request: unsupported content type: %s",
			req.Header.Get("Content-Type"))
	}
	bodyBytes, err := codec.Marshal(body)
	if err != nil {
		return err
	}
	return SetRequestBody(req, bytes.NewBuffer(bodyBytes))
}

// SetRequestBody modifies the body of the given HTTP request.
//
// This function allows you to set or replace the body of the HTTP request with
// the provided 'body' parameter. The body is expected to be an io.Reader, which
// means it can be any type that implements the io.Reader interface, such as a
// byte buffer or a file stream.
//
// Example usage:
//
//	req, err := http.NewRequest("POST", "https://example.com", nil)
//	if err != nil {
//	    log.Fatal("Failed to create request:", err)
//	}
//
//	body := strings.NewReader("some request data")
//	err = SetRequestBody(req, body)
//	if err != nil {
//	    log.Fatal("Failed to set request body:", err)
//	}
//	// Now the request body is set to "some request data"
func SetRequestBody(req *http.Request, body io.Reader) error {
	if body == nil || req == nil {
		return nil
	}
	rc, ok := body.(io.ReadCloser)
	if !ok && body != nil {
		rc = io.NopCloser(body)
	}
	req.Body = rc
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
	return nil
}
