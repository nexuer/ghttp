package ghttp

import (
	"context"
	"net/http"

	"github.com/nexuer/ghttp/query"
)

type Limiter interface {
	Wait(ctx context.Context) error
}

type CallOption interface {
	Before(request *http.Request) error
	After(response *http.Response) error
}

func setQuery(req *http.Request, q any) error {
	if q == nil {
		return nil
	}
	values, err := query.Values(q)
	if err != nil {
		return err
	}
	queryStr := values.Encode()
	if queryStr == "" {
		return nil
	}

	if req.URL.RawQuery == "" {
		req.URL.RawQuery = queryStr
	} else {
		req.URL.RawQuery += "&" + queryStr
	}
	return nil
}

func Query(q any) CallOption {
	return queryCallOption{query: q}
}

type queryCallOption struct {
	query any
}

func (q queryCallOption) Before(request *http.Request) error {
	return setQuery(request, q.query)
}

func (q queryCallOption) After(response *http.Response) error {
	return nil
}

func BasicAuth(username, password string) CallOption {
	return basicAuthCallOption{username, password}
}

type basicAuthCallOption struct {
	username string
	password string
}

func (b basicAuthCallOption) Before(request *http.Request) error {
	request.SetBasicAuth(b.username, b.password)
	if b.username != "" || b.password != "" {
		request.SetBasicAuth(b.username, b.password)
	}
	return nil
}

func (b basicAuthCallOption) After(response *http.Response) error {
	return nil
}

// CallOptions default call options
type CallOptions struct {
	// Set query parameters
	Query any

	// Basic auth
	Username string
	Password string

	// Bearer token
	BearerToken string

	// hooks
	BeforeHook func(request *http.Request) error
	AfterHook  func(response *http.Response) error
}

func (c *CallOptions) Before(request *http.Request) error {
	if c.BeforeHook != nil {
		if err := c.BeforeHook(request); err != nil {
			return err
		}
	}

	if err := setQuery(request, c.Query); err != nil {
		return err
	}

	if c.Username != "" || c.Password != "" {
		request.SetBasicAuth(c.Username, c.Password)
	}

	if c.BearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}

	return nil
}

func (c *CallOptions) After(response *http.Response) error {
	if c.AfterHook != nil {
		if err := c.AfterHook(response); err != nil {
			return err
		}
	}
	return nil
}
