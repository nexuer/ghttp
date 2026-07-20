package ghttp_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nexuer/ghttp"
)

type callOptionRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f callOptionRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func requestsForCallOption(t *testing.T, opts ...ghttp.CallOption) []*http.Request {
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
		"http://example.test/metadata", nil, nil, opts...); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.test/metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req, opts...)
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

func TestCallOptionOrder(t *testing.T) {
	for _, req := range requestsForCallOption(t,
		ghttp.Before(func(req *http.Request) error {
			if got := req.URL.Query().Get("page"); got != "2" {
				t.Fatalf("Before hook page = %q; want 2", got)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer token" {
				t.Fatalf("Before hook Authorization = %q; want Bearer token", got)
			}
			if got := req.Header.Get("Content-Type"); got != "text/plain" {
				t.Fatalf("Before hook Content-Type = %q; want text/plain", got)
			}
			if got := req.Header.Get("Accept"); got != "text/plain" {
				t.Fatalf("Before hook Accept = %q; want text/plain", got)
			}
			return nil
		}),
		ghttp.BearerToken("token"),
		ghttp.Query(map[string]any{"page": 1}),
		ghttp.Query(map[string]any{"page": 2}),
		ghttp.ContentType("text/plain"),
	) {
		if got := req.URL.Query()["page"]; len(got) != 1 || got[0] != "2" {
			t.Fatalf("page values = %v; want [2]", got)
		}
		if got := req.Header.Get("Accept"); got != "text/plain" {
			t.Fatalf("Accept = %q; want text/plain", got)
		}
	}
}

func TestLastNonEmptyAuthOptionWins(t *testing.T) {
	tests := []struct {
		name     string
		opts     []ghttp.CallOption
		wantAuth string
	}{
		{
			name: "bearer",
			opts: []ghttp.CallOption{
				ghttp.BasicAuth("username", "password"),
				ghttp.BearerToken("token"),
			},
			wantAuth: "Bearer token",
		},
		{
			name: "basic",
			opts: []ghttp.CallOption{
				ghttp.BearerToken("token"),
				ghttp.BasicAuth("username", "password"),
				ghttp.BearerToken(""),
			},
			wantAuth: "Basic dXNlcm5hbWU6cGFzc3dvcmQ=",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, req := range requestsForCallOption(t, test.opts...) {
				if got := req.Header.Get("Authorization"); got != test.wantAuth {
					t.Fatalf("Authorization = %q; want %q", got, test.wantAuth)
				}
			}
		})
	}
}

func TestInvokeContentTypeSelectsCodec(t *testing.T) {
	type capturedRequest struct {
		contentType string
		accept      string
		body        string
	}
	captured := make(chan capturedRequest, 1)
	client := ghttp.NewClient(ghttp.WithTransport(callOptionRoundTripperFunc(
		func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			captured <- capturedRequest{
				contentType: req.Header.Get("Content-Type"),
				accept:      req.Header.Get("Accept"),
				body:        string(body),
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		},
	)))

	if _, err := client.Invoke(context.Background(), http.MethodPost,
		"http://example.test/data", "plain body", nil,
		ghttp.ContentType("text/plain")); err != nil {
		t.Fatal(err)
	}

	got := <-captured
	if got.contentType != "text/plain" || got.accept != "text/plain" {
		t.Fatalf("headers = (%q, %q); want (text/plain, text/plain)",
			got.contentType, got.accept)
	}
	if got.body != "plain body" {
		t.Fatalf("body = %q; want %q", got.body, "plain body")
	}
}

func TestDoContentTypeHeaders(t *testing.T) {
	type capturedRequest struct {
		contentType string
		accept      string
		body        string
	}
	captured := make(chan capturedRequest, 2)
	client := ghttp.NewClient(ghttp.WithTransport(callOptionRoundTripperFunc(
		func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			captured <- capturedRequest{
				contentType: req.Header.Get("Content-Type"),
				accept:      req.Header.Get("Accept"),
				body:        string(body),
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		},
	)))

	req, err := http.NewRequest(http.MethodPost, "http://example.test/data", strings.NewReader("encoded"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	got := <-captured
	if got.contentType != "application/xml" || got.accept != "application/json" {
		t.Fatalf("preserved headers = (%q, %q); want (application/xml, application/json)",
			got.contentType, got.accept)
	}

	req, err = http.NewRequest(http.MethodPost, "http://example.test/data", strings.NewReader("encoded"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req, ghttp.ContentType("text/plain"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	got = <-captured
	if got.contentType != "text/plain" || got.accept != "text/plain" {
		t.Fatalf("overridden headers = (%q, %q); want (text/plain, text/plain)",
			got.contentType, got.accept)
	}
	if got.body != "encoded" {
		t.Fatalf("body = %q; want unchanged body %q", got.body, "encoded")
	}
}

func TestHooksRunInRegistrationOrder(t *testing.T) {
	var calls []string
	client := ghttp.NewClient(ghttp.WithTransport(callOptionRoundTripperFunc(
		func(req *http.Request) (*http.Response, error) {
			calls = append(calls, "transport")
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		},
	)))

	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req,
		ghttp.After(func(*http.Response) error {
			calls = append(calls, "after-1")
			return nil
		}),
		ghttp.Before(func(*http.Request) error {
			calls = append(calls, "before-1")
			return nil
		}),
		ghttp.After(func(*http.Response) error {
			calls = append(calls, "after-2")
			return nil
		}),
		ghttp.Before(func(*http.Request) error {
			calls = append(calls, "before-2")
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	want := []string{"before-1", "before-2", "transport", "after-1", "after-2"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("hook order = %v; want %v", calls, want)
	}
}
