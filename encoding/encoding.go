package encoding

import "strings"

// Codec encodes and decodes messages. Implementations must be thread safe; a Codec's
// methods can be called from concurrent goroutines.
type Codec interface {
	// Marshal returns the wire format of v.
	Marshal(v interface{}) ([]byte, error)
	// Unmarshal parses the wire format into v.
	Unmarshal(buf []byte, v interface{}) error
	// Name returns the name of the Codec implementation. Names are
	// case-insensitive. The returned string will be used as part of content type
	// in transmission. The result must be static; the result cannot change
	// between calls.
	Name() string
}

var registeredCodecs = make(map[string]Codec)

// RegisterCodec registers codec by its lower-cased name. Registration must be
// completed before codecs are accessed concurrently.
func RegisterCodec(codec Codec) {
	if codec == nil {
		panic("cannot register a nil Codec")
	}
	if codec.Name() == "" {
		panic("cannot register Codec with empty string result for Name()")
	}
	contentSubtype := strings.ToLower(codec.Name())
	registeredCodecs[contentSubtype] = codec
}

// GetCodec returns the codec registered under name, ignoring case, or nil when
// none exists.
func GetCodec(name string) Codec {
	if codec := registeredCodecs[name]; codec != nil {
		return codec
	}
	return registeredCodecs[strings.ToLower(name)]
}
