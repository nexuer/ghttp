package ghttp

import (
	"fmt"
	"net/http"
	"testing"
)

func TestClient_Do(t *testing.T) {
	opts := []ClientOption{
		//WithTimeout(1 * time.Millisecond),
		WithEndpoint("https://gitlab.com"),
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
