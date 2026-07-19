package ghttp_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/nexuer/ghttp"
)

type callOptionRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f callOptionRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func requestsForCallOption(t *testing.T, option ghttp.CallOption) []*http.Request {
	t.Helper()

	requests := make(chan *http.Request, 2)
	client := ghttp.NewClient(ghttp.WithTransport(callOptionRoundTripperFunc(
		func(req *http.Request) (*http.Response, error) {
			requests <- req.Clone(req.Context())
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		},
	)))

	if _, err := client.Invoke(context.Background(), http.MethodGet,
		"http://example.test/metadata", nil, nil, option); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.test/metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req, option)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}

	return []*http.Request{<-requests, <-requests}
}

func TestBasicAuth(t *testing.T) {
	for _, req := range requestsForCallOption(t, ghttp.BasicAuth("username", "password")) {
		username, password, ok := req.BasicAuth()
		if !ok || username != "username" || password != "password" {
			t.Fatalf("BasicAuth() = (%q, %q, %t); want (username, password, true)",
				username, password, ok)
		}
	}
}

func TestQuery(t *testing.T) {
	for _, req := range requestsForCallOption(t,
		ghttp.Query(map[string]any{"page": "1", "size": 10})) {
		query := req.URL.Query()
		if got := query.Get("page"); got != "1" {
			t.Fatalf("page = %q; want 1", got)
		}
		if got := query.Get("size"); got != "10" {
			t.Fatalf("size = %q; want 10", got)
		}
	}
}

func TestBearerToken(t *testing.T) {
	for _, req := range requestsForCallOption(t, ghttp.BearerToken("token")) {
		if got := req.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q; want Bearer token", got)
		}
	}
}
