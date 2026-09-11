package ghttp

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadBody(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		maxBytes  []int64
		want      string
		wantLarge bool
	}{
		{name: "default unlimited", body: "response", want: "response"},
		{name: "zero unlimited", body: "response", maxBytes: []int64{0}, want: "response"},
		{name: "negative unlimited", body: "response", maxBytes: []int64{-1}, want: "response"},
		{name: "below limit", body: "response", maxBytes: []int64{9}, want: "response"},
		{name: "exact limit", body: "response", maxBytes: []int64{8}, want: "response"},
		{name: "over limit", body: "response", maxBytes: []int64{7}, wantLarge: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadBody(strings.NewReader(tt.body), tt.maxBytes...)
			if got := IsBodyTooLarge(err); got != tt.wantLarge {
				t.Fatalf("IsBodyTooLarge() = %t; want %t (error: %v)",
					got, tt.wantLarge, err)
			}
			if got := string(got); got != tt.want {
				t.Fatalf("ReadBody() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestReadBodyNilReader(t *testing.T) {
	body, err := ReadBody(nil)
	if err == nil || err.Error() != "nil body" {
		t.Fatalf("ReadBody(nil) = (%q, %v); want (nil, nil body)", body, err)
	}
}

func TestInvokeResponseBodyLimit(t *testing.T) {
	body := &trackingReadCloser{Reader: strings.NewReader(`{"message":"response"}`)}
	client := NewClient(
		WithMaxResponseBodyBytes(8),
		WithTransport(clientTestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       body,
				Request:    req,
			}, nil
		})),
	)

	var reply struct {
		Message string `json:"message"`
	}
	response, err := client.Invoke(context.Background(), http.MethodGet,
		"http://example.test", nil, &reply)
	if response != nil {
		t.Fatal("Invoke() response is non-nil; want nil")
	}
	if !IsBodyTooLarge(err) {
		t.Fatalf("Invoke() error = %v; want response body too large", err)
	}
	httpErr, ok := FromError(err)
	if !ok || httpErr.StatusCode != http.StatusOK {
		t.Fatalf("FromError() = (%v, %t); want status 200", httpErr, ok)
	}
	if !body.closed {
		t.Fatal("Invoke() did not close the limited response body")
	}
}

func TestInvokeResponseBodyDefaultUnlimited(t *testing.T) {
	client := NewClient(
		WithTransport(clientTestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"message":"response"}`)),
				Request:    req,
			}, nil
		})),
	)

	var reply struct {
		Message string `json:"message"`
	}
	if _, err := client.Invoke(context.Background(), http.MethodGet,
		"http://example.test", nil, &reply); err != nil {
		t.Fatal(err)
	}
	if got, want := reply.Message, "response"; got != want {
		t.Fatalf("Invoke() reply message = %q; want %q", got, want)
	}
}

type responseBodyLimitError struct {
	Message string `json:"message"`
}

func (e *responseBodyLimitError) Error() string {
	return e.Message
}

func TestNot2xxResponseBodyLimit(t *testing.T) {
	body := &trackingReadCloser{Reader: strings.NewReader(`{"message":"response"}`)}
	client := NewClient(
		WithMaxResponseBodyBytes(8),
		WithNot2xxError(func() error { return &responseBodyLimitError{} }),
		WithTransport(clientTestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       body,
				Request:    req,
			}, nil
		})),
	)
	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Do(req)
	if response != nil {
		t.Fatal("Do() response is non-nil; want nil")
	}
	if !IsBodyTooLarge(err) {
		t.Fatalf("Do() error = %v; want response body too large", err)
	}
	httpErr, ok := FromError(err)
	if !ok || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("FromError() = (%v, %t); want status 400", httpErr, ok)
	}
	if !body.closed {
		t.Fatal("Do() did not close the limited response body")
	}
}

func TestDoSuccessfulResponseBodyIsNotLimited(t *testing.T) {
	client := NewClient(
		WithMaxResponseBodyBytes(1),
		WithTransport(clientTestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("response")),
				Request:    req,
			}, nil
		})),
	)
	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "response"; got != want {
		t.Fatalf("Do() body = %q; want %q", got, want)
	}
}
