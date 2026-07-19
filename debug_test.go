package ghttp

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"testing"
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

func TestDebugEndClosesAndReplacesResponseBody(t *testing.T) {
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
	debugger := &Debug{Writer: io.Discard}

	debugger.End(req, response, nil)

	if !originalBody.closed {
		t.Fatal("original response body was not closed")
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !bytes.Equal(got, []byte("response")) {
		t.Fatalf("replacement response body = %q, want %q", got, "response")
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
