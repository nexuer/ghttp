package ghttp_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/nexuer/ghttp"
)

func TestBasicAuth(t *testing.T) {
	client := ghttp.NewClient(
		ghttp.WithDebug(true),
		ghttp.WithUserAgent("go-http-client"),
		ghttp.WithEndpoint("https://gitlab.com/api/v4"),
	)
	fmt.Println("---------------------------------- Invoke ----------------------------------")
	var reply any
	_, err := client.Invoke(context.Background(), http.MethodGet, "metadata", nil, &reply,
		ghttp.BasicAuth("username", "password"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("---------------------------------- Do ----------------------------------")
	req, err := http.NewRequest(http.MethodGet, "metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req, ghttp.BasicAuth("username", "password"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuery(t *testing.T) {
	client := ghttp.NewClient(
		ghttp.WithDebug(true),
		ghttp.WithUserAgent("go-http-client"),
		ghttp.WithEndpoint("https://gitlab.com/api/v4"),
	)

	fmt.Println("---------------------------------- Invoke ----------------------------------")
	var reply any
	_, err := client.Invoke(context.Background(), http.MethodGet, "metadata", nil, &reply,
		ghttp.Query(map[string]any{"page": "1", "size": 10}))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("---------------------------------- Do ----------------------------------")
	req, err := http.NewRequest(http.MethodGet, "metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req, ghttp.Query(map[string]any{"page": "2", "size": 10}))
	if err != nil {
		t.Fatal(err)
	}
}

func TestBearerToken(t *testing.T) {
	client := ghttp.NewClient(
		ghttp.WithDebug(true),
		ghttp.WithUserAgent("go-http-client"),
		ghttp.WithEndpoint("https://gitlab.com/api/v4"),
	)
	fmt.Println("---------------------------------- Invoke ----------------------------------")
	var reply any
	_, err := client.Invoke(context.Background(), http.MethodGet, "metadata", nil, &reply,
		ghttp.BearerToken("xxxxxxxx"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("---------------------------------- Do ----------------------------------")
	req, err := http.NewRequest(http.MethodGet, "metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req, ghttp.BearerToken("xxxxxxxx"))
	if err != nil {
		t.Fatal(err)
	}
}
