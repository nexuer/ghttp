package ghttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type Error struct {
	// The http status code returned.
	StatusCode int
	// The request that failed.
	Request *http.Request

	Err error
}

func newError(req *http.Request, response *http.Response, err error) *Error {
	e := &Error{
		Request: req,
		Err:     err,
	}
	if response != nil {
		e.StatusCode = response.StatusCode
	}
	return e
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	var buf strings.Builder

	if e.Request != nil {
		buf.WriteString(e.Request.Method)
		buf.WriteByte(' ')
		if e.Request.URL != nil {
			buf.WriteString(`"`)
			buf.WriteString(e.Request.URL.String())
			buf.WriteString(`"`)
			buf.WriteByte(' ')
		}
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

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func FromError(err error) (*Error, bool) {
	var e *Error
	if !errors.As(err, &e) || e == nil {
		return nil, false
	}
	return e, true
}

func StatusCode(err error) (int, bool) {
	e, ok := FromError(err)
	if !ok {
		return 0, false
	}
	return e.StatusCode, true
}
