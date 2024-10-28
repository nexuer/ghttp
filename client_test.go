package ghttp

import (
	"fmt"
	"net/http"
	"testing"
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

	req, err := http.NewRequest(http.MethodGet, "api/v4/version", nil)
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
