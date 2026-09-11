package ghttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
)

var errBodyTooLarge = errors.New("body too large")

// Error describes a failed HTTP request and wraps its underlying error.
type Error struct {
	// The http status code returned.
	StatusCode int
	// The request that failed.
	Request *http.Request

	// Err is the underlying error.
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

// Error returns a description of the failed request.
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

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsBodyTooLarge reports whether err was caused by a body
// exceeding the configured maximum size.
func IsBodyTooLarge(err error) bool {
	return errors.Is(err, errBodyTooLarge)
}

// IsTimeout reports whether err represents a context or network timeout.
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

// FromError finds the first *Error in err's unwrap chain.
func FromError(err error) (*Error, bool) {
	var e *Error
	if !errors.As(err, &e) || e == nil {
		return nil, false
	}
	return e, true
}

// StatusCode returns the HTTP status code carried by an *Error.
func StatusCode(err error) (int, bool) {
	e, ok := FromError(err)
	if !ok {
		return 0, false
	}
	return e.StatusCode, true
}
