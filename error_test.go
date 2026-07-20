package ghttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// test gitlab
type gitlabErr struct {
	Message          any    `json:"message"`
	Err              string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (g *gitlabErr) Error() string {
	if g.ErrorDescription != "" {
		return g.ErrorDescription
	}
	if g.Err != "" {
		return g.Err
	}
	if g.Message != nil {
		switch msg := g.Message.(type) {
		case string:
			return msg
		default:
			b, _ := json.Marshal(g.Message)
			return string(b)
		}
	}
	return ""
}

func TestError_Error(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://gitlab.com/oauth/token", nil)
	e := &Error{
		Request:    req,
		StatusCode: http.StatusBadRequest,
		Err: &gitlabErr{
			Err:              "invalid_request",
			ErrorDescription: "Missing required parameter: grant_type.",
		},
	}

	want := `POST "https://gitlab.com/oauth/token" [400] - Missing required parameter: grant_type.`
	if got := e.Error(); got != want {
		t.Fatalf("Error() = %q; want %q", got, want)
	}
}

func TestError_Unwrap(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://gitlab.com/oauth/token", nil)
	ge := &gitlabErr{
		Err:              "invalid_request",
		ErrorDescription: "Missing required parameter: grant_type.",
	}
	e := &Error{
		Request:    req,
		StatusCode: http.StatusBadRequest,
		Err:        ge,
	}
	if !errors.Is(e, ge) {
		t.Fatal("errors.Is() did not find wrapped gitlabErr")
	}
	var ge2 *gitlabErr
	if !errors.As(e, &ge2) {
		t.Fatal("errors.As() did not find wrapped *gitlabErr")
	}
	if ge2 != ge {
		t.Fatalf("errors.As() = %p; want %p", ge2, ge)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return false }

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "regular error", err: errors.New("failed"), want: false},
		{name: "context deadline", err: context.DeadlineExceeded, want: true},
		{name: "wrapped context deadline", err: fmt.Errorf("request: %w", context.DeadlineExceeded), want: true},
		{name: "network timeout", err: timeoutError{}, want: true},
		{
			name: "wrapped URL network timeout",
			err:  &url.Error{Op: http.MethodGet, URL: "https://example.com", Err: timeoutError{}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTimeout(tt.err); got != tt.want {
				t.Fatalf("IsTimeout(%v) = %t; want %t", tt.err, got, tt.want)
			}
		})
	}
}

func TestFromError(t *testing.T) {
	want := &Error{StatusCode: http.StatusBadRequest, Err: errors.New("bad request")}
	wrapped := fmt.Errorf("request failed: %w", want)

	got, ok := FromError(wrapped)
	if !ok {
		t.Fatal("FromError() did not find wrapped *Error")
	}
	if got != want {
		t.Fatalf("FromError() = %p; want %p", got, want)
	}

	status, ok := StatusCode(wrapped)
	if !ok || status != http.StatusBadRequest {
		t.Fatalf("StatusCode() = (%d, %t); want (%d, true)", status, ok, http.StatusBadRequest)
	}
}

func TestFromErrorRejectsNonGHTTPErrorAndTypedNil(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "nil", err: nil},
		{name: "unrelated", err: errors.New("unrelated")},
		{name: "typed nil", err: (*Error)(nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := FromError(test.err); ok || got != nil {
				t.Fatalf("FromError() = (%v, %t); want (nil, false)", got, ok)
			}
			if status, ok := StatusCode(test.err); ok || status != 0 {
				t.Fatalf("StatusCode() = (%d, %t); want (0, false)", status, ok)
			}
		})
	}
}
