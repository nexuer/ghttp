package ghttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type timeoutRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f timeoutRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type timeoutTrackingReadCloser struct {
	io.Reader
	closed bool
}

func (b *timeoutTrackingReadCloser) Close() error {
	b.closed = true
	return nil
}

func TestClientDoKeepsDefaultTimeoutUntilBodyFinishes(t *testing.T) {
	releaseBody := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "7")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-releaseBody
		_, _ = io.WriteString(w, "delayed")
	}))
	t.Cleanup(server.Close)

	client := NewClient(WithTimeout(time.Second))
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	close(releaseBody)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read delayed response body: %v", err)
	}
	if got, want := string(body), "delayed"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestClientInvokeWithNilReplyClosesBodyAndCancelsDefaultContext(t *testing.T) {
	body := &timeoutTrackingReadCloser{Reader: strings.NewReader("body")}
	requestContext := make(chan context.Context, 1)
	transport := timeoutRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestContext <- req.Context()
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Header:        make(http.Header),
			Body:          body,
			ContentLength: 4,
			Request:       req,
		}, nil
	})
	client := NewClient(
		WithTransport(transport),
		WithTimeout(time.Hour),
	)

	response, err := client.Invoke(context.Background(), http.MethodGet, "http://example.test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != body {
		t.Fatal("Invoke replaced the response body")
	}
	if !body.closed {
		t.Fatal("Invoke did not close the response body when reply is nil")
	}

	ctx := <-requestContext
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Invoke did not cancel the default timeout context before returning")
	}
}

func TestCancelResponseBodyCancelsDefaultContextOnEOFAndClose(t *testing.T) {
	tests := []struct {
		name   string
		finish func(*testing.T, io.ReadCloser)
	}{
		{
			name: "EOF",
			finish: func(t *testing.T, body io.ReadCloser) {
				t.Helper()
				if _, err := io.ReadAll(body); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "Close",
			finish: func(t *testing.T, body io.ReadCloser) {
				t.Helper()
				if err := body.Close(); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestContext := make(chan context.Context, 1)
			transport := timeoutRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requestContext <- req.Context()
				return &http.Response{
					StatusCode:    http.StatusOK,
					Status:        "200 OK",
					Header:        make(http.Header),
					Body:          io.NopCloser(strings.NewReader("body")),
					ContentLength: 4,
					Request:       req,
				}, nil
			})

			client := NewClient(
				WithTransport(transport),
				WithTimeout(time.Hour),
			)
			req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
			if err != nil {
				t.Fatal(err)
			}

			response, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			ctx := <-requestContext
			select {
			case <-ctx.Done():
				t.Fatal("request context canceled before response body finished")
			default:
			}

			tt.finish(t, response.Body)

			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("request context was not canceled after response body finished")
			}
		})
	}
}

func TestClientDoDoesNotCancelCallerDeadline(t *testing.T) {
	transport := timeoutRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader("body")),
			ContentLength: 4,
			Request:       req,
		}, nil
	})
	client := NewClient(
		WithTransport(transport),
		WithTimeout(time.Nanosecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ctx.Done():
		t.Fatal("client canceled a context owned by the caller")
	default:
	}
}

func TestClientDoCancelsDefaultContextForNoBody(t *testing.T) {
	requestContext := make(chan context.Context, 1)
	transport := timeoutRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestContext <- req.Context()
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Status:     "204 No Content",
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    req,
		}, nil
	})
	client := NewClient(
		WithTransport(transport),
		WithTimeout(time.Hour),
	)
	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != http.NoBody {
		t.Fatal("Do did not preserve http.NoBody")
	}

	ctx := <-requestContext
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("default timeout context was not canceled for http.NoBody")
	}
}
