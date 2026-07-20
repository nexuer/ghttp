package ghttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type clientTestRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f clientTestRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type countingLimiter struct {
	calls atomic.Int32
	err   error
}

func (l *countingLimiter) Wait(context.Context) error {
	l.calls.Add(1)
	return l.err
}

func TestClient_Do(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("method = %q; want GET", req.Method)
		}
		if req.URL.Path != "/api/v4/projects" {
			t.Errorf("path = %q; want /api/v4/projects", req.URL.Path)
		}
		if got := req.URL.Query().Get("membership"); got != "true" {
			t.Errorf("membership = %q; want true", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"projects":[]}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(WithEndpoint(server.URL))
	req, err := http.NewRequest(http.MethodGet, "api/v4/projects", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req, Query(map[string]interface{}{"membership": true}))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
}

func TestInvoke_WithLimiter(t *testing.T) {
	limiter := &countingLimiter{}
	client := NewClient(
		WithLimiter(limiter),
		WithTransport(clientTestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	if _, err := client.Invoke(context.Background(), http.MethodGet, "http://example.test", nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := limiter.calls.Load(); got != 1 {
		t.Fatalf("limiter Wait calls = %d; want 1", got)
	}
}

func TestDo_WithLimiter(t *testing.T) {
	limiter := &countingLimiter{}
	client := NewClient(
		WithLimiter(limiter),
		WithTransport(clientTestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)
	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := limiter.calls.Load(); got != 1 {
		t.Fatalf("limiter Wait calls = %d; want 1", got)
	}
}

func TestClient_LimiterError(t *testing.T) {
	wantErr := errors.New("limited")
	limiter := &countingLimiter{err: wantErr}
	var transportCalls atomic.Int32
	client := NewClient(
		WithLimiter(limiter),
		WithTransport(clientTestRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			transportCalls.Add(1)
			return nil, errors.New("transport should not be called")
		})),
	)
	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Do(req); !errors.Is(err, wantErr) {
		t.Fatalf("Do() error = %v; want %v", err, wantErr)
	}
	if got := limiter.calls.Load(); got != 1 {
		t.Fatalf("limiter Wait calls = %d; want 1", got)
	}
	if got := transportCalls.Load(); got != 0 {
		t.Fatalf("transport calls = %d; want 0", got)
	}
}

func TestInvokeLimiterDoesNotRunWhenBodyMarshalFails(t *testing.T) {
	limiter := &countingLimiter{}
	client := NewClient(WithLimiter(limiter))

	_, err := client.Invoke(context.Background(), http.MethodPost, "http://example.test", make(chan int), nil)
	if err == nil {
		t.Fatal("Invoke() error = nil; want body marshal error")
	}
	if got := limiter.calls.Load(); got != 0 {
		t.Fatalf("limiter Wait calls = %d; want 0", got)
	}
}

func TestInvokeLimiterDoesNotRunForUnsupportedContentType(t *testing.T) {
	limiter := &countingLimiter{}
	client := NewClient(WithLimiter(limiter))

	_, err := client.Invoke(context.Background(), http.MethodPost, "http://example.test",
		"body", nil, ContentType("application/unsupported"))
	if err == nil {
		t.Fatal("Invoke() error = nil; want unsupported content type error")
	}
	if got := limiter.calls.Load(); got != 0 {
		t.Fatalf("limiter Wait calls = %d; want 0", got)
	}
}

func TestInvokeLimiterDoesNotRunWhenRequestCreationFails(t *testing.T) {
	limiter := &countingLimiter{}
	client := NewClient(WithLimiter(limiter))

	_, err := client.Invoke(context.Background(), "invalid\nmethod", "http://example.test", nil, nil)
	if err == nil {
		t.Fatal("Invoke() error = nil; want request creation error")
	}
	if got := limiter.calls.Load(); got != 0 {
		t.Fatalf("limiter Wait calls = %d; want 0", got)
	}
}

func TestLimiterDoesNotRunWhenEndpointParsingFails(t *testing.T) {
	limiter := &countingLimiter{}
	client := NewClient(WithLimiter(limiter), WithEndpoint("http://%"))
	req, err := http.NewRequest(http.MethodGet, "/path", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = client.Do(req); err == nil {
		t.Fatal("Do() error = nil; want endpoint parsing error")
	}
	if got := limiter.calls.Load(); got != 0 {
		t.Fatalf("limiter Wait calls = %d; want 0", got)
	}
}

func TestLimiterDoesNotRunWhenBeforeHookFails(t *testing.T) {
	wantErr := errors.New("before hook failed")
	for _, test := range []struct {
		name string
		call func(*Client, CallOption) error
	}{
		{
			name: "Invoke",
			call: func(client *Client, option CallOption) error {
				_, err := client.Invoke(context.Background(), http.MethodGet, "http://example.test", nil, nil, option)
				return err
			},
		},
		{
			name: "Do",
			call: func(client *Client, option CallOption) error {
				req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
				if err != nil {
					return err
				}
				_, err = client.Do(req, option)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			limiter := &countingLimiter{}
			client := NewClient(WithLimiter(limiter))
			option := Before(func(*http.Request) error { return wantErr })

			err := test.call(client, option)
			if !errors.Is(err, wantErr) {
				t.Fatalf("request error = %v; want %v", err, wantErr)
			}
			if got := limiter.calls.Load(); got != 0 {
				t.Fatalf("limiter Wait calls = %d; want 0", got)
			}
		})
	}
}

func TestTextPlain(t *testing.T) {
	client := NewClient()
	wantErr := `Get "/path": unsupported protocol scheme ""`

	_, err := client.Invoke(context.Background(), http.MethodGet, "/path", "text data", nil)
	if err == nil || err.Error() != wantErr {
		t.Fatalf("Invoke() error = %v; want %q", err, wantErr)
	}

	req, err := http.NewRequest(http.MethodGet, "/path", strings.NewReader("text data"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain")
	_, err = client.Do(req)
	if err == nil || err.Error() != wantErr {
		t.Fatalf("Do() error = %v; want %q", err, wantErr)
	}
}
