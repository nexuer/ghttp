package ghttp

import (
	"context"
	"net/http"
)

type Limiter interface {
	Wait(ctx context.Context) error
}

type CallOption interface {
	Before(request *http.Request) error
	After(response *http.Response) error
}

func Query(q any) CallOption {
	return queryCallOption{query: q}
}

type queryCallOption struct {
	query any
}

func (q queryCallOption) Before(request *http.Request) error {
	return SetQuery(request, q.query)
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

func BearerToken(token string) CallOption {
	return bearerTokenCallOption{token}
}

type bearerTokenCallOption struct {
	token string
}

func (b bearerTokenCallOption) Before(request *http.Request) error {
	if b.token != "" {
		request.Header.Set("Authorization", "Bearer "+b.token)
	}
	return nil
}

func (b bearerTokenCallOption) After(response *http.Response) error {
	return nil
}

func Before(hooks ...RequestFunc) CallOption {
	return beforeHooksCallOption{hooks}
}

type beforeHooksCallOption struct {
	hooks []RequestFunc
}

func (b beforeHooksCallOption) Before(request *http.Request) error {
	for _, f := range b.hooks {
		if err := f(request); err != nil {
			return err
		}
	}
	return nil
}

func (b beforeHooksCallOption) After(response *http.Response) error {
	return nil
}

func After(hooks ...ResponseFunc) CallOption {
	return afterHooksCallOption{hooks}
}

type afterHooksCallOption struct {
	hooks []ResponseFunc
}

func (b afterHooksCallOption) Before(request *http.Request) error {
	return nil
}

func (b afterHooksCallOption) After(response *http.Response) error {
	for _, f := range b.hooks {
		if err := f(response); err != nil {
			return err
		}
	}
	return nil
}

type RequestFunc func(request *http.Request) error
type ResponseFunc func(response *http.Response) error

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
	BeforeHooks []RequestFunc
	AfterHooks  []ResponseFunc
}

func (c *CallOptions) Before(request *http.Request) error {
	for _, f := range c.BeforeHooks {
		if err := f(request); err != nil {
			return err
		}
	}

	if err := SetQuery(request, c.Query); err != nil {
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
	for _, f := range c.AfterHooks {
		if err := f(response); err != nil {
			return err
		}
	}
	return nil
}
