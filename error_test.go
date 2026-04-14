package ghttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

	t.Logf("err: %s", e)
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
	t.Logf("errors.Is(Error, gitlabErr): %t", errors.Is(e, ge))
	var ge2 *gitlabErr
	t.Logf("errors.As(Error, gitlabErr): %t - gitlab err: %v", errors.As(e, &ge2), ge2)
}
