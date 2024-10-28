package query

import (
	"net/url"
	"reflect"
	"time"
)

var tags = [2]string{"query", "url"}

const (
	omitemptyTagOpt = "omitempty"
	intTagOpt       = "int"
	inlineTagOpt    = "inline"

	unixTagOpt      = "unix"
	unixmilliTagOpt = "unixmilli"
	unixnanoTagOpt  = "unixnano"
	numberedTagOpt  = "numbered"

	layoutTag = "layout"
	delTag    = "del"
)

var encoderType = reflect.TypeOf(new(Encoder)).Elem()

// Encoder is an interface implemented by any type that wishes to encode
// itself into URL values in a non-standard way.
type Encoder interface {
	EncodeValues(key string, v *url.Values) error
}

var timeType = reflect.TypeOf(time.Time{})
