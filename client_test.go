package ghttp

import (
	"context"
	"errors"
	"fmt"
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

func TestClient_Do1(t *testing.T) {
	opts := []ClientOption{
		//WithTimeout(1 * time.Millisecond),
		WithEndpoint("https://gitlab.com"),
		WithDebug(true),
		WithNot2xxError(func() error {
			return &gitlabErr{}
		}),
	}
	c := NewClient(opts...)

	req, err := http.NewRequest(http.MethodGet, "api/v4/projects", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req, &CallOptions{
		Query: map[string]interface{}{
			"membership": true,
		},
	})
	if err != nil {
		if IsTimeout(err) {
			fmt.Println("timeout!")
		}
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(resp.StatusCode)
	fmt.Println(string(body))
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
	resp, err := client.Do(req, &CallOptions{
		Query: map[string]interface{}{"membership": true},
	})
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

func TestTextPlain(t *testing.T) {
	// global
	client := NewClient(
		WithDebug(true),
	)
	// The default json is used again
	fmt.Println("---------------------------------- Invoke ----------------------------------")
	_, err := client.Invoke(context.Background(), http.MethodGet, "/path", "text data", nil)
	if err != nil && err.Error() != `Get "/path": unsupported protocol scheme ""` {
		t.Fatal(err)
	}
	fmt.Println("---------------------------------- Do ----------------------------------")
	// If you need to use the 'text/plain 'type for just one request, you can only use Do()
	req, err := http.NewRequest(http.MethodGet, "/path", strings.NewReader("text data"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain")
	_, err = client.Do(req)
	if err != nil && err.Error() != `Get "/path": unsupported protocol scheme ""` {
		t.Fatal(err)
	}
}
