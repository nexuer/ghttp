package ghttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
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
	not2xxError    func() error
	limiter        Limiter
}

// WithLimiter sets a rate limiter for the client.
// This limiter will be applied to control the number of requests made
// to the server, ensuring that the requests stay within the specified limits.
func WithLimiter(l Limiter) ClientOption {
	return func(c *clientOptions) {
		c.limiter = l
	}
}

// WithNot2xxError handle response status code < 200 and code > 299
func WithNot2xxError(f func() error) ClientOption {
	return func(c *clientOptions) {
		c.not2xxError = f
	}
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

	if options.tlsConf != nil || options.proxy != nil {
		if tr, ok := options.transport.(*http.Transport); ok {
			if options.tlsConf != nil {
				tr.TLSClientConfig = options.tlsConf
			}
			if options.proxy != nil {
				tr.Proxy = options.proxy
			}
		}
	}

	return &Client{
		opts: options,
		hc: &http.Client{
			Transport: options.transport,
		},
		contentSubType: subContentType(options.contentType),
	}
}

func (c *Client) SetEndpoint(endpoint string) {
	c.opts.endpoint = endpoint
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

func (c *Client) Invoke(ctx context.Context, method, path string, args any, reply any, opts ...CallOption) (*http.Response, error) {
	var (
		body   io.Reader
		cancel context.CancelFunc
	)
	// set timeout, Do() is not set repeatedly and does not trigger defer()
	ctx, cancel, _ = c.setTimeout(ctx)
	defer cancel()

	if c.opts.limiter != nil {
		if err := c.opts.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	// marshal request body
	if args != nil {
		codec := defaultContentType.get(c.contentSubType)
		if codec == nil {
			return nil, fmt.Errorf("request: unsupported content type: %s", c.opts.contentType)
		}
		bodyBytes, err := codec.Marshal(args)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	response, err := c.do(req, opts...)
	if err != nil {
		return nil, err
	}

	if err = c.BindResponseBody(response, reply); err != nil {
		return nil, newError(req, response, err)
	}

	return response, nil
}

// Do send an HTTP request and decodes the body of response into target.
func (c *Client) Do(req *http.Request, opts ...CallOption) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("http: nil http request")
	}

	// set timeout
	ctx, cancel, ok := c.setTimeout(req.Context())
	if ok {
		defer cancel()
		req = req.WithContext(ctx)
	}

	if c.opts.limiter != nil {
		if err := c.opts.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}
	return c.do(req, opts...)
}

func (c *Client) do(req *http.Request, opts ...CallOption) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("http: nil http request")
	}

	// First set the default header, the user can overwrite
	c.setHeader(req)

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

	if err = c.bindNot2xxError(response); err != nil {
		return nil, newError(req, response, err)
	}

	return response, nil
}

func (c *Client) bindNot2xxError(response *http.Response) error {
	if !Not2xxCode(response.StatusCode) || c.opts.not2xxError == nil {
		return nil
	}
	// new not2xxError
	not2xxError := c.opts.not2xxError()
	if not2xxError == nil {
		return nil
	}

	if err := c.BindResponseBody(response, not2xxError); err != nil {
		return err
	}

	return not2xxError
}

func (c *Client) BindResponseBody(response *http.Response, reply any) error {
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
