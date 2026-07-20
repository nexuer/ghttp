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
	"sync"
	"time"
)

// ClientOption is HTTP client option.
type ClientOption func(*clientOptions)

// Client is an HTTP transport client.
type clientOptions struct {
	transport   http.RoundTripper
	tlsConf     *tls.Config
	timeout     time.Duration
	endpoint    string
	userAgent   string
	contentType string
	proxy       func(*http.Request) (*url.URL, error)
	debugger    func() Debugger
	debug       bool
	not2xxError func() error
	limiter     Limiter
}

// WithLimiter sets the blocking outbound request limiter. It runs after local
// request preparation and immediately before the request is sent.
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

// WithDebugger sets the function used to create a Debugger for each request.
// It takes effect only when debugging is enabled with WithDebug(true).
func WithDebugger(f func() Debugger) ClientOption {
	return func(c *clientOptions) {
		c.debugger = f
	}
}

// WithDebug enables or disables request debugging.
func WithDebug(open bool) ClientOption {
	return func(c *clientOptions) {
		c.debug = open
	}
}

// WithTransport sets the HTTP round tripper. A custom RoundTripper owns all
// transport-level configuration. WithTLSConfig and WithProxy are not applied
// unless transport is *http.Transport.
func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *clientOptions) {
		c.transport = transport
	}
}

// WithTLSConfig configures TLS on the default transport or a transport passed
// directly as *http.Transport. It is not applied to other RoundTrippers.
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

// WithContentType sets the default Content-Type and Accept headers.
func WithContentType(contentType string) ClientOption {
	return func(c *clientOptions) {
		if contentType != "" {
			c.contentType = contentType
		}
	}
}

// WithProxy configures the proxy on the default transport or a transport
// passed directly as *http.Transport. It is not applied to other RoundTrippers.
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
		transport:   http.DefaultTransport,
	}

	for _, o := range opts {
		o(&options)
	}

	if options.tlsConf != nil || options.proxy != nil {
		if tr, ok := options.transport.(*http.Transport); ok {
			// Clone before applying client-specific settings to avoid mutating a shared transport.
			tr = tr.Clone()

			if options.tlsConf != nil {
				tr.TLSClientConfig = options.tlsConf
			}
			if options.proxy != nil {
				tr.Proxy = options.proxy
			}

			options.transport = tr
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

func (c *Client) setHeader(req *http.Request, options *callOptions) {
	if req.Header == nil {
		req.Header = make(http.Header)
	}

	if c.opts.userAgent != "" && req.UserAgent() == "" {
		req.Header.Set("User-Agent", c.opts.userAgent)
	}

	if options.contentType != "" {
		req.Header.Set("Content-Type", options.contentType)
		req.Header.Set("Accept", options.contentType)
		return
	}

	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		contentType = c.opts.contentType
		req.Header.Set("Content-Type", contentType)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", contentType)
	}
}

func (c *Client) newDebugger() Debugger {
	if !c.opts.debug {
		return nil
	}
	if c.opts.debugger != nil {
		return c.opts.debugger()
	}
	return &Debug{
		Trace:  true,
		Writer: os.Stderr,
		TraceCallback: func(w io.Writer, info TraceInfo) {
			_, _ = w.Write(info.Table())
		},
	}
}

func (c *Client) Invoke(ctx context.Context, method, path string, args any, reply any,
	opts ...CallOption) (resp *http.Response, err error) {
	ctx, cancel, _ := c.setTimeout(ctx)
	defer cancel()
	options := resolveCallOptions(opts...)

	// marshal request body
	body, err := c.body(args, options.contentType)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	response, err := c.do(req, options)
	if err != nil {
		return nil, err
	}

	if err = BindResponseBody(response, reply); err != nil {
		return nil, newError(req, response, err)
	}

	return response, nil
}

// Do sends an HTTP request.
func (c *Client) Do(req *http.Request, opts ...CallOption) (resp *http.Response, err error) {
	if req == nil {
		return nil, errors.New("http: nil http request")
	}

	// set timeout
	ctx, cancel, managed := c.setTimeout(req.Context())
	if managed {
		req = req.WithContext(ctx)
	}

	defer func() {
		if err != nil && managed {
			cancel()
		}
	}()

	response, err := c.do(req, resolveCallOptions(opts...))
	if err != nil {
		return nil, err
	}
	if !managed {
		return response, nil
	}
	if response.Body == http.NoBody {
		cancel()
		return response, nil
	}
	response.Body = &cancelResponseBody{
		ReadCloser: response.Body,
		cancel:     cancel,
	}
	return response, nil
}

func (c *Client) do(req *http.Request, options *callOptions) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("http: nil http request")
	}

	// First set the default and structured headers; Before hooks may overwrite them.
	c.setHeader(req, options)

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
	if options.query != nil {
		if err = SetQuery(req, options.query); err != nil {
			return nil, newError(req, nil, err)
		}
	}
	if options.applyAuth != nil {
		options.applyAuth(req)
	}
	for _, hook := range options.beforeHooks {
		if err = hook(req); err != nil {
			return nil, newError(req, nil, err)
		}
	}

	if c.opts.limiter != nil {
		if err = c.opts.limiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}

	debugger := c.newDebugger()

	if debugger != nil {
		if debugReq := debugger.Begin(req); debugReq != nil {
			req = debugReq
		}
	}

	response, err := c.hc.Do(req)
	if debugger != nil {
		debugger.End(req, response, err)
	}

	if err != nil {
		return nil, err
	}

	for _, hook := range options.afterHooks {
		if err = hook(response); err != nil {
			closeResponseBody(response)
			return nil, newError(req, response, err)
		}
	}

	if err = c.bindNot2xxError(response); err != nil {
		return nil, newError(req, response, err)
	}

	return response, nil
}

func closeResponseBody(response *http.Response) {
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
}

type cancelResponseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (b *cancelResponseBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.once.Do(b.cancel)
	}
	return n, err
}

func (b *cancelResponseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.cancel)
	return err
}

func (c *Client) bindNot2xxError(response *http.Response) error {
	if !not2xxCode(response.StatusCode) || c.opts.not2xxError == nil {
		return nil
	}
	// new not2xxError
	not2xxError := c.opts.not2xxError()
	if not2xxError == nil {
		return nil
	}

	if err := BindResponseBody(response, not2xxError); err != nil {
		return err
	}

	return not2xxError
}

func (c *Client) body(body any, contentType ...string) (io.Reader, error) {
	ct := c.opts.contentType
	cst := c.contentSubType
	if len(contentType) > 0 && len(contentType[0]) > 0 {
		ct = contentType[0]
		cst = subContentType(contentType[0])
	}

	if body == nil {
		return nil, nil
	}

	codec := codecForSubtype(cst)
	if codec == nil {
		return nil, fmt.Errorf("request: unsupported content type: %s", ct)
	}
	bodyBytes, err := codec.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(bodyBytes), err
}
