package ghttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

type countingReader struct {
	io.Reader
	reads int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.Reader.Read(p)
}

type partialErrorReadCloser struct {
	data        []byte
	err         error
	errReturned bool
	closed      bool
}

func (r *partialErrorReadCloser) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		if len(r.data) == 0 {
			r.errReturned = true
			return n, r.err
		}
		return n, nil
	}
	if !r.errReturned {
		r.errReturned = true
		return 0, r.err
	}
	return 0, io.EOF
}

func (r *partialErrorReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestClientDebuggerAttachesTraceToSentRequest(t *testing.T) {
	var sentRequest *http.Request
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		sentRequest = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    req,
		}, nil
	})

	client := NewClient(
		WithTransport(transport),
		WithDebug(true),
		WithDebugger(func() Debugger {
			return &Debug{Writer: io.Discard, Trace: true}
		}),
	)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if sentRequest == nil {
		t.Fatal("transport did not receive a request")
	}
	if trace := httptrace.ContextClientTrace(sentRequest.Context()); trace == nil {
		t.Fatal("sent request does not contain an HTTP client trace")
	}
}

func TestClientClosesResponseBodyWhenAfterHookFails(t *testing.T) {
	body := &trackingReadCloser{Reader: strings.NewReader("response")}
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       body,
			Request:    req,
		}, nil
	})

	client := NewClient(WithTransport(transport))
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	hookErr := errors.New("after hook failed")

	resp, err := client.Do(req, After(func(*http.Response) error {
		return hookErr
	}))
	if resp != nil {
		t.Fatal("expected a nil response when the after hook fails")
	}
	if !errors.Is(err, hookErr) {
		t.Fatalf("expected after hook error, got %v", err)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestDebugEndWrapsResponseBodyWithoutPreReading(t *testing.T) {
	reader := &countingReader{Reader: strings.NewReader("response")}
	originalBody := &trackingReadCloser{Reader: reader}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       originalBody,
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	debugger := &Debug{Writer: &output}

	debugger.End(req, response, nil)

	if reader.reads != 0 {
		t.Fatalf("Debug.End read the response body %d times", reader.reads)
	}
	if originalBody.closed {
		t.Fatal("Debug.End closed the streaming response body")
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("response")) {
		t.Fatalf("response body = %q, want %q", got, "response")
	}
	if !strings.Contains(output.String(), "response") {
		t.Fatalf("debug output does not contain the response body: %q", output.String())
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !originalBody.closed {
		t.Fatal("closing the wrapped response body did not close the original body")
	}
}

func TestDebugEndUsesNegotiatedProtocolAndStableHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/path?q=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = http.Header{
		"Z-Header": {"last"},
		"A-Header": {"first", "second"},
		"Host":     {"duplicate.example.com"},
	}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Proto:      "HTTP/2.0",
		Header: http.Header{
			"Z-Response": {"last"},
			"A-Response": {"first", "second"},
		},
		Body: http.NoBody,
	}
	var output bytes.Buffer
	debugger := &Debug{Writer: &output}

	debugger.End(req, response, nil)

	want := "" +
		"* using HTTP/2.0\n" +
		"> GET /path?q=1 HTTP/2.0\n" +
		"> Host: example.com\n" +
		"> A-Header: first\n" +
		"> A-Header: second\n" +
		"> Z-Header: last\n" +
		">\n" +
		"\n" +
		"< HTTP/2.0 200 OK\n" +
		"< A-Response: first\n" +
		"< A-Response: second\n" +
		"< Z-Response: last\n" +
		"<\n"
	if output.String() != want {
		t.Fatalf("debug output:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestDebugEndOmitsUnconfirmedProtocol(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/path", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	debugger := &Debug{Writer: &output}

	debugger.End(req, nil, errors.New("connection failed"))

	if strings.Contains(output.String(), "* using") || strings.Contains(output.String(), "HTTP/1.1") {
		t.Fatalf("debug output claims an unconfirmed protocol: %q", output.String())
	}
	if !strings.Contains(output.String(), "> GET /path\n") {
		t.Fatalf("debug output does not contain the request target: %q", output.String())
	}
}

func TestDebugResponseBodyPreservesPartialDataAndReadError(t *testing.T) {
	readErr := errors.New("stream interrupted")
	originalBody := &partialErrorReadCloser{data: []byte("partial"), err: readErr}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       originalBody,
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	debugger := &Debug{Writer: &output}

	debugger.End(req, response, nil)
	got, err := io.ReadAll(response.Body)
	if !errors.Is(err, readErr) {
		t.Fatalf("response body error = %v, want %v", err, readErr)
	}
	if !bytes.Equal(got, []byte("partial")) {
		t.Fatalf("response body = %q, want %q", got, "partial")
	}
	if !strings.Contains(output.String(), "partial") || !strings.Contains(output.String(), readErr.Error()) {
		t.Fatalf("debug output does not contain partial data and its error: %q", output.String())
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !originalBody.closed {
		t.Fatal("original response body was not closed")
	}
}

func TestDebugResponseBodyLimitsCapturedDataWithoutTruncatingCaller(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader("response")),
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	debugger := &Debug{Writer: &output, ResponseBodyLimit: 4}

	debugger.End(req, response, nil)
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if string(got) != "response" {
		t.Fatalf("response body = %q, want %q", got, "response")
	}
	if !strings.Contains(output.String(), "resp") ||
		!strings.Contains(output.String(), "response body truncated after 4 bytes") {
		t.Fatalf("unexpected debug output: %q", output.String())
	}
}

func TestDebugResponseBodyClosesEarlyAndLogsOnce(t *testing.T) {
	originalBody := &trackingReadCloser{Reader: strings.NewReader("response")}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       originalBody,
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	debugger := &Debug{Writer: &output}

	debugger.End(req, response, nil)
	buf := make([]byte, 3)
	if _, err := io.ReadFull(response.Body, buf); err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !originalBody.closed {
		t.Fatal("original response body was not closed")
	}
	if string(buf) != "res" {
		t.Fatalf("response prefix = %q, want %q", buf, "res")
	}
	if strings.Count(output.String(), "response body closed before EOF") != 1 {
		t.Fatalf("early-close message was not logged exactly once: %q", output.String())
	}
}

func TestDebugTraceDurationsIgnoreMissingEvents(t *testing.T) {
	start := time.Unix(100, 0)
	state := &traceInfo{
		startTime:        start,
		responseDoneTime: start.Add(time.Second),
		dnsStartTime:     start,
		dnsDoneTime:      start.Add(5 * time.Millisecond),
	}
	debugger := &Debug{Trace: true}

	info := debugger.statTraceInfo(context.Background(), state)
	if info.DNSDuration != 5*time.Millisecond {
		t.Fatalf("DNSDuration = %s, want %s", info.DNSDuration, 5*time.Millisecond)
	}
	if info.ConnectDuration != 0 || info.RequestDuration != 0 ||
		info.WaitResponseDuration != 0 || info.ResponseDuration != 0 {
		t.Fatalf("missing trace events produced non-zero durations: %+v", info)
	}
	if info.TotalDuration != time.Second {
		t.Fatalf("TotalDuration = %s, want %s", info.TotalDuration, time.Second)
	}
	table := string(info.Table())
	if value := traceTableValue(table, "TCP Connect"); value != "-" {
		t.Fatalf("TCP Connect table value = %q, want %q\n%s", value, "-", table)
	}
	if value := traceTableValue(table, "Connection Reused"); value != "-" {
		t.Fatalf("Connection Reused table value = %q, want %q\n%s", value, "-", table)
	}
}

func TestDebugTraceTableUsesAccurateTimingNames(t *testing.T) {
	start := time.Unix(100, 0)
	state := &traceInfo{
		startTime:                start,
		dnsStartTime:             start,
		dnsDoneTime:              start.Add(time.Millisecond),
		getConnTime:              start,
		tcpConnectStartTime:      start.Add(time.Millisecond),
		tcpConnectDoneTime:       start.Add(3 * time.Millisecond),
		tlsHandshakeStartTime:    start.Add(3 * time.Millisecond),
		tlsHandshakeDoneTime:     start.Add(8 * time.Millisecond),
		gotConnTime:              start.Add(8 * time.Millisecond),
		gotConnInfo:              &httptrace.GotConnInfo{Reused: false},
		wroteRequestTime:         start.Add(9 * time.Millisecond),
		gotFirstResponseByteTime: start.Add(20 * time.Millisecond),
		responseDoneTime:         start.Add(21 * time.Millisecond),
	}
	debugger := &Debug{Trace: true}

	info := debugger.statTraceInfo(context.Background(), state)
	if info.TCPConnectDuration != 2*time.Millisecond {
		t.Fatalf("TCPConnectDuration = %s, want %s", info.TCPConnectDuration, 2*time.Millisecond)
	}
	if info.ConnectDuration != 8*time.Millisecond {
		t.Fatalf("ConnectDuration = %s, want %s", info.ConnectDuration, 8*time.Millisecond)
	}
	table := string(info.Table())
	wants := map[string]string{
		"DNS Lookup":         "1ms",
		"TCP Connect":        "2ms",
		"TLS Handshake":      "5ms",
		"Connection Acquire": "8ms",
		"Connection Reused":  "false",
		"Request Write":      "1ms",
		"TTFB":               "11ms",
		"Time to Headers":    "21ms",
	}
	for label, want := range wants {
		if got := traceTableValue(table, label); got != want {
			t.Fatalf("%s table value = %q, want %q\n%s", label, got, want, table)
		}
	}
}

func TestDebugTraceCapturesTCPConnect(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	debugger := &Debug{Trace: true}
	req = debugger.Begin(req)
	trace := httptrace.ContextClientTrace(req.Context())
	if trace == nil {
		t.Fatal("request does not contain an HTTP client trace")
	}

	trace.ConnectStart("tcp", "192.0.2.1:443")
	trace.ConnectDone("tcp", "192.0.2.1:443", nil)
	state, _ := req.Context().Value(debugStateKey{}).(*traceInfo)
	info := debugger.statTraceInfo(context.Background(), state)
	if !info.tcpConnectDurationSet {
		t.Fatal("successful TCP connection was not recorded")
	}
}

func TestDebugTraceDoesNotReportFailedDNSAsResolved(t *testing.T) {
	state := &traceInfo{
		dnsHost:     "missing.example.com",
		dnsDoneInfo: &httptrace.DNSDoneInfo{Err: errors.New("no such host")},
	}
	var output bytes.Buffer

	state.write(&output, "missing.example.com")

	if strings.Contains(output.String(), "was resolved") {
		t.Fatalf("failed DNS lookup was reported as resolved: %q", output.String())
	}
	if !strings.Contains(output.String(), "Could not resolve host missing.example.com: no such host") {
		t.Fatalf("unexpected DNS failure output: %q", output.String())
	}
}

func traceTableValue(table, label string) string {
	for _, line := range strings.Split(table, "\n") {
		if strings.HasPrefix(line, label) {
			return strings.TrimSpace(strings.TrimPrefix(line, label))
		}
	}
	return ""
}

func TestDebugTraceDoesNotRecordFailedTLSState(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	debugger := &Debug{Trace: true}
	req = debugger.Begin(req)
	trace := httptrace.ContextClientTrace(req.Context())
	if trace == nil {
		t.Fatal("request does not contain an HTTP client trace")
	}

	trace.TLSHandshakeStart()
	trace.TLSHandshakeDone(tls.ConnectionState{}, errors.New("TLS handshake failed"))
	state, _ := req.Context().Value(debugStateKey{}).(*traceInfo)
	if state == nil {
		t.Fatal("request does not contain debug trace state")
	}
	if state.tlsConnectionState != nil {
		t.Fatal("failed TLS handshake recorded an empty connection state")
	}
}

func TestDebugEndClosesRequestBodyCopy(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	var bodyCopy *trackingReadCloser
	req.GetBody = func() (io.ReadCloser, error) {
		bodyCopy = &trackingReadCloser{Reader: strings.NewReader("request")}
		return bodyCopy, nil
	}
	debugger := &Debug{Writer: io.Discard}

	debugger.End(req, nil, nil)

	if bodyCopy == nil || !bodyCopy.closed {
		t.Fatal("request body copy was not closed")
	}
}
