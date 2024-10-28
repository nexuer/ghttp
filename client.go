package ghttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/nexuer/ghttp/encoding/json"

	"github.com/nexuer/ghttp/encoding"
)

// ClientOption is HTTP client option.
type ClientOption func(*clientOptions)

// Client is an HTTP transport client.
type clientOptions struct {
	transport      http.RoundTripper
	tlsConf        *tls.Config
	timeout        time.Duration
	endpoint       string
	userAgent      string
	contentType    string
	proxy          func(*http.Request) (*url.URL, error)
	debugInterface func() DebugInterface
	debug          bool
}

// WithDebugInterface sets the function to create a new DebugInterface instance.
func WithDebugInterface(f func() DebugInterface) ClientOption {
	return func(c *clientOptions) {
		c.debugInterface = f
	}
}

// WithDebug open debug.
func WithDebug(open bool) ClientOption {
	return func(c *clientOptions) {
		c.debug = open
	}
}

// WithTransport with http.RoundTrippe.
func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *clientOptions) {
		c.transport = transport
	}
}

// WithTLSConfig with tls config.
func WithTLSConfig(cfg *tls.Config) ClientOption {
	return func(c *clientOptions) {
		c.tlsConf = cfg
	}
}

// WithTimeout with client request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *clientOptions) {
		c.timeout = timeout
	}
}

// WithUserAgent with client user agent.
func WithUserAgent(userAgent string) ClientOption {
	return func(c *clientOptions) {
		c.userAgent = userAgent
	}
}

// WithEndpoint with client addr.
func WithEndpoint(endpoint string) ClientOption {
	return func(c *clientOptions) {
		c.endpoint = endpoint
	}
}

// WithContentType with client request content type.
func WithContentType(contentType string) ClientOption {
	return func(c *clientOptions) {
		c.contentType = contentType
	}
}

// WithProxy with proxy url.
func WithProxy(f func(*http.Request) (*url.URL, error)) ClientOption {
	return func(c *clientOptions) {
		c.proxy = f
	}
}

// Client is an HTTP client.
type Client struct {
	opts           clientOptions
	hc             *http.Client
	contentSubType string
}

func NewClient(opts ...ClientOption) *Client {
	options := clientOptions{
		contentType: "application/json",
		timeout:     5 * time.Second,
		transport:   http.DefaultTransport,
	}

	for _, o := range opts {
		o(&options)
	}

	return &Client{
		opts: options,
		hc: &http.Client{
			Transport: options.transport,
		},
		contentSubType: subContentType(options.contentType),
	}
}

func (c *Client) setTimeout(ctx context.Context) (context.Context, context.CancelFunc, bool) {
	if c.opts.timeout > 0 {
		// the timeout period of this request will not be overwritten
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.opts.timeout)
			return ctx, cancel, true
		}
	}
	return ctx, func() {}, false
}

func (c *Client) setHeader(req *http.Request) {
	if c.opts.userAgent != "" && req.UserAgent() == "" {
		req.Header.Set("User-Agent", c.opts.userAgent)
	}

	if c.opts.contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Accept", c.opts.contentType)
		req.Header.Set("Content-Type", c.opts.contentType)
	}
}

func (c *Client) debugger() DebugInterface {
	if !c.opts.debug {
		return nil
	}
	if c.opts.debugInterface != nil {
		return c.opts.debugInterface()
	}
	return &Debug{
		Trace:  true,
		Writer: os.Stderr,
		TraceCallback: func(w io.Writer, info TraceInfo) {
			_, _ = w.Write(info.Table())
		},
	}
}

// Do send an HTTP request and decodes the body of response into target.
func (c *Client) Do(req *http.Request, opts ...CallOption) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("http: nil http request")
	}

	// First set the default header, the user can overwrite
	c.setHeader(req)

	// set timeout
	ctx, cancel, ok := c.setTimeout(req.Context())
	if ok {
		defer cancel()
		req = req.WithContext(ctx)
	}

	// set default endpoint
	if c.opts.endpoint != "" {
		fullPath := joinPath(c.opts.endpoint, req.URL.String())
		newUrl, err := url.Parse(fullPath)
		if err != nil {
			return nil, newError(req, nil, err)
		}
		req.URL = newUrl
	}

	var err error
	// apply CallOption before
	for _, callOpt := range opts {
		if err = callOpt.Before(req); err != nil {
			return nil, newError(req, nil, err)
		}
	}

	debugger := c.debugger()

	if debugger != nil {
		debugger.Before(req)
	}

	response, err := c.hc.Do(req)
	if debugger != nil {
		debugger.After(req, response, err)
	}

	if err != nil {
		return nil, err
	}

	// apply CallOption After
	for _, callOpt := range opts {
		if err = callOpt.After(response); err != nil {
			return nil, newError(req, response, err)
		}
	}

	return response, nil
}

// CodecForRequest get encoding.Codec via http.Request
func CodecForRequest(r *http.Request, name ...string) (encoding.Codec, bool) {
	headerName := "Content-Type"
	if len(name) > 0 && name[0] != "" {
		headerName = name[0]
	}
	for _, accept := range r.Header[headerName] {
		codec := GetCodecByContentType(accept)
		if codec != nil {
			return codec, true
		}
	}
	return encoding.GetCodec(json.Name), false
}
