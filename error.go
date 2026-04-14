package ghttp

import (
	"context"
	"errors"
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

func (e Error) Error() string {
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

func (e Error) Unwrap() error {
	return e.Err
}

func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func StatusForErr(err error) (int, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode, true
	}
	return 0, false
}
