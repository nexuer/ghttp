package ghttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/nexuer/ghttp/encoding"
)

// Debugger observes a single HTTP request lifecycle. Begin may return a derived
// request. Pass the returned request to both the transport and End.
type Debugger interface {
	Begin(request *http.Request) *http.Request
	End(request *http.Request, response *http.Response, err error)
}

// Debug writes curl-style request, response, and optional network trace output.
// It does not pre-read streaming bodies.
type Debug struct {
	Writer        io.Writer
	Trace         bool
	TraceCallback func(w io.Writer, info TraceInfo)

	// RequestBodyLimit is the maximum number of replayable text request body
	// bytes included in debug output. Zero uses the default limit; a negative
	// value disables request body logging.
	RequestBodyLimit int64

	// ResponseBodyLimit is the maximum number of response body bytes kept for
	// debug output. Zero uses the default limit; a negative value disables
	// response body logging.
	ResponseBodyLimit int64
}

const defaultDebugBodyLimit int64 = 64 << 10

func (d *Debug) writer() io.Writer {
	if d.Writer == nil {
		return os.Stderr
	}
	return d.Writer
}

type debugStateKey struct{}

func (d *Debug) Begin(req *http.Request) *http.Request {
	if !d.Trace {
		return req
	}

	state := &traceInfo{
		startTime:     time.Now(),
		connectStarts: make(map[string]time.Time),
	}
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			now := time.Now()
			state.mu.Lock()
			state.dnsStartTime = now
			state.dnsHost = info.Host
			state.mu.Unlock()
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			now := time.Now()
			copied := dnsInfo
			copied.Addrs = append([]net.IPAddr(nil), dnsInfo.Addrs...)
			state.mu.Lock()
			state.dnsDoneTime = now
			state.dnsDoneInfo = &copied
			state.mu.Unlock()
		},
		GetConn: func(hostPort string) {
			now := time.Now()
			state.mu.Lock()
			state.getConnTime = now
			state.getConnHostPort = hostPort
			state.mu.Unlock()
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			now := time.Now()
			copied := connInfo
			state.mu.Lock()
			state.gotConnTime = now
			state.gotConnInfo = &copied
			state.mu.Unlock()
		},
		ConnectStart: func(network, addr string) {
			now := time.Now()
			state.mu.Lock()
			state.connectStarts[connectTraceKey(network, addr)] = now
			state.mu.Unlock()
		},
		ConnectDone: func(network, addr string, err error) {
			now := time.Now()
			key := connectTraceKey(network, addr)
			state.mu.Lock()
			started := state.connectStarts[key]
			delete(state.connectStarts, key)
			if err == nil && state.tcpConnectDoneTime.IsZero() {
				state.tcpConnectStartTime = started
				state.tcpConnectDoneTime = now
				state.tcpConnectAddr = addr
			}
			state.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			now := time.Now()
			state.mu.Lock()
			state.tlsHandshakeStartTime = now
			state.mu.Unlock()
		},
		TLSHandshakeDone: func(connectionState tls.ConnectionState, err error) {
			now := time.Now()
			state.mu.Lock()
			state.tlsHandshakeDoneTime = now
			if err == nil {
				copied := connectionState
				state.tlsConnectionState = &copied
			}
			state.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			now := time.Now()
			state.mu.Lock()
			state.gotFirstResponseByteTime = now
			state.mu.Unlock()
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			now := time.Now()
			state.mu.Lock()
			state.wroteRequestTime = now
			state.mu.Unlock()
		},
	}

	ctx := context.WithValue(req.Context(), debugStateKey{}, state)
	ctx = httptrace.WithClientTrace(ctx, trace)
	return req.WithContext(ctx)
}

func (d *Debug) statTraceInfo(ctx context.Context, state *traceInfo) TraceInfo {
	if !d.Trace || state == nil {
		return TraceInfo{}
	}
	state.mu.Lock()
	defer state.mu.Unlock()

	dnsDuration, dnsDurationSet := traceDuration(state.dnsStartTime, state.dnsDoneTime)
	connectDuration, connectDurationSet := traceDuration(state.getConnTime, state.gotConnTime)
	tcpConnectDuration, tcpConnectDurationSet := traceDuration(
		state.tcpConnectStartTime, state.tcpConnectDoneTime)
	tlsDuration, tlsDurationSet := traceDuration(
		state.tlsHandshakeStartTime, state.tlsHandshakeDoneTime)
	requestDuration, requestDurationSet := traceDuration(state.gotConnTime, state.wroteRequestTime)
	waitDuration, waitDurationSet := traceDuration(
		state.wroteRequestTime, state.gotFirstResponseByteTime)
	responseDuration, responseDurationSet := traceDuration(
		state.gotFirstResponseByteTime, state.responseDoneTime)
	totalDuration, totalDurationSet := traceDuration(state.startTime, state.responseDoneTime)

	connectionReused := false
	connectionReusedSet := state.gotConnInfo != nil
	if connectionReusedSet {
		connectionReused = state.gotConnInfo.Reused
	}

	return TraceInfo{
		ctx:                     ctx,
		DNSDuration:             dnsDuration,
		ConnectDuration:         connectDuration,
		TCPConnectDuration:      tcpConnectDuration,
		TLSHandshakeDuration:    tlsDuration,
		RequestDuration:         requestDuration,
		WaitResponseDuration:    waitDuration,
		ResponseDuration:        responseDuration,
		TotalDuration:           totalDuration,
		ConnectionReused:        connectionReused,
		dnsDurationSet:          dnsDurationSet,
		connectDurationSet:      connectDurationSet,
		tcpConnectDurationSet:   tcpConnectDurationSet,
		tlsHandshakeDurationSet: tlsDurationSet,
		requestDurationSet:      requestDurationSet,
		waitResponseDurationSet: waitDurationSet,
		responseDurationSet:     responseDurationSet,
		totalDurationSet:        totalDurationSet,
		connectionReusedSet:     connectionReusedSet,
	}
}

func connectTraceKey(network, addr string) string {
	return network + "\x00" + addr
}

func traceDuration(start, end time.Time) (time.Duration, bool) {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0, false
	}
	return end.Sub(start), true
}

func (d *Debug) End(request *http.Request, response *http.Response, err error) {
	writer := d.writer()

	path := request.URL.RequestURI()
	if path == "" {
		path = "/"
	}

	if d.Trace {
		state, _ := request.Context().Value(debugStateKey{}).(*traceInfo)
		if state != nil {
			now := time.Now()
			state.mu.Lock()
			state.responseDoneTime = now
			state.mu.Unlock()
		}
		if d.TraceCallback != nil {
			d.TraceCallback(writer, d.statTraceInfo(request.Context(), state))
		}
		if state != nil {
			state.write(writer, request.URL.Hostname())
		}
	}

	if response != nil && response.Proto != "" {
		write(writer, "* using %s", response.Proto)
		write(writer, "> %s %s %s", request.Method, path, response.Proto)
	} else {
		write(writer, "> %s %s", request.Method, path)
	}
	host := request.Host
	if host == "" {
		host = request.URL.Host
	}
	if host != "" {
		write(writer, "> Host: %s", host)
	}
	writeHeaders(writer, ">", request.Header, true)
	write(writer, ">")

	d.writeRequestBody(writer, request)

	if response != nil {
		write(writer, "")
		if response.Proto != "" {
			write(writer, "< %s %s", response.Proto, response.Status)
		} else {
			write(writer, "< %s", response.Status)
		}
		writeHeaders(writer, "<", response.Header, false)
		write(writer, "<")
		// response body
		if response.Body != nil && response.Body != http.NoBody {
			limit := d.responseBodyLimit()
			if limit >= 0 {
				response.Body = &debugResponseBody{
					body:     response.Body,
					limit:    limit,
					expected: response.ContentLength,
					onDone: func(body []byte, truncated, closedEarly bool, readErr error) {
						d.writeResponseBody(writer, response, body, truncated, closedEarly, readErr)
					},
				}
			}
		}
	}

	if err != nil {
		write(writer, "")
		write(writer, "** ERROR: %s", err)
	}
}

func writeHeaders(writer io.Writer, prefix string, header http.Header, skipHost bool) {
	keys := make([]string, 0, len(header))
	for key := range header {
		if skipHost && strings.EqualFold(key, "Host") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := strings.ToLower(keys[i])
		right := strings.ToLower(keys[j])
		if left == right {
			return keys[i] < keys[j]
		}
		return left < right
	})
	for _, key := range keys {
		values := header[key]
		if len(values) == 0 {
			write(writer, "%s %s:", prefix, key)
			continue
		}
		for _, value := range values {
			write(writer, "%s %s: %s", prefix, key, value)
		}
	}
}

func (d *Debug) requestBodyLimit() int64 {
	if d.RequestBodyLimit == 0 {
		return defaultDebugBodyLimit
	}
	return d.RequestBodyLimit
}

func (d *Debug) writeRequestBody(writer io.Writer, request *http.Request) {
	if request.Body == nil || request.Body == http.NoBody {
		return
	}
	limit := d.requestBodyLimit()
	if limit < 0 {
		return
	}

	previewable, description := requestBodyPreviewable(request)
	if !previewable {
		write(writer, "* request body omitted (%s)", requestBodyDescription(description, request.ContentLength))
		return
	}
	if request.GetBody == nil {
		write(writer, "* request body is streaming; not captured (%s)",
			requestBodyDescription(description, request.ContentLength))
		return
	}

	bodyReader, err := request.GetBody()
	if err != nil {
		write(writer, "** REQUEST BODY COPY ERROR: %s", err)
		return
	}
	if bodyReader == nil {
		write(writer, "** REQUEST BODY COPY ERROR: GetBody returned a nil reader")
		return
	}

	body, truncated, readErr := readDebugBody(bodyReader, limit)
	closeErr := bodyReader.Close()
	if len(body) > 0 {
		codec, _ := CodecForRequest(request)
		formatted, _ := formatIndent(codec, body)
		if len(formatted) == 0 {
			formatted = body
		}
		write(writer, "* request body preview:")
		write(writer, "%s", string(formatted))
	}
	if truncated {
		write(writer, "* request body truncated after %d bytes", len(body))
	}
	if readErr != nil {
		write(writer, "** REQUEST BODY ERROR: %s", readErr)
	}
	if closeErr != nil {
		write(writer, "** REQUEST BODY CLOSE ERROR: %s", closeErr)
	}
}

func requestBodyPreviewable(request *http.Request) (bool, string) {
	contentEncoding := strings.Join(request.Header.Values("Content-Encoding"), ",")
	for _, encoding := range strings.Split(contentEncoding, ",") {
		encoding = strings.TrimSpace(encoding)
		if encoding != "" && !strings.EqualFold(encoding, "identity") {
			return false, "Content-Encoding: " + contentEncoding
		}
	}

	contentType := strings.TrimSpace(request.Header.Get("Content-Type"))
	if contentType == "" {
		return false, "unknown content type"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false, "invalid Content-Type: " + contentType
	}
	mediaType = strings.ToLower(mediaType)
	if strings.HasPrefix(mediaType, "text/") ||
		strings.HasSuffix(mediaType, "+json") ||
		strings.HasSuffix(mediaType, "+xml") ||
		strings.HasSuffix(mediaType, "+yaml") {
		return true, mediaType
	}
	switch mediaType {
	case "application/json", "application/xml", "application/yaml", "application/x-yaml",
		"application/x-www-form-urlencoded", "application/graphql", "application/javascript",
		"application/ndjson", "application/x-ndjson", "application/json-seq":
		return true, mediaType
	default:
		return false, mediaType
	}
}

func requestBodyDescription(description string, contentLength int64) string {
	if contentLength > 0 {
		return fmt.Sprintf("%s, %d bytes", description, contentLength)
	}
	return description
}

func readDebugBody(reader io.Reader, limit int64) ([]byte, bool, error) {
	maxRead := limit
	if limit < int64(^uint64(0)>>1) {
		maxRead++
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxRead))
	truncated := int64(len(body)) > limit
	if truncated {
		body = body[:limit]
	}
	return body, truncated, err
}

func (d *Debug) responseBodyLimit() int64 {
	if d.ResponseBodyLimit == 0 {
		return defaultDebugBodyLimit
	}
	return d.ResponseBodyLimit
}

func (d *Debug) writeResponseBody(writer io.Writer, response *http.Response, body []byte,
	truncated, closedEarly bool, readErr error) {
	if len(body) > 0 {
		codec, _ := CodecForResponse(response)
		formatted, _ := formatIndent(codec, body)
		if len(formatted) == 0 {
			formatted = body
		}
		write(writer, "%s", string(formatted))
	}
	if truncated {
		write(writer, "* response body truncated after %d bytes", len(body))
	}
	if closedEarly {
		write(writer, "* response body closed before EOF")
	}
	if readErr != nil && readErr != io.EOF {
		write(writer, "** RESPONSE BODY ERROR: %s", readErr)
	}
}

type debugResponseBody struct {
	body     io.ReadCloser
	limit    int64
	expected int64
	buf      bytes.Buffer
	onDone   func(body []byte, truncated, closedEarly bool, readErr error)

	truncated bool
	done      bool
	read      int64
	mu        sync.Mutex
	once      sync.Once
}

func (b *debugResponseBody) Read(p []byte) (n int, err error) {
	n, err = b.body.Read(p)
	if n > 0 {
		b.capture(p[:n])
	}
	if err != nil {
		b.mu.Lock()
		b.done = true
		b.mu.Unlock()
		b.finish(false, err)
	}
	return n, err
}

func (b *debugResponseBody) Close() error {
	err := b.body.Close()
	b.mu.Lock()
	closedEarly := !b.done
	b.mu.Unlock()
	b.finish(closedEarly, nil)
	return err
}

func (b *debugResponseBody) capture(p []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.read += int64(len(p))
	if b.expected > 0 && b.read >= b.expected {
		b.done = true
	}

	remaining := b.limit - int64(b.buf.Len())
	if remaining <= 0 {
		b.truncated = true
		return
	}
	if int64(len(p)) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return
	}
	_, _ = b.buf.Write(p)
}

func (b *debugResponseBody) finish(closedEarly bool, readErr error) {
	b.once.Do(func() {
		if b.onDone == nil {
			return
		}
		b.mu.Lock()
		body := append([]byte(nil), b.buf.Bytes()...)
		truncated := b.truncated
		b.mu.Unlock()
		b.onDone(body, truncated, closedEarly, readErr)
	})
}

type TraceInfo struct {
	ctx context.Context

	DNSDuration          time.Duration `json:"DNSDuration,omitempty" yaml:"DNSDuration" xml:"DNSDuration"`
	ConnectDuration      time.Duration `json:"connectDuration,omitempty" yaml:"connectDuration" xml:"connectDuration"`
	TCPConnectDuration   time.Duration `json:"tcpConnectDuration,omitempty" yaml:"tcpConnectDuration" xml:"tcpConnectDuration"`
	TLSHandshakeDuration time.Duration `json:"TLSHandshakeDuration,omitempty" yaml:"TLSHandshakeDuration" xml:"TLSHandshakeDuration"`
	RequestDuration      time.Duration `json:"requestDuration,omitempty" yaml:"requestDuration" xml:"requestDuration"`
	WaitResponseDuration time.Duration `json:"waitResponseDuration,omitempty" yaml:"waitResponseDuration" xml:"waitResponseDuration"`
	ResponseDuration     time.Duration `json:"responseDuration,omitempty" yaml:"responseDuration" xml:"responseDuration"`
	TotalDuration        time.Duration `json:"totalDuration,omitempty" yaml:"totalDuration" xml:"totalDuration"`
	ConnectionReused     bool          `json:"connectionReused,omitempty" yaml:"connectionReused" xml:"connectionReused"`

	dnsDurationSet          bool
	connectDurationSet      bool
	tcpConnectDurationSet   bool
	tlsHandshakeDurationSet bool
	requestDurationSet      bool
	waitResponseDurationSet bool
	responseDurationSet     bool
	totalDurationSet        bool
	connectionReusedSet     bool
}

func (t TraceInfo) Context() context.Context {
	return t.ctx
}

func (t TraceInfo) String() string {
	return string(t.Table())
}

func (t TraceInfo) Table() []byte {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 24, 0, 3, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(w, "--------------------------------------------")
	_, _ = fmt.Fprintln(w, "Timing\tValue\t")
	_, _ = fmt.Fprintln(w, "--------------------------------------------")
	_, _ = fmt.Fprintf(w, "DNS Lookup\t%s\t\n", traceDurationString(t.DNSDuration, t.dnsDurationSet))
	_, _ = fmt.Fprintf(w, "TCP Connect\t%s\t\n",
		traceDurationString(t.TCPConnectDuration, t.tcpConnectDurationSet))
	_, _ = fmt.Fprintf(w, "TLS Handshake\t%s\t\n",
		traceDurationString(t.TLSHandshakeDuration, t.tlsHandshakeDurationSet))
	_, _ = fmt.Fprintf(w, "Connection Acquire\t%s\t\n",
		traceDurationString(t.ConnectDuration, t.connectDurationSet))
	_, _ = fmt.Fprintf(w, "Connection Reused\t%s\t\n",
		traceBoolString(t.ConnectionReused, t.connectionReusedSet))
	_, _ = fmt.Fprintf(w, "Request Write\t%s\t\n",
		traceDurationString(t.RequestDuration, t.requestDurationSet))
	_, _ = fmt.Fprintf(w, "TTFB\t%s\t\n",
		traceDurationString(t.WaitResponseDuration, t.waitResponseDurationSet))
	_, _ = fmt.Fprintf(w, "Time to Headers\t%s\t\n",
		traceDurationString(t.TotalDuration, t.totalDurationSet))
	_, _ = fmt.Fprintln(w, "--------------------------------------------")

	_ = w.Flush()
	return buf.Bytes()
}

func traceDurationString(duration time.Duration, set bool) string {
	if !set && duration == 0 {
		return "-"
	}
	return duration.String()
}

func traceBoolString(value, set bool) string {
	if !set && !value {
		return "-"
	}
	return fmt.Sprintf("%t", value)
}

type traceInfo struct {
	mu                 sync.Mutex
	dnsHost            string
	dnsDoneInfo        *httptrace.DNSDoneInfo
	getConnHostPort    string
	gotConnInfo        *httptrace.GotConnInfo
	tlsConnectionState *tls.ConnectionState
	connectStarts      map[string]time.Time
	tcpConnectAddr     string

	dnsStartTime             time.Time
	dnsDoneTime              time.Time
	getConnTime              time.Time
	gotConnTime              time.Time
	tcpConnectStartTime      time.Time
	tcpConnectDoneTime       time.Time
	tlsHandshakeStartTime    time.Time
	tlsHandshakeDoneTime     time.Time
	gotFirstResponseByteTime time.Time
	wroteRequestTime         time.Time

	startTime        time.Time
	responseDoneTime time.Time
}

type traceOutput struct {
	dnsHost            string
	dnsDoneInfo        *httptrace.DNSDoneInfo
	getConnHostPort    string
	gotConnInfo        *httptrace.GotConnInfo
	tlsConnectionState *tls.ConnectionState
	tcpConnectAddr     string
}

func (t *traceInfo) output() traceOutput {
	t.mu.Lock()
	defer t.mu.Unlock()
	return traceOutput{
		dnsHost:            t.dnsHost,
		dnsDoneInfo:        t.dnsDoneInfo,
		getConnHostPort:    t.getConnHostPort,
		gotConnInfo:        t.gotConnInfo,
		tlsConnectionState: t.tlsConnectionState,
		tcpConnectAddr:     t.tcpConnectAddr,
	}
}

func (t *traceInfo) write(w io.Writer, requestHost string) {
	output := t.output()
	if output.dnsDoneInfo != nil {
		host := resolvedHost(output.dnsHost, output.getConnHostPort)
		if output.dnsDoneInfo.Err != nil {
			write(w, "* Could not resolve host %s: %s", host, output.dnsDoneInfo.Err)
		} else {
			write(w, "* Host %s was resolved.", host)
			for _, ipAddr := range output.dnsDoneInfo.Addrs {
				if len(ipAddr.IP) == net.IPv4len {
					write(w, "* IPv4: %s", ipAddr.IP)
				}
				if len(ipAddr.IP) == net.IPv6len {
					write(w, "* IPv6: %s", ipAddr.IP)
				}
			}
		}
	}

	connectionHost := connectedHost(output.getConnHostPort, requestHost)
	if output.gotConnInfo != nil && output.gotConnInfo.Reused {
		write(w, "* Reusing existing connection to %s", connectionHost)
	} else if output.gotConnInfo != nil && output.gotConnInfo.Conn != nil {
		remoteAddr := output.gotConnInfo.Conn.RemoteAddr().String()
		tryingAddr := output.tcpConnectAddr
		if tryingAddr == "" {
			tryingAddr = remoteAddr
		}
		write(w, "*   Trying %s...", tryingAddr)
		if ip, port, err := net.SplitHostPort(remoteAddr); err == nil {
			write(w, "* Connected to %s (%s) port %s", connectionHost, ip, port)
		} else {
			write(w, "* Connected to %s (%s)", connectionHost, remoteAddr)
		}
	}

	if output.tlsConnectionState != nil {
		write(w, "* TLS connection using %s / %s",
			tls.VersionName(output.tlsConnectionState.Version),
			tls.CipherSuiteName(output.tlsConnectionState.CipherSuite),
		)
		if output.tlsConnectionState.NegotiatedProtocol != "" {
			write(w, "* ALPN: server accepted %s", output.tlsConnectionState.NegotiatedProtocol)
		}
		if len(output.tlsConnectionState.VerifiedChains) > 0 &&
			len(output.tlsConnectionState.VerifiedChains[0]) > 0 {
			cer := output.tlsConnectionState.VerifiedChains[0][0]
			write(w, `* Server certificate:
*   subject: %s
*   start date: %s
*   expire date: %s
*   issuer: %s
*   SSL certificate verify ok.`, formatPKIXName(cer.Subject), cer.NotBefore.UTC().Format(time.RFC3339),
				cer.NotAfter.UTC().Format(time.RFC3339), formatPKIXName(cer.Issuer))
		}
	}
}

func resolvedHost(dnsHost, hostPort string) string {
	if dnsHost == "" {
		return hostPort
	}
	host, _, err := net.SplitHostPort(hostPort)
	if err == nil && strings.EqualFold(host, dnsHost) {
		return hostPort
	}
	return dnsHost
}

func connectedHost(hostPort, fallback string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err == nil && host != "" {
		return host
	}
	if hostPort != "" {
		return hostPort
	}
	return fallback
}

func formatPKIXName(name pkix.Name) string {
	parts := make([]string, 0, 6)
	appendNamePart := func(key string, values []string) {
		if len(values) > 0 && values[0] != "" {
			parts = append(parts, key+"="+values[0])
		}
	}
	appendNamePart("C", name.Country)
	appendNamePart("ST", name.Province)
	appendNamePart("L", name.Locality)
	appendNamePart("O", name.Organization)
	appendNamePart("OU", name.OrganizationalUnit)
	if name.CommonName != "" {
		parts = append(parts, "CN="+name.CommonName)
	}
	return strings.Join(parts, "; ")
}

func write(w io.Writer, format string, args ...any) {
	if format != "" {
		_, _ = fmt.Fprintf(w, format, args...)
	}
	_, _ = fmt.Fprintf(w, "\n")
}

func formatIndent(codec encoding.Codec, data []byte) (result []byte, err error) {
	if len(data) == 0 || codec == nil {
		return result, nil
	}

	var anyData any
	if err = codec.Unmarshal(data, &anyData); err != nil {
		return data, err
	}

	switch codec.Name() {
	case "json":
		result, err = json.MarshalIndent(anyData, "", "    ")
	default:
		result, err = codec.Marshal(anyData)
	}

	return
}
