package ghttp

import (
	"net/http"

	"github.com/nexuer/ghttp/query"
)

type CallOption interface {
	Before(request *http.Request) error
	After(response *http.Response) error
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
	if c.Query != nil {
		values, err := query.Values(c.Query)
		if err != nil {
			return err
		}
		q := values.Encode()
		if q != "" {
			if request.URL.RawQuery == "" {
				request.URL.RawQuery = q
			} else {
				request.URL.RawQuery += "&" + q
			}
		}
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
