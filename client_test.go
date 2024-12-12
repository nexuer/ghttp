package ghttp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestClient_Do(t *testing.T) {
	opts := []ClientOption{
		//WithTimeout(1 * time.Millisecond),
		WithEndpoint("https://gtilab.com"),
		WithDebug(true),
		WithNot2xxError(func() error {
			return &gitlabErr{}
		}),
		//WithDebugInterface(func() DebugInterface {
		//	return &Debug{
		//		Trace:  false,
		//		Writer: os.Stdout,
		//	}
		//}),
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
	fmt.Println(resp.StatusCode)
}

func TestInvoke_WithLimiter(t *testing.T) {
	opts := []ClientOption{
		WithDebug(false),
		WithNot2xxError(func() error {
			return &gitlabErr{}
		}),
		WithLimiter(rate.NewLimiter(1, 1)),
	}
	c := NewClient(opts...)
	for i := 0; i < 100; i++ {
		_, err := c.Invoke(context.Background(), http.MethodGet, "127.0.0.1", nil, nil)
		if err != nil {
			t.Errorf("now: %s err: %s", time.Now(), err)
		} else {
			t.Errorf("now: %s", time.Now())
		}
	}
}

func TestDo_WithLimiter(t *testing.T) {
	opts := []ClientOption{
		//WithTimeout(1 * time.Millisecond),
		WithDebug(false),
		WithNot2xxError(func() error {
			return &gitlabErr{}
		}),
		WithLimiter(rate.NewLimiter(1, 1)),
	}
	c := NewClient(opts...)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
		_, err := c.Do(req)
		if err != nil {
			t.Errorf("now: %s err: %s", time.Now(), err)
		} else {
			t.Errorf("now: %s", time.Now())
		}
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
