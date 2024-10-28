package ghttp

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Error struct {
	URL        *url.URL
	Method     string
	StatusCode int
	Err        error
}

func newError(req *http.Request, response *http.Response, err error) *Error {
	e := &Error{
		URL:    req.URL,
		Method: req.Method,
		Err:    err,
	}
	if response != nil {
		e.StatusCode = response.StatusCode
	}
	return e
}

func (e Error) Error() string {
	var buf strings.Builder

	if e.Method != "" {
		buf.WriteString(e.Method)
		buf.WriteByte(' ')
	}

	if e.URL != nil {
		buf.WriteString(`"`)
		buf.WriteString(e.URL.String())
		buf.WriteString(`"`)
		buf.WriteByte(' ')
	}
	if e.StatusCode > 0 {
		buf.WriteByte('[')
		buf.WriteString(strconv.Itoa(e.StatusCode))
		buf.WriteByte(']')
		buf.WriteByte(' ')
	}
	if e.Err != nil {
		buf.WriteString("- ")
		buf.WriteString(e.Err.Error())
	}
	return buf.String()
}

func (e Error) Unwrap() error {
	return e.Err
}

func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}
