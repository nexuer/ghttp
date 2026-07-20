package ghttp

import "net/http"

// CallOption configures one client call.
type CallOption func(*callOptions)

// BeforeHook runs after structured request options are applied and before the
// request is sent.
type BeforeHook func(request *http.Request) error

// AfterHook runs after a response is received and before it is processed.
type AfterHook func(response *http.Response) error

type callOptions struct {
	contentType string
	query       any
	applyAuth   func(*http.Request)
	beforeHooks []BeforeHook
	afterHooks  []AfterHook
}

func resolveCallOptions(opts ...CallOption) *callOptions {
	options := new(callOptions)
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
	return options
}

// Query sets query parameters for one call. The last non-nil Query wins.
func Query(q any) CallOption {
	return func(o *callOptions) {
		if q != nil {
			o.query = q
		}
	}
}

// ContentType overrides Content-Type and Accept for one call. Invoke also uses
// it to select the request body codec.
func ContentType(contentType string) CallOption {
	return func(o *callOptions) {
		if contentType != "" {
			o.contentType = contentType
		}
	}
}

// BasicAuth configures HTTP Basic Authentication. The last non-empty auth
// option wins.
func BasicAuth(username, password string) CallOption {
	return func(o *callOptions) {
		if username == "" && password == "" {
			return
		}
		o.applyAuth = func(request *http.Request) {
			request.SetBasicAuth(username, password)
		}
	}
}

// BearerToken configures bearer token authentication. The last non-empty auth
// option wins.
func BearerToken(token string) CallOption {
	return func(o *callOptions) {
		if token == "" {
			return
		}
		o.applyAuth = func(request *http.Request) {
			request.Header.Set("Authorization", "Bearer "+token)
		}
	}
}

// Before appends hooks that run before the request is sent.
func Before(hooks ...BeforeHook) CallOption {
	return func(o *callOptions) {
		o.beforeHooks = append(o.beforeHooks, hooks...)
	}
}

// After appends hooks that run after a response is received.
func After(hooks ...AfterHook) CallOption {
	return func(o *callOptions) {
		o.afterHooks = append(o.afterHooks, hooks...)
	}
}
