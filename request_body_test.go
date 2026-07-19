package ghttp

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type requestBodyTrackingReadCloser struct {
	io.Reader
	closed bool
}

func (b *requestBodyTrackingReadCloser) Close() error {
	b.closed = true
	return nil
}

func TestSetRequestBodyClearsStaleMetadata(t *testing.T) {
	oldBody := &requestBodyTrackingReadCloser{Reader: strings.NewReader("old")}
	req, err := http.NewRequest(http.MethodPost, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Body = oldBody
	req.ContentLength = 3
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("old")), nil
	}

	newBody := bufio.NewReader(strings.NewReader("new body"))
	if err := SetRequestBody(req, newBody); err != nil {
		t.Fatal(err)
	}

	if !oldBody.closed {
		t.Fatal("old request body was not closed")
	}
	if req.GetBody != nil {
		t.Fatal("GetBody still refers to the old request body")
	}
	if req.ContentLength != 0 {
		t.Fatalf("ContentLength = %d, want 0 for an unknown length", req.ContentLength)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "new body"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestSetRequestBodyReplacesReplayableMetadata(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://example.test", strings.NewReader("old"))
	if err != nil {
		t.Fatal(err)
	}

	if err := SetRequestBody(req, strings.NewReader("new body")); err != nil {
		t.Fatal(err)
	}

	if got, want := req.ContentLength, int64(len("new body")); got != want {
		t.Fatalf("ContentLength = %d, want %d", got, want)
	}
	if req.GetBody == nil {
		t.Fatal("GetBody is nil for a replayable body")
	}
	replay, err := req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	body, err := io.ReadAll(replay)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "new body"; got != want {
		t.Fatalf("replayed body = %q, want %q", got, want)
	}
}

func TestSetRequestBodyDoesNotReplayOldBodyOn307Redirect(t *testing.T) {
	requestBody := make(chan string, 1)
	var redirected atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/start":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
				return
			}
			requestBody <- string(body)
			http.Redirect(w, req, "/redirected", http.StatusTemporaryRedirect)
		case "/redirected":
			redirected.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/start", strings.NewReader("old"))
	if err != nil {
		t.Fatal(err)
	}
	newBody := bufio.NewReader(strings.NewReader("new"))
	if err := SetRequestBody(req, newBody); err != nil {
		t.Fatal(err)
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if got, want := response.StatusCode, http.StatusTemporaryRedirect; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := <-requestBody, "new"; got != want {
		t.Fatalf("initial request body = %q, want %q", got, want)
	}
	if redirected.Load() {
		t.Fatal("307 redirect replayed the stale request body")
	}
}
